package submission

import (
	"context"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/phone"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
)

// state is where a session is. It maps to the submission status of the save
// file: waiting and receiving are waiting_photos, grading and applying are
// grading, done is graded, cancelled covers expiry too.
type state int

const (
	stWaiting   state = iota // the QR code is shown, no photos yet
	stReceiving              // an upload arrived and is being stored
	stGrading                // the grader works
	stApplying               // the results are being written to the save file
	stFailed                 // grading or saving failed; Retry may follow
	stDone                   // graded
	stCancelled              // cancelled or expired
)

// Session is one open submission: the exercises, the URL for the phone and the
// channel of events. Sessions are made by Service.Open. The exported fields are
// set before Open returns and never change.
type Session struct {
	// ID is the submission id, "<yyyymmddThhmmss>-<4 hex>", also the folder
	// name of its photos.
	ID string
	// Username is the student.
	Username string
	// CassetteID is the id of the cassette.
	CassetteID string
	// Exercises are the ids of the exercises, in the order selected.
	Exercises []string
	// Token is the token of the student's phone link (the same for every
	// session of the student while the link lives).
	Token string
	// URL is the address of the phone link (shown as the QR code).
	URL string
	// PhoneReady is true when a phone had the link open as the session was
	// made: its page switched to this submission by itself and the QR code is
	// only for another phone (SPEC §10.1).
	PhoneReady bool
	// ExpiresAt is when the session stops waiting for photos.
	ExpiresAt time.Time

	svc       *Service
	offer     *phone.Offer
	cass      *cassette.Cassette
	createdAt time.Time
	events    chan Event

	// persistMu serializes the changes of the submission record, together
	// with the state change they belong to: a transition decides the new
	// state under mu and writes the record before persistMu is released, so
	// the record cannot go back in time. Lock order: persistMu, then mu.
	persistMu sync.Mutex

	mu          sync.Mutex
	state       state
	opened      bool
	closed      bool // events is closed
	photos      []string
	pending     *outcome // a grading answer whose results could not be saved
	cancelRun   context.CancelFunc
	hintTimer   Timer
	expiryTimer Timer
}

// Events returns the channel of events of the session. It is closed after the
// last event of the session (Graded, Cancelled or Expired). See Event for what
// happens when the consumer is slow.
func (s *Session) Events() <-chan Event { return s.events }

// currentState returns the state.
func (s *Session) currentState() state {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// emit sends an event without ever blocking: when the buffer is full the oldest
// event is dropped. The caller holds s.mu, which orders the events.
func (s *Session) emit(ev Event) {
	if s.closed {
		return
	}
	for {
		select {
		case s.events <- ev:
			return
		default:
		}
		select {
		case <-s.events:
		default:
		}
	}
}

// finish ends the life of the session after its terminal event: the timers are
// stopped, the offer is withdrawn (the phone's page goes back to waiting; the
// link stays), the channel is closed and the session leaves the registry. The
// caller holds s.mu.
func (s *Session) finish() {
	s.stopTimers()
	if s.offer != nil {
		s.offer.Withdraw()
	}
	if !s.closed {
		s.closed = true
		close(s.events)
	}
	s.svc.forget(s)
}

// stopTimers stops the hint and the expiry timers. The caller holds s.mu.
func (s *Session) stopTimers() {
	if s.hintTimer != nil {
		s.hintTimer.Stop()
		s.hintTimer = nil
	}
	if s.expiryTimer != nil {
		s.expiryTimer.Stop()
		s.expiryTimer = nil
	}
}

// pageOpened is the Opened handler of the offer.
func (s *Session) pageOpened() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != stWaiting || s.opened {
		return
	}
	s.opened = true
	if s.hintTimer != nil {
		s.hintTimer.Stop()
		s.hintTimer = nil
	}
	s.emit(PageOpened{})
}

// uploadStarted is the Started handler of the phone token.
func (s *Session) uploadStarted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == stWaiting {
		s.emit(UploadStarted{})
	}
}

// firewallHint is the callback of the hint timer.
func (s *Session) firewallHint() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == stWaiting && !s.opened {
		s.emit(FirewallHint{})
	}
}

// expire is the callback of the expiry timer.
func (s *Session) expire() {
	s.end(s.svc.base, []state{stWaiting}, Expired{}, "The link expired before the photos were sent.")
}

// end moves the session from one of the states in from to cancelled, sends the
// terminal event and stores the submission as cancelled with the reason in its
// error field. It does nothing when the session is in another state (it ended
// already, or the results are being saved).
func (s *Session) end(ctx context.Context, from []state, ev Event, reason string) {
	s.persistMu.Lock()
	defer s.persistMu.Unlock()
	s.mu.Lock()
	if !slices.Contains(from, s.state) {
		s.mu.Unlock()
		return
	}
	s.state = stCancelled
	if s.cancelRun != nil {
		s.cancelRun()
	}
	s.emit(ev)
	s.finish()
	s.mu.Unlock()
	err := s.persist(ctx, func(_ *progress.Progress, sub *progress.Submission) error {
		sub.Status = progress.SubmissionCancelled
		sub.Error = reason
		return nil
	})
	if err != nil {
		s.svc.opts.Log.Error("the cancelled submission could not be saved", "id", s.ID, "err", err)
	}
}

// received is the Received handler of the phone token: it stores the photos,
// records them in the save file and starts grading on a goroutine, so the HTTP
// request of the phone is not held by the AI call. An error revokes the token.
func (s *Session) received(_ context.Context, photos [][]byte) error {
	s.mu.Lock()
	if s.state != stWaiting {
		s.mu.Unlock()
		return errors.New("SubmissionNotWaiting", "the submission is not waiting for photos", "id", s.ID)
	}
	s.state = stReceiving
	s.stopTimers()
	s.mu.Unlock()
	if hook := s.svc.onReceiving; hook != nil {
		hook()
	}

	dir := photoDir(s.svc.opts.Paths.PhotosDir(), s.Username, s.CassetteID, s.ID)
	names, err := storePhotos(dir, photos)
	if err != nil {
		s.failStoring(err)
		return err
	}

	s.persistMu.Lock()
	defer s.persistMu.Unlock()
	s.mu.Lock()
	if s.state != stReceiving {
		s.mu.Unlock()
		if rerr := os.RemoveAll(dir); rerr != nil {
			s.svc.opts.Log.Error("the photos of a cancelled submission were not removed", "dir", dir, "err", rerr)
		}
		return errors.New("SubmissionCancelled", "the submission was cancelled while the photos arrived", "id", s.ID)
	}
	s.photos = names
	s.emit(PhotosReceived{Count: len(names)})
	runCtx := s.beginGrading()
	s.mu.Unlock()

	err = s.persist(s.svc.base, func(_ *progress.Progress, sub *progress.Submission) error {
		sub.Photos = slices.Clone(names)
		sub.Status = progress.SubmissionGrading
		sub.Error = ""
		return nil
	})
	if err != nil {
		// The photos are stored and the child saw "Sent!": Retry can go on.
		s.svc.opts.Log.Error("the submission could not be saved", "id", s.ID, "err", err)
		s.mu.Lock()
		s.state = stFailed
		s.emit(Failed{Err: err, CanRetry: true})
		s.mu.Unlock()
		return nil
	}
	go s.run(runCtx)
	return nil
}

// failStoring ends a session whose photos could not be stored: it has nothing
// to grade, so it cannot be retried.
func (s *Session) failStoring(cause error) {
	s.persistMu.Lock()
	defer s.persistMu.Unlock()
	s.mu.Lock()
	if s.state != stReceiving {
		s.mu.Unlock()
		return
	}
	s.state = stFailed
	s.emit(Failed{Err: cause, CanRetry: false})
	s.mu.Unlock()
	s.persistFailure(s.svc.base, cause, "")
}

// beginGrading moves the session to grading, sends the Grading event and
// returns the context of the run. The caller holds s.mu.
func (s *Session) beginGrading() context.Context {
	s.state = stGrading
	ctx, cancel := context.WithCancel(s.svc.base)
	s.cancelRun = cancel
	s.emit(Grading{})
	return ctx
}

// retry is Service.Retry.
func (s *Session) retry(ctx context.Context) error {
	s.persistMu.Lock()
	defer s.persistMu.Unlock()
	s.mu.Lock()
	switch {
	case s.state != stFailed:
		s.mu.Unlock()
		return errors.New("SubmissionNotRetryable", "only a failed submission can be tried again", "id", s.ID)
	case len(s.photos) == 0:
		s.mu.Unlock()
		return errors.New("SubmissionNoPhotos", "the submission has no stored photos to grade again", "id", s.ID)
	}
	runCtx := s.beginGrading()
	names := slices.Clone(s.photos)
	s.mu.Unlock()
	err := s.persist(ctx, func(_ *progress.Progress, sub *progress.Submission) error {
		sub.Photos = names
		sub.Status = progress.SubmissionGrading
		sub.Error = ""
		return nil
	})
	if err != nil {
		s.mu.Lock()
		s.cancelRun()
		s.state = stFailed
		s.mu.Unlock()
		return err
	}
	go s.run(runCtx)
	return nil
}

// persist changes the submission record in the save file, adding the record
// when the file does not have it (the admin may have reset the progress). The
// save file is merged with the cassette first when it is older (prepare). A
// save file that was broken is set aside by the progress store and the change
// goes into a fresh one.
func (s *Session) persist(ctx context.Context, change func(p *progress.Progress, sub *progress.Submission) error) error {
	err := s.svc.opts.Progress.Update(ctx, s.Username, s.CassetteID, func(p *progress.Progress) error {
		if err := prepare(s.cass, p); err != nil {
			return err
		}
		sub := p.Submission(s.ID)
		if sub == nil {
			p.Submissions = append(p.Submissions, progress.Submission{
				ID:          s.ID,
				CreatedAt:   s.createdAt,
				ExerciseIDs: slices.Clone(s.Exercises),
				Photos:      []string{},
				Status:      progress.SubmissionWaitingPhotos,
				Results:     []progress.Result{},
			})
			sub = &p.Submissions[len(p.Submissions)-1]
		}
		return change(p, sub)
	})
	if appErr, ok := errors.FromError(err); ok && appErr.Tag == "ProgressFileBroken" {
		// The file was set aside and the change was saved in a fresh one.
		s.svc.opts.Log.Warn("a save file was broken and was set aside", "username", s.Username, "err", err)
		return nil
	}
	return err
}

// persistFailure stores the submission as failed with the error text and the
// provider's name. A failure to save is logged: the caller has the original
// error.
func (s *Session) persistFailure(ctx context.Context, cause error, providerName string) {
	s.mu.Lock()
	photos := slices.Clone(s.photos)
	s.mu.Unlock()
	err := s.persist(ctx, func(_ *progress.Progress, sub *progress.Submission) error {
		sub.Status = progress.SubmissionFailed
		sub.Error = cause.Error()
		if providerName != "" {
			sub.Provider = providerName
		}
		if len(sub.Photos) == 0 {
			sub.Photos = photos
		}
		return nil
	})
	if err != nil {
		s.svc.opts.Log.Error("the failed submission could not be saved", "id", s.ID, "err", err)
	}
}

package worksheet

import (
	"context"
	"sync"
	"time"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/qrcode"
	"github.com/rm4n0s/radio-tutor/internal/core/submission"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// Keys of the widgets of the submission flow.
const (
	// KeyQR is the key of the QR code.
	KeyQR = "qr"
	// KeyURL is the key of the text with the address of the phone page.
	KeyURL = "url"
	// KeyLabels is the key of the text with the labels of the exercises.
	KeyLabels = "labels"
	// KeyPhotos is the key of the "Photos received: n" text.
	KeyPhotos = "photos"
	// KeyPhoneReady is the key of the line that says that a phone has the
	// link open and its page switched to the submission by itself.
	KeyPhoneReady = "phone-ready"
	// KeyPhoneStatus is the key of the text that says what the phone does.
	KeyPhoneStatus = "phone-status"
	// KeyFirewall is the key of the firewall hint.
	KeyFirewall = "firewall-hint"
	// KeyCancel is the key of the Cancel buttons.
	KeyCancel = "cancel"
	// KeyGrading is the key of the grading indicator.
	KeyGrading = "grading"
	// KeyResultPrefix starts the key of the row of an exercise in the results;
	// the exercise id follows.
	KeyResultPrefix = "result-"
	// KeyMarkPrefix starts the key of the big mark of a result.
	KeyMarkPrefix = "mark-"
	// KeyStarPrefix starts the key of the star of an exercise that was passed.
	KeyStarPrefix = "star-"
	// KeyFeedbackPrefix starts the key of the feedback text of a result.
	KeyFeedbackPrefix = "feedback-"
	// KeyReviewPrefix starts the key of the "ask a parent" text of a result.
	KeyReviewPrefix = "review-"
	// KeyNewHintPrefix starts the key of the newly unlocked hint of a result.
	KeyNewHintPrefix = "new-hint-"
	// KeyBack is the key of the Back to the worksheet button.
	KeyBack = "back-to-worksheet"
	// KeyFailure is the key of the message of the failure screen.
	KeyFailure = "failure"
	// KeyDetails is the key of the details of the failure that only admins see.
	KeyDetails = "failure-details"
	// KeyRetry is the key of the Try again button.
	KeyRetry = "retry"
	// KeyDowngraded is the key of the note for the admin about the structured
	// output mode.
	KeyDowngraded = "downgraded"
)

// phase is where the flow is.
type phase int

const (
	phaseOpening phase = iota // the submission is being opened
	phaseWaiting              // the QR code is shown
	phaseGrading              // the photos arrived and are graded
	phaseResults              // graded
	phaseFailed               // opening or grading failed, or the link expired
)

// flowRequest is what the flow needs: the student and the exercises (1 to 10,
// in the order of the worksheet).
type flowRequest struct {
	Username    string
	Cassette    *cassette.Cassette
	ExerciseIDs []string
}

// failure is why the flow stopped.
type failure struct {
	// err is the cause; nil when the link expired.
	err error
	// canRetry is true when the stored photos can be graded again (Retry);
	// otherwise Try again opens a new submission.
	canRetry bool
	// expired is true when the link ran out of time.
	expired bool
}

// flow is the screen pushed by Check my answers: it opens the submission, shows
// the QR code for the phone while it waits for the photos, the grading
// indicator, and then the results or the failure.
type flow struct {
	m.StatefulWidget
	req flowRequest
}

// newFlow returns the flow screen.
func newFlow(req flowRequest, opts ...m.WidgetOpts) *flow {
	return &flow{StatefulWidget: m.NewStatefulWidgetBase(opts...), req: req}
}

// CreateState creates the state.
func (w *flow) CreateState() m.State { return &flowState{} }

type flowState struct {
	m.StateOf[*flow]
	env     *app.Env
	started bool

	// The fields below are read and written on the UI thread only.
	phase      phase
	sess       *submission.Session
	matrix     qrcode.Matrix
	photos     int
	pageOpened bool
	uploading  bool
	firewall   bool
	graded     *submission.Graded
	fail       *failure

	// mu guards the session that Dispose cancels, because the goroutine that
	// opens it can finish after Dispose ran.
	mu       sync.Mutex
	live     *submission.Session
	disposed bool
}

// DidChangeDependencies starts the submission (the Scope cannot be read in
// InitState).
func (s *flowState) DidChangeDependencies() {
	if s.started {
		return
	}
	s.started = true
	s.env = app.ScopeOf(s.Context()).Env()
	s.begin()
}

// begin opens a new submission on a goroutine.
func (s *flowState) begin() {
	s.phase, s.sess, s.fail, s.graded = phaseOpening, nil, nil, nil
	s.photos, s.pageOpened, s.uploading, s.firewall = 0, false, false, false
	req := s.Widget().req
	s.Go(func(ctx context.Context) { s.open(ctx, req) })
}

// open opens the submission and reads its events until the channel closes. It
// runs on a goroutine and hands everything to the UI thread.
func (s *flowState) open(ctx context.Context, req flowRequest) {
	sess, err := s.env.Submission.Open(ctx, req.Username, req.Cassette, req.ExerciseIDs)
	if err != nil {
		if ctx.Err() == nil {
			s.env.Log.Error("opening a submission failed", "username", req.Username, "err", err)
			s.SetState(func() { s.phase, s.fail = phaseFailed, &failure{err: err} })
		}
		return
	}
	if !s.adopt(sess) {
		return
	}
	matrix, err := qrcode.Encode(sess.URL, qrcode.DefaultLevel)
	if err != nil {
		s.env.Log.Error("the QR code could not be made", "url", sess.URL, "err", err)
		s.cancel(sess)
		s.SetState(func() { s.phase, s.fail = phaseFailed, &failure{err: err} })
		return
	}
	s.SetState(func() {
		s.sess, s.matrix, s.phase = sess, matrix, phaseWaiting
	})
	for {
		select {
		case ev, ok := <-sess.Events():
			if !ok {
				return
			}
			s.SetState(func() { s.handle(sess, ev) })
		case <-ctx.Done():
			return
		}
	}
}

// adopt remembers the session so that Dispose can cancel it; it cancels the
// session itself and returns false when the screen is gone already.
func (s *flowState) adopt(sess *submission.Session) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.disposed {
		s.cancel(sess)
		return false
	}
	s.live = sess
	return true
}

// cancel cancels the session; it takes a context of its own because the
// screen's context ends with the screen.
func (s *flowState) cancel(sess *submission.Session) {
	ctx, stop := context.WithTimeout(context.Background(), cancelTimeout)
	defer stop()
	if err := s.env.Submission.Cancel(ctx, sess.ID); err != nil {
		s.env.Log.Error("cancelling a submission failed", "id", sess.ID, "err", err)
	}
}

// cancelTimeout is how long cancelling a submission may take.
const cancelTimeout = 10 * time.Second

// Dispose cancels the submission when the screen is closed (Cancel, Back to the
// worksheet or the window closing); cancelling one that ended does nothing.
// Cancel is one small write of the save file, so it runs here and not on a
// goroutine: the service's Close does not wait for a cancel that is still
// writing, and the screen must not leave a write behind.
func (s *flowState) Dispose() {
	s.mu.Lock()
	s.disposed = true
	sess := s.live
	s.mu.Unlock()
	if sess != nil {
		s.cancel(sess)
	}
}

// handle applies an event of the session to the screen; it runs on the UI
// thread. Events of a session that was replaced (Try again after a failure that
// could not be retried opens a new one) are ignored.
func (s *flowState) handle(sess *submission.Session, ev submission.Event) {
	if s.sess != sess {
		return
	}
	switch e := ev.(type) {
	case submission.PageOpened:
		s.pageOpened = true
	case submission.UploadStarted:
		s.uploading = true
	case submission.PhotosReceived:
		s.photos, s.phase = e.Count, phaseGrading
	case submission.Grading:
		s.phase, s.fail = phaseGrading, nil
	case submission.Graded:
		s.graded, s.phase, s.fail = &e, phaseResults, nil
	case submission.Failed:
		s.phase, s.fail = phaseFailed, &failure{err: e.Err, canRetry: e.CanRetry}
	case submission.Expired:
		s.phase, s.fail = phaseFailed, &failure{expired: true}
	case submission.FirewallHint:
		s.firewall = true
	case submission.Cancelled:
		// Cancel and Back close the screen, whose disposal cancels the session,
		// so this is a cancel from elsewhere (the application closing).
		s.phase, s.fail = phaseFailed, &failure{}
	}
}

// tryAgain grades the stored photos again, or opens a new submission when
// there is nothing to grade again.
func (s *flowState) tryAgain() {
	sess, f := s.sess, s.fail
	if f == nil || !f.canRetry || f.expired || sess == nil {
		s.SetState(s.begin)
		return
	}
	s.SetState(func() { s.phase, s.fail = phaseGrading, nil })
	s.Go(func(ctx context.Context) {
		err := s.env.Submission.Retry(ctx, sess.ID)
		if err == nil || ctx.Err() != nil {
			return
		}
		s.env.Log.Error("grading again failed", "id", sess.ID, "err", err)
		s.SetState(func() {
			if s.sess == sess {
				s.phase, s.fail = phaseFailed, &failure{err: err}
			}
		})
	})
}

func (s *flowState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	var body m.Widget
	switch s.phase {
	case phaseOpening:
		body = s.openingView(bc, scope)
	case phaseWaiting:
		body = s.qrView(bc, scope)
	case phaseGrading:
		body = s.gradingView(bc, scope)
	case phaseResults:
		body = s.resultsView(bc, scope)
	default:
		body = s.failureView(bc, scope)
	}
	return m.NewScaffold(m.ScaffoldOpts{
		AppBar: m.NewAppBar(m.AppBarOpts{
			Title:                       m.NewText(s.title(scope)),
			NoAutomaticallyImplyLeading: true,
		}),
		Body: body,
	})
}

// title is the title of the app bar of the phase.
func (s *flowState) title(scope *app.Scope) string {
	if s.phase == phaseResults {
		return scope.T("flow.results")
	}
	return scope.T("worksheet.check")
}

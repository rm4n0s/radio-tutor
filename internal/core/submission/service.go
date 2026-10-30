// Package submission is the submission service (SPEC §9, §10 and §11): it ties
// the progress store, the phone server, the grader and the active provider
// together. A student opens a submission for some exercises of a worksheet; the
// service makes a token and a URL for the phone, stores the photos that arrive,
// grades them on a goroutine, applies the results to the save file and tells
// the UI what happens through typed events.
//
// # Threads
//
// The phone server calls the service on its own goroutines, grading runs on a
// goroutine of the service, and the timers run on theirs. None of them touches
// a widget: the UI reads Session.Events (a buffered channel that never blocks
// the service) and Posts what it needs to the UI thread.
//
// # Life of a session
//
//	waiting_photos -> grading -> graded
//	       |             |
//	       |             +-> failed -> (Retry) -> grading
//	       +-> cancelled (Cancel, a newer Open, Close, or 30 minutes)
//
// One session is open per student: Open cancels the student's earlier one. The
// 30 minute expiry covers the wait for the photos only (the phone token has the
// same lifetime); a session that is grading or failed stays until it ends or is
// cancelled. A failed session keeps its photos, so Retry grades the same photos
// again.
package submission

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/phone"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/providers"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
)

// Times of the service (SPEC §10.1, §10.2).
const (
	// ExpiresAfter is how long a session waits for its photos.
	ExpiresAfter = 30 * time.Minute
	// FirewallHintAfter is how long after Open the FirewallHint event is
	// sent when no phone has opened the page.
	FirewallHintAfter = 60 * time.Second
	// MaxExercises is the most exercises one submission may hold.
	MaxExercises = grading.MaxExercises
	// EventBuffer is the size of a session's event channel.
	EventBuffer = 16
)

// idAttempts is how many ids Open tries before it gives up.
const idAttempts = 8

// ProviderSource gives the active grading provider. *providers.Store is one.
type ProviderSource interface {
	// Active returns the active provider; false when none is.
	Active() (providers.Config, bool)
}

// SettingsSource gives the app settings. *settings.Store is one.
type SettingsSource interface {
	// Get returns the current settings.
	Get() settings.Settings
}

// Options configure a Service. Paths, Progress, Providers, Settings and Phone
// are required; the other fields have defaults and exist for tests.
type Options struct {
	// Paths locates the photos folder (SPEC §4).
	Paths storage.Paths
	// Progress is the save file store.
	Progress *progress.Store
	// Providers gives the active provider.
	Providers ProviderSource
	// Settings gives the phone port, the interface and the language.
	Settings SettingsSource
	// Phone is the phone server. The service starts it on the first Open
	// (on settings.Phone.Port) and keeps it running until Close.
	Phone *phone.Server
	// Clock is the time source. Default SystemClock.
	Clock Clock
	// Random is the source of the random part of submission ids. Default
	// crypto/rand.
	Random io.Reader
	// Log receives the service's log lines. Default: discarded.
	Log *slog.Logger
	// DetectLAN finds the address for the phone's URL. Default
	// phone.DetectLAN.
	DetectLAN func(ctx context.Context, preferred string) (phone.LANAddr, error)
	// Version is the app version for the User-Agent of the provider's
	// requests ("radio-tutor/<version>"); empty means "dev". It is used by
	// the default BuildProvider.
	Version string
	// BuildProvider builds the grading provider from the active config.
	// Default providers.Build with Version.
	BuildProvider func(cfg providers.Config, log *slog.Logger) (provider grading.Provider, lenient bool, err error)
}

// Service opens submissions and grades them. Use New. All methods are safe for
// concurrent use.
type Service struct {
	opts Options
	base context.Context

	openMu sync.Mutex // serializes Open and Close

	// onReceiving, when set by a test, runs when an upload has been accepted
	// and before its photos are stored.
	onReceiving func()

	mu       sync.Mutex
	sessions map[string]*Session // live sessions by submission id
	byUser   map[string]*Session // the live session of each student
}

// New returns a Service. ctx is the life of the application: the phone server
// and the grading goroutines run under it, so a cancelled dialog context does
// not stop them. Cancelling ctx stops the phone server; Close also cancels the
// open sessions.
func New(ctx context.Context, opts Options) *Service {
	if opts.Clock == nil {
		opts.Clock = SystemClock()
	}
	if opts.Random == nil {
		opts.Random = rand.Reader
	}
	if opts.Log == nil {
		opts.Log = slog.New(slog.DiscardHandler)
	}
	if opts.DetectLAN == nil {
		opts.DetectLAN = phone.DetectLAN
	}
	if opts.BuildProvider == nil {
		version := opts.Version
		opts.BuildProvider = func(cfg providers.Config, log *slog.Logger) (grading.Provider, bool, error) {
			return providers.Build(cfg, version, log)
		}
	}
	return &Service{
		opts:     opts,
		base:     ctx,
		sessions: map[string]*Session{},
		byUser:   map[string]*Session{},
	}
}

// Open starts a submission of the student for the exercises of c named by
// exerciseIDs (1 to 10, no repeats, in the order of the call).
//
// Each exercise must be available for the student (progress.States); in free
// navigation any exercise of the cassette is accepted. The student's earlier
// open session, if any, is cancelled. The phone server is started if it is not
// running, the submission is stored in the save file as waiting_photos, and the
// Session carries the URL and token for the QR code and the channel of events.
//
// It fails with the tags SubmissionOpenCancelled, SubmissionExerciseCount,
// SubmissionExerciseDuplicate, SubmissionExerciseUnknown,
// SubmissionExerciseUnavailable, SubmissionIDRandomFailed and SubmissionIDTaken,
// and with the tags of the layers it calls: the progress store (an invalid
// username, an unreadable save file), the LAN detection (PhoneNoNetwork), the
// phone server (PhonePortInUse) and storage. A save file that cannot be parsed
// is set aside by the progress store and Open goes on with a fresh one.
func (svc *Service) Open(ctx context.Context, username string, c *cassette.Cassette, exerciseIDs []string) (*Session, error) {
	svc.openMu.Lock()
	defer svc.openMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, errors.NewErr("SubmissionOpenCancelled", err).SetMetadata("username", username)
	}
	if err := checkExercises(c, exerciseIDs); err != nil {
		return nil, err
	}
	p, err := svc.loadProgress(ctx, username, c)
	if err != nil {
		return nil, err
	}
	if err := checkAvailable(c, p, exerciseIDs); err != nil {
		return nil, err
	}
	cfg := svc.opts.Settings.Get()
	lan, err := svc.opts.DetectLAN(ctx, cfg.Phone.Interface)
	if err != nil {
		return nil, err
	}
	now := svc.opts.Clock.Now()
	id, err := svc.newID(now, p)
	if err != nil {
		return nil, err
	}

	svc.cancelOf(ctx, username)
	if err := svc.ensurePhone(ctx, cfg); err != nil {
		return nil, err
	}

	s := &Session{
		ID:         id,
		Username:   username,
		CassetteID: c.Manifest.ID,
		Exercises:  append([]string(nil), exerciseIDs...),
		ExpiresAt:  now.Add(ExpiresAfter),
		svc:        svc,
		cass:       c,
		createdAt:  stampTime(now),
		events:     make(chan Event, EventBuffer),
		state:      stWaiting,
	}
	token, url, err := svc.opts.Phone.Open(lan.IP.String(), pageData(cfg.Language, c, exerciseIDs), phone.Handlers{
		Opened:   s.pageOpened,
		Started:  s.uploadStarted,
		Received: s.received,
	})
	if err != nil {
		return nil, err
	}
	// The token reaches nobody before Open returns, so no handler runs yet.
	s.mu.Lock()
	s.Token, s.URL = token, url
	s.mu.Unlock()
	if err := s.persist(ctx, func(_ *progress.Progress, sub *progress.Submission) error {
		sub.Status = progress.SubmissionWaitingPhotos
		return nil
	}); err != nil {
		svc.opts.Phone.Close(token)
		return nil, err
	}

	svc.mu.Lock()
	svc.sessions[s.ID] = s
	svc.byUser[username] = s
	svc.mu.Unlock()
	s.mu.Lock()
	s.hintTimer = svc.opts.Clock.AfterFunc(FirewallHintAfter, s.firewallHint)
	s.expiryTimer = svc.opts.Clock.AfterFunc(ExpiresAfter, s.expire)
	s.mu.Unlock()
	return s, nil
}

// Retry grades the stored photos of a failed session again. No new photos are
// needed and no attempt was used by the failure (attempts count only when
// results are applied). If the grader had answered but the results could not be
// saved, they are applied again without asking the provider. The Grading event
// follows. It fails with SubmissionSessionUnknown, SubmissionNotRetryable (the
// session is not failed), SubmissionNoPhotos (there are no stored photos), or
// the tags of the progress store when the save file cannot be written.
func (svc *Service) Retry(ctx context.Context, sessionID string) error {
	s := svc.find(sessionID)
	if s == nil {
		return errors.New("SubmissionSessionUnknown", "no such open submission", "id", sessionID)
	}
	return s.retry(ctx)
}

// Cancel ends the session: the phone token stops working, grading in progress
// is abandoned, the submission is stored as cancelled and the Cancelled event
// is sent. Photos already stored stay on disk. Cancelling a session that has
// ended, or an id that is not open, does nothing.
func (svc *Service) Cancel(ctx context.Context, sessionID string) error {
	s := svc.find(sessionID)
	if s == nil {
		return nil
	}
	s.end(ctx, []state{stWaiting, stReceiving, stGrading, stFailed}, Cancelled{}, "")
	return nil
}

// Close cancels every open session and stops the phone server.
func (svc *Service) Close(ctx context.Context) error {
	svc.openMu.Lock()
	defer svc.openMu.Unlock()
	svc.mu.Lock()
	open := make([]*Session, 0, len(svc.sessions))
	for _, s := range svc.sessions {
		open = append(open, s)
	}
	svc.mu.Unlock()
	for _, s := range open {
		s.end(ctx, []state{stWaiting, stReceiving, stGrading, stFailed}, Cancelled{}, "")
	}
	return svc.opts.Phone.Stop(ctx)
}

// find returns the live session with the id, or nil.
func (svc *Service) find(id string) *Session {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return svc.sessions[id]
}

// cancelOf cancels the student's open session, if any.
func (svc *Service) cancelOf(ctx context.Context, username string) {
	svc.mu.Lock()
	prev := svc.byUser[username]
	svc.mu.Unlock()
	if prev != nil {
		prev.end(ctx, []state{stWaiting, stReceiving, stGrading, stFailed}, Cancelled{}, "")
	}
}

// forget removes a finished session from the registry.
func (svc *Service) forget(s *Session) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	delete(svc.sessions, s.ID)
	if svc.byUser[s.Username] == s {
		delete(svc.byUser, s.Username)
	}
}

// ensurePhone starts the phone server on the configured port, and restarts it
// when the port setting changed and no session is waiting for photos.
func (svc *Service) ensurePhone(ctx context.Context, cfg settings.Settings) error {
	ph := svc.opts.Phone
	ph.SetLang(cfg.Language)
	if ph.Running() {
		if cfg.Phone.Port == 0 || ph.Port() == cfg.Phone.Port || svc.anyWaiting() {
			return nil
		}
		if err := ph.Stop(ctx); err != nil {
			return err
		}
	}
	return ph.Start(svc.base, cfg.Phone.Port)
}

// anyWaiting reports whether a live session is waiting for photos.
func (svc *Service) anyWaiting() bool {
	svc.mu.Lock()
	open := make([]*Session, 0, len(svc.sessions))
	for _, s := range svc.sessions {
		open = append(open, s)
	}
	svc.mu.Unlock()
	for _, s := range open {
		if s.currentState() == stWaiting {
			return true
		}
	}
	return false
}

// loadProgress reads the student's save file for the cassette and merges it
// with the cassette in memory (prepare). A file that was broken is set aside by
// the progress store and a fresh one is used.
func (svc *Service) loadProgress(ctx context.Context, username string, c *cassette.Cassette) (*progress.Progress, error) {
	p, err := svc.opts.Progress.Load(ctx, username, c.Manifest.ID)
	if p == nil {
		return nil, err
	}
	if err != nil {
		svc.opts.Log.Warn("a save file was broken and was set aside", "username", username, "err", err)
	}
	if err := prepare(c, p); err != nil {
		return nil, err
	}
	return p, nil
}

// prepare merges a save file that belongs to an older version of the cassette
// with the cassette (progress.Migrate, SPEC §7.5) before the file is used. A
// file already merged with this version or a newer one (the cassette was
// upgraded while a session was open) is left alone.
func prepare(c *cassette.Cassette, p *progress.Progress) error {
	if !p.CassetteVersion.Less(c.Manifest.Version) {
		return nil
	}
	_, err := progress.Migrate(c, p)
	return err
}

// checkExercises checks the list of exercises: 1 to 10, no repeats, all in the
// cassette.
func checkExercises(c *cassette.Cassette, ids []string) error {
	if n := len(ids); n < 1 || n > MaxExercises {
		return errors.New("SubmissionExerciseCount", "a submission needs 1 to 10 exercises", "count", n, "max", MaxExercises)
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			return errors.New("SubmissionExerciseDuplicate", "an exercise is selected twice", "exercise", id)
		}
		seen[id] = true
		if c.Exercises[id] == nil {
			return errors.New("SubmissionExerciseUnknown", "the cassette has no such exercise", "exercise", id)
		}
	}
	return nil
}

// checkAvailable checks that the student may submit each exercise: it is
// available by the unlock rule, or free navigation is on.
func checkAvailable(c *cassette.Cassette, p *progress.Progress, ids []string) error {
	if p.Settings.FreeNavigation {
		return nil
	}
	states := progress.States(c, p)
	for _, id := range ids {
		if st := states[id]; st != progress.StateAvailable {
			return errors.New("SubmissionExerciseUnavailable", "the exercise is not available for a submission", "exercise", id, "state", string(st))
		}
	}
	return nil
}

// newID makes a submission id, "<yyyymmddThhmmss>-<4 hex>" (SPEC §7.4), that no
// submission of the save file has.
func (svc *Service) newID(now time.Time, p *progress.Progress) (string, error) {
	prefix := now.UTC().Format("20060102T150405") + "-"
	for range idAttempts {
		var raw [2]byte
		if _, err := io.ReadFull(svc.opts.Random, raw[:]); err != nil {
			return "", errors.NewErr("SubmissionIDRandomFailed", err)
		}
		if id := prefix + hex.EncodeToString(raw[:]); p.Submission(id) == nil {
			return id, nil
		}
	}
	return "", errors.New("SubmissionIDTaken", "no free submission id was found", "attempts", idAttempts)
}

// pageData is what the phone page shows (SPEC §10.1).
func pageData(lang i18n.Lang, c *cassette.Cassette, ids []string) phone.PageData {
	page := phone.PageData{Lang: lang, Title: c.Manifest.Title}
	for _, id := range ids {
		ex := c.Exercises[id]
		page.Exercises = append(page.Exercises, phone.PageExercise{Label: ex.Label, Title: ex.Title, Prompt: ex.Prompt})
	}
	return page
}

// stampTime is a time as stored in the save file: UTC, whole seconds.
func stampTime(t time.Time) time.Time { return t.UTC().Truncate(time.Second) }

// NoProvider reports whether err says that no grading provider is active, so
// the UI shows "Ask a parent to set up the checker".
func NoProvider(err error) bool {
	appErr, ok := errors.FromError(err)
	return ok && appErr.Tag == "SubmissionNoProvider"
}

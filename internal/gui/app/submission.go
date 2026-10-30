package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/phone"
	"github.com/rm4n0s/radio-tutor/internal/core/providers"
	"github.com/rm4n0s/radio-tutor/internal/core/submission"
)

// closeTimeout is how long closing the submission service may take at exit.
const closeTimeout = 5 * time.Second

// SubmissionOptions are the parts of the submission service that tests replace
// (SPEC §10). The zero value gives the real service: the phone server on every
// interface, the real LAN detection, the real grading providers and the system
// clock.
type SubmissionOptions struct {
	// Phone replaces the phone server. Tests pass one that listens on
	// 127.0.0.1 only (phone.Options.ListenHost).
	Phone *phone.Server
	// DetectLAN replaces the LAN detection (submission.Options.DetectLAN).
	DetectLAN func(ctx context.Context, preferred string) (phone.LANAddr, error)
	// BuildProvider replaces providers.Build, so a test grades with the fake
	// provider (submission.Options.BuildProvider).
	BuildProvider func(cfg providers.Config, log *slog.Logger) (provider grading.Provider, lenient bool, err error)
	// Clock replaces the clock of the service (its two timers).
	Clock submission.Clock
}

// noProviders is the ProviderSource used when providers.json could not be
// read: no provider is active, so grading fails with "Ask a parent to set up
// the checker".
type noProviders struct{}

// Active reports that no provider is active.
func (noProviders) Active() (providers.Config, bool) { return providers.Config{}, false }

// openProviders opens providers.json. A file that cannot be read (too open, not
// JSON) is logged and leaves Env.Providers nil with the reason in
// Env.ProvidersErr: the admin sees a notice and the providers screen explains
// it; grading says that no checker is set up. The file is left as it is.
func (e *Env) openProviders(ctx context.Context) {
	path := e.Paths.ProvidersFile()
	st, err := providers.Open(ctx, path)
	if err != nil {
		e.ProvidersErr = err
		e.Log.Error("providers.json could not be read; grading is unavailable", "err", err)
		message := err.Error()
		if appErr, ok := errors.FromError(err); ok {
			message = appErr.Message
		}
		e.Notices = append(e.Notices, i18n.M("notice.providers_unusable", "path", path, "message", message))
		return
	}
	e.Providers = st
}

// openSubmission creates the submission service for the life of the
// application: ctx is the application's context, and Env.Close closes the
// service (it cancels the open sessions and stops the phone server).
func (e *Env) openSubmission(ctx context.Context, opts SubmissionOptions) {
	var source submission.ProviderSource = noProviders{}
	if e.Providers != nil {
		source = e.Providers
	}
	ph := opts.Phone
	if ph == nil {
		ph = phone.New(phone.Options{Lang: e.Settings.Get().Language, Log: e.Log})
	}
	base, cancel := context.WithCancel(ctx)
	e.Submission = submission.New(base, submission.Options{
		Paths:         e.Paths,
		Progress:      e.Progress,
		Providers:     source,
		Settings:      e.Settings,
		Phone:         ph,
		Clock:         opts.Clock,
		Log:           e.Log,
		DetectLAN:     opts.DetectLAN,
		BuildProvider: opts.BuildProvider,
	})
	svc := e.Submission
	e.closers = append(e.closers, func() {
		closeCtx, stop := context.WithTimeout(context.Background(), closeTimeout)
		defer stop()
		if err := svc.Close(closeCtx); err != nil {
			e.Log.Error("closing the submission service failed", "err", err)
		}
		cancel()
	})
}

package app

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/logging"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
)

// DebugVariable is the environment variable that turns the debug log on.
const DebugVariable = "RADIO_TUTOR_DEBUG"

// Env is what the screens share: the folders, the logger and the stores that
// were opened at startup (SPEC §4). The later steps add their stores here.
type Env struct {
	// Paths are the folders of SPEC §4.
	Paths storage.Paths
	// Log is the application log (SPEC §15).
	Log *slog.Logger
	// Profiles are the profiles and the login.
	Profiles *profile.Store
	// Settings are the app settings.
	Settings *settings.Store
	// Notices are the messages for the admin that startup produced (a broken
	// file that was set aside); the home of the admin shows them.
	Notices []i18n.Message

	closers []func()
}

// StartOptions are the optional parameters of Start.
type StartOptions struct {
	// Getenv reads the environment; the default is os.Getenv. Start reads LANG,
	// RADIO_TUTOR_DEBUG and, through storage.ResolveFrom, HOME and the XDG
	// variables.
	Getenv func(string) string
	// ProfileOptions are passed to profile.Open (tests use few hash iterations).
	ProfileOptions []profile.Option
	// Now gives the time used to name a quarantined file; the default is time.Now.
	Now func() time.Time
}

// Start prepares the application (SPEC §4, §15): it resolves the folders, takes
// the single-instance lock (failing with the storage tag AlreadyRunning when
// another instance holds it), opens the log (debug level when
// RADIO_TUTOR_DEBUG=1), and opens the settings and the profiles. A settings.json
// or profiles.json that cannot be parsed is set aside with
// storage.QuarantineBroken, logged, and reported in Env.Notices; the app starts
// with the defaults or no profiles. Failures are returned as the storage,
// logging, settings and profile packages made them (AlreadyRunning among them).
//
// The caller calls Env.Close when the application ends.
func Start(ctx context.Context, opts StartOptions) (*Env, error) {
	getenv := opts.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	paths, err := storage.ResolveFrom(getenv)
	if err != nil {
		return nil, err
	}
	unlock, err := storage.Lock(paths)
	if err != nil {
		return nil, err
	}
	env := &Env{Paths: paths, closers: []func(){unlock}}
	logger, logCloser, err := logging.New(paths, getenv(DebugVariable) == "1")
	if err != nil {
		env.Close()
		return nil, err
	}
	env.Log = logger
	env.closers = append(env.closers, func() { closeLog(logger, logCloser) })

	if err := env.openSettings(ctx, getenv("LANG"), now); err != nil {
		env.Close()
		return nil, err
	}
	if err := env.openProfiles(ctx, opts.ProfileOptions, now); err != nil {
		env.Close()
		return nil, err
	}
	logger.Info("started", "config", paths.Config, "data", paths.Data, "state", paths.State)
	return env, nil
}

// closeLog closes the log file; a failure has nowhere left to be reported.
func closeLog(l *slog.Logger, c io.Closer) {
	if err := c.Close(); err != nil {
		l.Error("closing the log failed", "err", err)
	}
}

// Close releases the lock and closes the log. It may be called more than once.
func (e *Env) Close() {
	for i := len(e.closers) - 1; i >= 0; i-- {
		e.closers[i]()
	}
	e.closers = nil
}

// openSettings opens settings.json, setting a broken file aside.
func (e *Env) openSettings(ctx context.Context, locale string, now func() time.Time) error {
	path := e.Paths.SettingsFile()
	st, err := settings.Open(ctx, path, locale)
	if err != nil {
		if !isBroken(err) {
			return err
		}
		if qerr := e.quarantine(path, err, now, "notice.settings_broken"); qerr != nil {
			return qerr
		}
		if st, err = settings.Open(ctx, path, locale); err != nil {
			return err
		}
	}
	e.Settings = st
	return nil
}

// openProfiles opens profiles.json, setting a broken file aside.
func (e *Env) openProfiles(ctx context.Context, opts []profile.Option, now func() time.Time) error {
	path := e.Paths.ProfilesFile()
	st, err := profile.Open(ctx, path, opts...)
	if err != nil {
		if !isBroken(err) {
			return err
		}
		if qerr := e.quarantine(path, err, now, "notice.profiles_broken"); qerr != nil {
			return qerr
		}
		if st, err = profile.Open(ctx, path, opts...); err != nil {
			return err
		}
	}
	e.Profiles = st
	return nil
}

// quarantine renames the broken file, logs the failure and adds the notice.
func (e *Env) quarantine(path string, cause error, now func() time.Time, noticeKey string) error {
	moved, err := storage.QuarantineBroken(path, now())
	if err != nil {
		return err
	}
	e.Log.Error("a file could not be read and was set aside", "path", path, "moved_to", moved, "err", cause)
	e.Notices = append(e.Notices, i18n.M(noticeKey, "path", moved))
	return nil
}

// isBroken reports whether an Open failure means the file's content is damaged
// (so it is set aside) and not that the open was cancelled or the file could
// not be read at all.
func isBroken(err error) bool {
	appErr, ok := errors.FromError(err)
	if !ok {
		return false
	}
	switch appErr.Tag {
	case "SettingsOpenCancelled", "ProfilesOpenCancelled", "JSONReadFailed":
		return false
	}
	return true
}

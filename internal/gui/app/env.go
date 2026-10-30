package app

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/archive"
	"github.com/rm4n0s/radio-tutor/internal/core/audio"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/logging"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/providers"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
	"github.com/rm4n0s/radio-tutor/internal/core/submission"
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
	// Archive is the store of the installed cassettes.
	Archive *archive.Store
	// Progress is the store of the students' save files.
	Progress *progress.Store
	// Providers is the list of AI providers; nil when providers.json could not
	// be read (see ProvidersErr).
	Providers *providers.Store
	// ProvidersErr is why Providers is nil: providers.json is readable by other
	// users (ProvidersFileTooOpen) or cannot be parsed. The app starts anyway;
	// the admin sees a notice and the providers screen explains it.
	ProvidersErr error
	// Submission is the submission service (SPEC §10), created once for the run
	// of the application and closed by Close.
	Submission *submission.Service
	// ChooseFile asks the user for a file (the Import button); the default is
	// DesktopFileChooser. Tests replace it.
	ChooseFile FileChooser
	// SinkFactory opens the audio sink of the screens that play sound; the
	// default is DefaultSinkFactory. Tests replace it.
	SinkFactory SinkFactory
	// Screens are the screens of the other GUI packages that the shell opens.
	Screens Screens
	// Notices are the messages for the admin that startup produced (a broken
	// file that was set aside); the home of the admin shows them.
	Notices []i18n.Message
	// AudioClock is the clock the lesson player's end timer runs on; the
	// default is audio.SystemClock. Tests that play on an audio.FakeSink pass
	// the audio.FakeClock the sink runs on.
	AudioClock audio.Clock
	// Version is the app version, which goes into the User-Agent of the
	// requests to the AI providers; empty means "dev".
	Version string
	// Location is the time zone the screens show dates and times in; the
	// default is time.Local. Tests pass a fixed zone, so that no test changes
	// the process-wide time.Local.
	Location *time.Location

	closers      []func()
	noSoundShown atomic.Bool
}

// StartOptions are the optional parameters of Start.
type StartOptions struct {
	// Getenv reads the environment; the default is os.Getenv. Start reads LANG,
	// RADIO_TUTOR_DEBUG and, through storage.ResolveFrom, HOME and the XDG
	// variables.
	Getenv func(string) string
	// ProfileOptions are passed to profile.Open (tests use few hash iterations).
	ProfileOptions []profile.Option
	// Render checks the SVG files of the cassettes that are imported (nil skips
	// that check).
	Render archive.RenderChecker
	// ChooseFile replaces DesktopFileChooser (tests).
	ChooseFile FileChooser
	// SinkFactory replaces DefaultSinkFactory (tests).
	SinkFactory SinkFactory
	// Submission replaces parts of the submission service (tests).
	Submission SubmissionOptions
	// Screens are the screens of the other GUI packages.
	Screens Screens
	// AudioClock replaces audio.SystemClock as the clock of the lesson player
	// (tests).
	AudioClock audio.Clock
	// Version is the app version (from the build information of the command);
	// it goes into the User-Agent of the AI provider requests.
	Version string
	// Location replaces time.Local as the time zone of the dates the screens
	// show (tests).
	Location *time.Location
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
// It also opens the cassette store (finishing or undoing an interrupted import,
// SPEC §6.2) and the progress store.
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
	env := &Env{Paths: paths, Version: opts.Version, closers: []func(){unlock}}
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
	env.Archive = archive.NewStore(paths, opts.Render)
	if err := env.Archive.Recover(ctx); err != nil {
		// An interrupted import or removal that cannot be finished must not
		// keep the app from starting; the cassette store lists what is there.
		logger.Error("recovering the cassettes folder failed", "err", err)
	}
	env.Progress = progress.NewStore(paths)
	env.ChooseFile = opts.ChooseFile
	if env.ChooseFile == nil {
		env.ChooseFile = DesktopFileChooser
	}
	env.SinkFactory = opts.SinkFactory
	if env.SinkFactory == nil {
		env.SinkFactory = env.DefaultSinkFactory
	}
	env.openProviders(ctx)
	env.openSubmission(ctx, opts.Submission)
	env.Screens = opts.Screens
	env.AudioClock = opts.AudioClock
	if env.AudioClock == nil {
		env.AudioClock = audio.SystemClock()
	}
	env.Location = opts.Location
	if env.Location == nil {
		env.Location = time.Local
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

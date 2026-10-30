// Package testutil has the helpers for the widget and golden tests of the GUI
// packages: pumping the application with isolated storage, waiting for the work
// the screens do on goroutines, and making profiles.
//
// Fonts: gluttertest lays text out with the fonts bundled with Glutter, Roboto
// and Material Icons, and no system fonts. Roboto has Latin, Cyrillic and Greek
// glyphs, so the Greek UI renders in goldens on every machine; no extra font is
// needed (Opts.Fonts of gluttertest would add more).
package testutil

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rm4n0s/glutter/foundation"
	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// DefaultSize is the size of the test window: the smallest window of the app.
var DefaultSize = m.Size{Width: app.MinWindowWidth, Height: app.MinWindowHeight}

// hashIterations is the PBKDF2 cost of the profiles made in tests: one
// iteration, so that a login takes microseconds instead of a third of a second.
const hashIterations = 1

// UseLinux makes the framework behave as on the Linux desktop, which the app is
// for (gluttertest defaults to Android). It is undone when the test ends and
// skips the test in a release build, where the override does not exist.
func UseLinux(t testing.TB) {
	t.Helper()
	if !foundation.DebugMode {
		t.Skip("the platform override is a debug-only feature")
	}
	foundation.SetDebugDefaultTargetPlatformOverride(foundation.Some(foundation.TargetPlatformLinux))
	t.Cleanup(func() {
		foundation.SetDebugDefaultTargetPlatformOverride(foundation.None[foundation.TargetPlatform]())
	})
}

// Options are the optional parameters of NewEnv and Pump.
type Options struct {
	// Locale is the LANG of the app, which picks the language of a first run;
	// the default is "en_US.UTF-8".
	Locale string
	// Size is the size of the window; the default is DefaultSize.
	Size m.Size
	// Screens are the screens of the other GUI packages (app.StartOptions).
	Screens app.Screens
	// ChooseFile answers the file dialogs; the default fails the test, because a
	// test must not open the dialog of the desktop.
	ChooseFile app.FileChooser
	// SinkFactory opens the audio sink of the screens (the default plays real
	// sound or nothing, so tests that play pass an audio.FakeSink).
	SinkFactory app.SinkFactory
	// Submission replaces parts of the submission service (the phone server on
	// 127.0.0.1, the fake provider, the LAN stub).
	Submission app.SubmissionOptions
	// Before runs on the environment after it started and before the window
	// opens, for example to make profiles.
	Before func(env *app.Env)
}

// NewEnv isolates the storage folders (storagetest.Isolate) and starts the app
// environment in them with a cheap password hash and the locale of the options.
// The environment is closed when the test ends.
func NewEnv(t testing.TB, opts Options) (*app.Env, storage.Paths) {
	t.Helper()
	paths := storagetest.Isolate(t)
	choose := opts.ChooseFile
	if choose == nil {
		choose = func(context.Context, m.BuildContext, app.FileQuestion) (string, bool, error) {
			t.Error("the app opened a file dialog and the test gave no ChooseFile")
			return "", false, nil
		}
	}
	locale := opts.Locale
	if locale == "" {
		locale = "en_US.UTF-8"
	}
	env, err := app.Start(context.Background(), app.StartOptions{
		Getenv: func(name string) string {
			switch name {
			case "LANG":
				return locale
			case app.DebugVariable:
				return ""
			}
			return os.Getenv(name)
		},
		ProfileOptions: []profile.Option{profile.WithIterations(hashIterations)},
		Screens:        opts.Screens,
		ChooseFile:     choose,
		SinkFactory:    opts.SinkFactory,
		Submission:     opts.Submission,
	})
	if err != nil {
		t.Fatalf("testutil.NewEnv: %v", err)
	}
	t.Cleanup(env.Close)
	if opts.Before != nil {
		opts.Before(env)
	}
	return env, paths
}

// AddProfile creates a profile in the environment's store, failing the test if
// that does not work.
func AddProfile(t testing.TB, env *app.Env, username, displayName string, role profile.Role, password string) profile.Profile {
	t.Helper()
	p, err := env.Profiles.Create(context.Background(), username, displayName, role, password)
	if err != nil {
		t.Fatalf("testutil.AddProfile(%s): %v", username, err)
	}
	return p
}

// App is the application pumped in a test: the tester, the environment and the
// folders.
type App struct {
	*gluttertest.Tester
	// Env is the environment the app runs on.
	Env *app.Env
	// Paths are the isolated storage folders.
	Paths storage.Paths
}

// Pump isolates storage, starts the environment, pumps the app's Root in a
// window of the size of the options and returns it.
func Pump(t *testing.T, opts Options) *App {
	t.Helper()
	UseLinux(t)
	env, paths := NewEnv(t, opts)
	return PumpEnv(t, env, paths, opts.Size)
}

// PumpEnv pumps the Root of an environment that NewEnv started.
func PumpEnv(t *testing.T, env *app.Env, paths storage.Paths, size m.Size) *App {
	t.Helper()
	UseLinux(t)
	if size == (m.Size{}) {
		size = DefaultSize
	}
	ts := gluttertest.New(t, gluttertest.Opts{Size: size})
	ts.PumpWidget(app.NewRoot(env))
	ts.PumpAndSettle()
	return &App{Tester: ts, Env: env, Paths: paths}
}

// waitLimit is how long WaitFor waits, in real time.
const waitLimit = 5 * time.Second

// WaitFor pumps frames until cond is true, giving the goroutines of the screens
// (a login, a profile creation) real time to finish; it fails the test after
// five seconds.
func (a *App) WaitFor(t testing.TB, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitLimit)
	for {
		a.FlushPosted()
		a.PumpAndSettle()
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// WaitForText pumps until the text is on screen.
func (a *App) WaitForText(t testing.TB, text string) {
	t.Helper()
	a.WaitFor(t, "the text "+text, func() bool { return a.Exists(gluttertest.ByText(text)) })
}

// TapKey taps the widget with the key and lets the animations settle.
func (a *App) TapKey(key string) {
	a.Tap(gluttertest.ByKey(m.NewValueKey(key)))
	a.PumpAndSettle()
}

// EnterKey types text into the field with the key.
func (a *App) EnterKey(key, text string) {
	a.EnterText(gluttertest.ByKey(m.NewValueKey(key)), text)
	a.Pump()
}

// HasKey reports whether a widget with the key is on screen.
func (a *App) HasKey(key string) bool {
	return a.Exists(gluttertest.ByKey(m.NewValueKey(key)))
}

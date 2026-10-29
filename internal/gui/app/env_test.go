package app_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// startOptions are the options of the tests: cheap hashes, a fixed clock and an
// environment that is only what the test names.
func startOptions(env map[string]string) app.StartOptions {
	return app.StartOptions{
		Getenv: func(name string) string {
			if v, ok := env[name]; ok {
				return v
			}
			return os.Getenv(name) // XDG_* from storagetest.Isolate
		},
		ProfileOptions: []profile.Option{profile.WithIterations(1)},
		Now:            func() time.Time { return time.Date(2026, 10, 30, 10, 15, 0, 0, time.UTC) },
	}
}

func hasRoute(t *testing.T, err error, route string) {
	t.Helper()
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T (%v)", err, err)
	}
	if !appErr.HasRoute(route) {
		t.Fatalf("unexpected failure path: %s", appErr.Route())
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestStartOpensEverything(t *testing.T) {
	paths := storagetest.Isolate(t)
	env, err := app.Start(context.Background(), startOptions(map[string]string{"LANG": "el_GR.UTF-8"}))
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	if env.Paths != paths {
		t.Errorf("paths = %+v, want %+v", env.Paths, paths)
	}
	if got := env.Settings.Get().Language; got != i18n.EL {
		t.Errorf("language = %q, want el (from LANG)", got)
	}
	if !env.Profiles.NeedsFirstAdmin() || len(env.Notices) != 0 {
		t.Errorf("a first run has no admin and no notice (notices %v)", env.Notices)
	}
	env.Log.Info("hello from the test")
	data, err := os.ReadFile(paths.LogFile())
	if err != nil || !strings.Contains(string(data), "hello from the test") {
		t.Errorf("the log file has %q, %v", data, err)
	}
	if env.Log.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("debug logging is on without RADIO_TUTOR_DEBUG")
	}
}

func TestStartDebugLog(t *testing.T) {
	tests := []struct {
		value string
		debug bool
	}{{"", false}, {"0", false}, {"1", true}}
	for _, tt := range tests {
		t.Run("value "+tt.value, func(t *testing.T) {
			storagetest.Isolate(t)
			env, err := app.Start(context.Background(), startOptions(map[string]string{app.DebugVariable: tt.value}))
			if err != nil {
				t.Fatal(err)
			}
			defer env.Close()
			if got := env.Log.Enabled(context.Background(), slog.LevelDebug); got != tt.debug {
				t.Errorf("debug enabled = %v, want %v", got, tt.debug)
			}
		})
	}
}

func TestStartTakesTheLock(t *testing.T) {
	storagetest.Isolate(t)
	first, err := app.Start(context.Background(), startOptions(nil))
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.Start(context.Background(), startOptions(nil))
	if err == nil {
		t.Fatal("a second instance started")
	}
	hasRoute(t, err, "AlreadyRunning")
	if appErr, _ := errors.FromError(err); appErr.Tag != "AlreadyRunning" {
		t.Errorf("tag = %q, want AlreadyRunning", appErr.Tag)
	}
	first.Close()
	first.Close() // closing twice is fine
	again, err := app.Start(context.Background(), startOptions(nil))
	if err != nil {
		t.Fatalf("the lock was not released by Close: %v", err)
	}
	again.Close()
}

func TestStartSetsBrokenFilesAside(t *testing.T) {
	tests := []struct {
		name    string
		file    func(paths storage.Paths) string
		content string
		notice  string
	}{
		{"profiles not json", storage.Paths.ProfilesFile, `{nope`, "notice.profiles_broken"},
		{"profiles bad format", storage.Paths.ProfilesFile, `{"format":9,"profiles":[]}`, "notice.profiles_broken"},
		{"profiles bad role", storage.Paths.ProfilesFile, `{"format":1,"profiles":[{"username":"mum","display_name":"Mum","role":"boss","password_hash":"x"}]}`, "notice.profiles_broken"},
		{"settings not json", storage.Paths.SettingsFile, `[1,2`, "notice.settings_broken"},
		{"settings bad language", storage.Paths.SettingsFile, `{"format":1,"language":"fr"}`, "notice.settings_broken"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := storagetest.Isolate(t)
			path := tt.file(paths)
			write(t, path, tt.content)
			env, err := app.Start(context.Background(), startOptions(nil))
			if err != nil {
				t.Fatalf("a broken file must not stop the start: %v", err)
			}
			defer env.Close()

			moved := path + ".broken-20261030T101500Z"
			if data, err := os.ReadFile(moved); err != nil || string(data) != tt.content {
				t.Errorf("the broken file was not kept at %s (%q, %v)", moved, data, err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("the broken file is still in place: %v", err)
			}
			if len(env.Notices) != 1 || env.Notices[0].Key != tt.notice || env.Notices[0].Args[1] != moved {
				t.Errorf("notices = %+v, want %s with the path %s", env.Notices, tt.notice, moved)
			}
			for _, lang := range i18n.Langs() {
				if text := i18n.For(lang).Msg(env.Notices[0]); !strings.Contains(text, moved) {
					t.Errorf("the %s notice %q does not name the moved file", lang, text)
				}
			}
			if !env.Profiles.NeedsFirstAdmin() || len(env.Profiles.List()) != 0 {
				t.Error("the app did not start with no profiles")
			}
			if env.Settings.Get().Language == "" {
				t.Error("the app did not start with default settings")
			}
			if data, err := os.ReadFile(paths.LogFile()); err != nil || !strings.Contains(string(data), "set aside") {
				t.Errorf("the failure was not logged: %q, %v", data, err)
			}
		})
	}
}

func TestStartBothFilesBroken(t *testing.T) {
	paths := storagetest.Isolate(t)
	write(t, paths.ProfilesFile(), `x`)
	write(t, paths.SettingsFile(), `y`)
	env, err := app.Start(context.Background(), startOptions(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	if len(env.Notices) != 2 {
		t.Errorf("notices = %+v, want two", env.Notices)
	}
}

func TestStartKeepsGoodFiles(t *testing.T) {
	paths := storagetest.Isolate(t)
	write(t, paths.SettingsFile(), `{"format":1,"language":"el"}`)
	first, err := app.Start(context.Background(), startOptions(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Profiles.Create(context.Background(), "mum", "Mum", profile.RoleAdmin, "parent-secret"); err != nil {
		t.Fatal(err)
	}
	first.Close()
	env, err := app.Start(context.Background(), startOptions(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	if len(env.Notices) != 0 || env.Profiles.NeedsFirstAdmin() || env.Settings.Get().Language != i18n.EL {
		t.Errorf("good files were not read back (notices %v)", env.Notices)
	}
}

func TestStartFailures(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, paths storage.Paths) (context.Context, map[string]string)
		route string
	}{
		{"home unknown", func(t *testing.T, _ storage.Paths) (context.Context, map[string]string) {
			return context.Background(), map[string]string{"XDG_CONFIG_HOME": "", "XDG_DATA_HOME": "", "XDG_STATE_HOME": "", "HOME": ""}
		}, "Start->ResolveFrom"},
		{"cancelled context", func(t *testing.T, _ storage.Paths) (context.Context, map[string]string) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx, nil
		}, "openSettings->Open.SettingsOpenCancelled"},
		{"unreadable profiles file is not set aside", func(t *testing.T, paths storage.Paths) (context.Context, map[string]string) {
			if err := os.MkdirAll(paths.ProfilesFile(), 0o700); err != nil { // a folder where the file should be
				t.Fatal(err)
			}
			return context.Background(), nil
		}, "openProfiles->Open->ReadJSON.JSONReadFailed"},
		{"state folder is a file", func(t *testing.T, paths storage.Paths) (context.Context, map[string]string) {
			write(t, paths.State, "not a folder")
			return context.Background(), nil
		}, "Start->Lock->lock->EnsureDir.DirCreateFailed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := storagetest.Isolate(t)
			ctx, env := tt.setup(t, paths)
			got, err := app.Start(ctx, startOptions(env))
			if err == nil {
				got.Close()
				t.Fatal("Start succeeded")
			}
			hasRoute(t, err, tt.route)
			if _, statErr := os.Stat(paths.ProfilesFile() + ".broken-20261030T101500Z"); statErr == nil {
				t.Error("an unreadable file was set aside as broken")
			}
		})
	}
}

func TestStartReleasesTheLockWhenItFails(t *testing.T) {
	paths := storagetest.Isolate(t)
	if err := os.MkdirAll(paths.ProfilesFile(), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Start(context.Background(), startOptions(nil)); err == nil {
		t.Fatal("Start succeeded")
	}
	unlock, err := storage.Lock(paths)
	if err != nil {
		t.Fatalf("the lock is still held after a failed start: %v", err)
	}
	unlock()
}

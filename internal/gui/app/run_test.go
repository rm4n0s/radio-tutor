package app_test

import (
	"context"
	"testing"

	"github.com/rm4n0s/errors"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

// TestRunAppContextEndedContext proves, without a display, that a context that
// is already done ends Glutter's RunAppContext with nil before any window
// opens, for the two widgets Run shows (the root and the "already running"
// window). If a window were opened, the run would fail here for lack of a
// display. The end of a context while a real window is open is
// TestRunEndsWhenContextIsDone (build tag radiotutor_display) and the Wine
// smoke test.
func TestRunAppContextEndedContext(t *testing.T) {
	tests := []struct {
		name  string
		child func(t *testing.T) m.Widget
	}{
		{"root", func(t *testing.T) m.Widget {
			env, _ := testutil.NewEnv(t, testutil.Options{})
			return app.NewRoot(env)
		}},
		{"already running", func(*testing.T) m.Widget { return app.NewAlreadyRunning(i18n.FromLocale("en_US.UTF-8")) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := m.RunAppContext(ctx, tt.child(t)); err != nil {
				t.Fatalf("RunAppContext with an ended context = %v, want nil", err)
			}
		})
	}
}

// TestRunWithAlreadyRunningEndedContext proves that RunWith still returns the
// AlreadyRunning error (the command turns it into exit code 2) when another
// instance holds the lock and the context is done, so the "already running"
// window ends at once and does not hide the error.
func TestRunWithAlreadyRunningEndedContext(t *testing.T) {
	storagetest.Isolate(t)
	first, err := app.Start(context.Background(), app.StartOptions{})
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}
	defer first.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = app.RunWith(ctx, app.StartOptions{})
	appErr, ok := errors.FromError(err)
	if !ok || appErr.Tag != "AlreadyRunning" {
		t.Fatalf("RunWith = %v, want AlreadyRunning", err)
	}
}

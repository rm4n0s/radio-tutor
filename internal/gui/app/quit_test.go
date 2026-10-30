package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

// TestQuitOnDone proves that a cancelled context (SIGTERM, SIGINT) asks the
// platform to exit, for the real root and for the "already running" window,
// and that nothing exits while the context is alive.
func TestQuitOnDone(t *testing.T) {
	tests := []struct {
		name  string
		child func(t *testing.T) m.Widget
		text  string
	}{
		{"root", func(t *testing.T) m.Widget {
			env, _ := testutil.NewEnv(t, testutil.Options{})
			return app.NewRoot(env)
		}, "Create the parent account"},
		{"already running", func(*testing.T) m.Widget { return app.NewAlreadyRunning(i18n.FromLocale("en_US.UTF-8")) }, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.UseLinux(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ts := gluttertest.New(t, gluttertest.Opts{Size: testutil.DefaultSize})
			ts.PumpWidget(app.NewQuitOnDone(ctx, tt.child(t)))
			ts.PumpAndSettle()
			if tt.text != "" && !ts.Exists(gluttertest.ByText(tt.text)) {
				t.Fatalf("the child is not shown: no %q", tt.text)
			}
			// Give the watcher goroutine real time to (wrongly) act.
			time.Sleep(20 * time.Millisecond)
			ts.FlushPosted()
			ts.Pump(100 * time.Millisecond)
			if n := ts.ExitRequests(); n != 0 {
				t.Fatalf("exit requests = %d before the context is done, want 0", n)
			}

			cancel()
			deadline := time.Now().Add(5 * time.Second)
			for ts.ExitRequests() == 0 {
				if time.Now().After(deadline) {
					t.Fatal("the app did not ask to exit after the context was cancelled")
				}
				ts.FlushPosted()
				ts.Pump(10 * time.Millisecond)
				time.Sleep(2 * time.Millisecond)
			}
			if n := ts.ExitRequests(); n != 1 {
				t.Errorf("exit requests = %d, want 1", n)
			}
		})
	}
}

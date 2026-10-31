//go:build radiotutor_display

package app_test

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// TestRunEndsWhenContextIsDone opens a real window, cancels the context as
// SIGTERM does (Glutter's RunAppContext ends the run), and checks that RunWith
// returns without an error soon after (the bound is generous: window teardown
// takes about two seconds under Xvfb with software rendering on a busy machine)
// and that its deferred cleanups ran: the single-instance lock is free again.
// It needs a display; the storage folders are isolated.
func TestRunEndsWhenContextIsDone(t *testing.T) {
	storagetest.Isolate(t)
	runtime.LockOSThread() // RunApp wants the thread it was started on
	defer runtime.UnlockOSThread()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var cancelled atomic.Int64
	time.AfterFunc(2*time.Second, func() {
		cancelled.Store(time.Now().UnixNano())
		cancel()
	})
	err := app.RunWith(ctx, app.StartOptions{})
	if err != nil {
		t.Fatalf("RunWith returned %v, want nil", err)
	}
	at := cancelled.Load()
	if at == 0 {
		t.Fatal("RunWith returned before the context was cancelled")
	}
	if took := time.Since(time.Unix(0, at)); took > 10*time.Second {
		t.Errorf("RunWith returned %v after the context was cancelled, want less than 10 seconds", took)
	}
	env, err := app.Start(context.Background(), app.StartOptions{})
	if err != nil {
		t.Fatalf("Start after RunWith: %v (the lock was not released)", err)
	}
	env.Close()
}

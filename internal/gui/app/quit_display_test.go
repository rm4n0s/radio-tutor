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
// SIGTERM does, and checks that RunWith returns without an error within a
// second. It needs a display; the storage folders are isolated.
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
	if took := time.Since(time.Unix(0, at)); took > time.Second {
		t.Errorf("RunWith returned %v after the context was cancelled, want less than a second", took)
	}
}

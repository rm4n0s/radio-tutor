package submission

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestSystemClock(t *testing.T) {
	c := SystemClock()
	if d := time.Since(c.Now()); d < 0 || d > time.Minute {
		t.Errorf("Now() is %v off", d)
	}
	var fired atomic.Bool
	done := make(chan struct{})
	c.AfterFunc(time.Millisecond, func() { fired.Store(true); close(done) })
	select {
	case <-done:
	case <-time.After(eventWait):
		t.Fatal("the timer did not fire")
	}
	stopped := c.AfterFunc(time.Hour, func() { t.Error("a stopped timer fired") })
	if !stopped.Stop() || !fired.Load() {
		t.Error("Stop of a pending timer failed")
	}
}

func TestNewFillsTheDefaults(t *testing.T) {
	e := newEnv(t)
	svc := New(context.Background(), Options{Paths: e.paths, Progress: e.progress, Providers: e.providers, Settings: e.settings, Phone: e.phone})
	o := svc.opts
	if o.Clock == nil || o.Random == nil || o.Log == nil || o.DetectLAN == nil || o.BuildProvider == nil {
		t.Errorf("defaults missing: %+v", o)
	}
}

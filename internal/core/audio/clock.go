package audio

import (
	"sort"
	"sync"
	"time"
)

// Clock is the time source of the sinks and the Player. The real one is
// SystemClock; tests use a FakeClock they advance by hand, so no test waits
// for real time.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
	// AfterFunc calls f in a goroutine of its own after d has passed, like
	// time.AfterFunc.
	AfterFunc(d time.Duration, f func()) Timer
}

// Timer is a pending AfterFunc call.
type Timer interface {
	// Stop prevents the call if it has not happened yet and reports whether
	// it did so.
	Stop() bool
}

// SystemClock returns the Clock backed by the time package.
func SystemClock() Clock { return systemClock{} }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func (systemClock) AfterFunc(d time.Duration, f func()) Timer { return time.AfterFunc(d, f) }

// FakeClock is a Clock that only moves when Advance is called. It is safe for
// concurrent use. Timers fire inside Advance, in the order of their due
// times, on the goroutine that called Advance.
type FakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
	seq    int
}

// NewFakeClock returns a FakeClock that reads start.
func NewFakeClock(start time.Time) *FakeClock { return &FakeClock{now: start} }

// Now returns the fake time.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// AfterFunc schedules f to run when the fake time reaches now+d. A d of zero
// or less runs f at the next Advance (even Advance(0)).
func (c *FakeClock) AfterFunc(d time.Duration, f func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	t := &fakeTimer{clock: c, due: c.now.Add(d), seq: c.seq, f: f}
	c.timers = append(c.timers, t)
	return t
}

// Advance moves the fake time forward by d and runs every timer that falls
// due, in order. A timer sees the time it was due at while it runs; timers
// that a running timer schedules are run too when they fall inside d.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	target := c.now.Add(d)
	c.mu.Unlock()
	for {
		c.mu.Lock()
		sort.Slice(c.timers, func(i, j int) bool {
			a, b := c.timers[i], c.timers[j]
			if !a.due.Equal(b.due) {
				return a.due.Before(b.due)
			}
			return a.seq < b.seq
		})
		if len(c.timers) == 0 || c.timers[0].due.After(target) {
			c.now = target
			c.mu.Unlock()
			return
		}
		t := c.timers[0]
		c.timers = c.timers[1:]
		t.fired = true
		if t.due.After(c.now) {
			c.now = t.due
		}
		c.mu.Unlock()
		t.f()
	}
}

// Pending returns the number of timers that have neither fired nor been
// stopped.
func (c *FakeClock) Pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.timers)
}

type fakeTimer struct {
	clock *FakeClock
	due   time.Time
	seq   int
	f     func()
	fired bool
}

func (t *fakeTimer) Stop() bool {
	c := t.clock
	c.mu.Lock()
	defer c.mu.Unlock()
	if t.fired {
		return false
	}
	for i, o := range c.timers {
		if o == t {
			c.timers = append(c.timers[:i], c.timers[i+1:]...)
			t.fired = true
			return true
		}
	}
	return false
}

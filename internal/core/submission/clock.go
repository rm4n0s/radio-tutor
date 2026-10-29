package submission

import "time"

// Clock is the time source of the service: the time stamps and the two timers
// (the firewall hint and the expiry). The real one is SystemClock; tests use a
// fake they advance by hand. The audio package's FakeClock fits.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
	// AfterFunc calls f in a goroutine of its own after d, like
	// time.AfterFunc.
	AfterFunc(d time.Duration, f func()) Timer
}

// Timer is a pending AfterFunc call.
type Timer interface {
	// Stop prevents the call if it has not happened yet.
	Stop() bool
}

// SystemClock returns the Clock backed by the time package.
func SystemClock() Clock { return systemClock{} }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func (systemClock) AfterFunc(d time.Duration, f func()) Timer { return time.AfterFunc(d, f) }

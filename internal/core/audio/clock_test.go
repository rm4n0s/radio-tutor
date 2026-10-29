package audio

import (
	"slices"
	"testing"
	"time"
)

func TestFakeClock(t *testing.T) {
	t.Run("timers fire in due order at their own time", func(t *testing.T) {
		c := NewFakeClock(epoch)
		var got []string
		var at []time.Duration
		add := func(name string, d time.Duration) Timer {
			return c.AfterFunc(d, func() {
				got = append(got, name)
				at = append(at, c.Now().Sub(epoch))
			})
		}
		add("c", ms(300))
		add("a", ms(100))
		stopped := add("b", ms(200))
		add("a2", ms(100))
		if !stopped.Stop() {
			t.Fatal("Stop of a pending timer must return true")
		}
		if stopped.Stop() {
			t.Fatal("second Stop must return false")
		}
		c.Advance(ms(250))
		if want := []string{"a", "a2"}; !slices.Equal(got, want) {
			t.Fatalf("fired %v, want %v", got, want)
		}
		if c.Now().Sub(epoch) != ms(250) {
			t.Fatalf("now = %v", c.Now().Sub(epoch))
		}
		c.Advance(ms(100))
		if want := []string{"a", "a2", "c"}; !slices.Equal(got, want) {
			t.Fatalf("fired %v, want %v", got, want)
		}
		if want := []time.Duration{ms(100), ms(100), ms(300)}; !slices.Equal(at, want) {
			t.Fatalf("fired at %v, want %v", at, want)
		}
		if c.Pending() != 0 {
			t.Fatalf("pending = %d", c.Pending())
		}
	})

	t.Run("a timer scheduled by a timer fires inside the same Advance", func(t *testing.T) {
		c := NewFakeClock(epoch)
		n := 0
		var tick func()
		tick = func() {
			n++
			c.AfterFunc(ms(10), tick)
		}
		c.AfterFunc(ms(10), tick)
		c.Advance(ms(55))
		if n != 5 {
			t.Fatalf("ticks = %d, want 5", n)
		}
	})

	t.Run("a fired timer cannot be stopped", func(t *testing.T) {
		c := NewFakeClock(epoch)
		tm := c.AfterFunc(0, func() {})
		c.Advance(0)
		if tm.Stop() {
			t.Fatal("Stop after firing must return false")
		}
	})
}

func TestSystemClock(t *testing.T) {
	c := SystemClock()
	before := time.Now()
	if c.Now().Before(before) {
		t.Fatal("Now is in the past")
	}
	fired := make(chan struct{})
	c.AfterFunc(time.Millisecond, func() { close(fired) })
	select {
	case <-fired:
	case <-time.After(5 * time.Second):
		t.Fatal("timer did not fire")
	}
	if !c.AfterFunc(time.Hour, func() {}).Stop() {
		t.Fatal("Stop of a pending system timer must return true")
	}
}

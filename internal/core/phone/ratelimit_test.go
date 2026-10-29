package phone

import (
	"fmt"
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	clock := newFakeClock()
	l := newLimiter(clock.Now, 30, 100)
	for i := range 30 {
		if ok, _ := l.allow("a"); !ok {
			t.Fatalf("request %d refused", i+1)
		}
	}
	ok, wait := l.allow("a")
	if ok {
		t.Fatal("31st request allowed")
	}
	if wait <= 0 || wait > 2*time.Second+time.Millisecond {
		t.Errorf("wait = %v, want about 2s", wait)
	}
	if ok, _ := l.allow("b"); !ok {
		t.Error("another address is limited")
	}
	clock.Advance(2 * time.Second)
	if ok, _ := l.allow("a"); !ok {
		t.Error("not allowed after the refill")
	}
	if ok, _ := l.allow("a"); ok {
		t.Error("allowed twice after one refill")
	}
	// A full minute of rest gives the whole bucket back, never more.
	clock.Advance(10 * time.Minute)
	n := 0
	for ok, _ := l.allow("a"); ok && n < 100; ok, _ = l.allow("a") {
		n++
	}
	if n != 30 {
		t.Errorf("%d requests after a long rest, want 30", n)
	}
}

func TestLimiterMemoryIsBounded(t *testing.T) {
	clock := newFakeClock()
	l := newLimiter(clock.Now, 30, 4)
	// Four addresses that used one request each.
	for i := range 4 {
		l.allow(fmt.Sprintf("a%d", i))
		clock.Advance(time.Second)
	}
	// Long enough for all of them to refill: they are dropped for the new one.
	clock.Advance(time.Minute)
	l.allow("new")
	if len(l.buckets) != 1 {
		t.Errorf("%d buckets, want 1 (refilled ones are dropped)", len(l.buckets))
	}

	// Four busy addresses: the one used longest ago makes room.
	l = newLimiter(clock.Now, 30, 4)
	for i := range 4 {
		for range 30 {
			l.allow(fmt.Sprintf("b%d", i))
		}
		clock.Advance(time.Millisecond)
	}
	l.allow("c")
	if len(l.buckets) != 4 {
		t.Errorf("%d buckets, want 4", len(l.buckets))
	}
	if _, kept := l.buckets["b0"]; kept {
		t.Error("the oldest address was kept")
	}
	for i := range 1000 {
		l.allow(fmt.Sprintf("flood%d", i))
		if len(l.buckets) > 4 {
			t.Fatalf("%d buckets after %d addresses", len(l.buckets), i+1)
		}
	}
}

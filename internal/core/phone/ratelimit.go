package phone

import (
	"sync"
	"time"
)

// limiter is a token bucket per remote address: a bucket holds up to perMinute
// requests and refills at perMinute per minute, so an address makes at most
// perMinute requests in any minute (plus the first burst). Its memory is
// bounded by maxAddrs: when it is full, the buckets that have refilled
// completely (which are the same as no bucket) are dropped, then the one that
// was used the longest ago.
type limiter struct {
	now       func() time.Time
	perMinute float64
	maxAddrs  int

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newLimiter(now func() time.Time, perMinute, maxAddrs int) *limiter {
	return &limiter{
		now:       now,
		perMinute: float64(perMinute),
		maxAddrs:  maxAddrs,
		buckets:   map[string]*bucket{},
	}
}

// allow takes one request of the address. When there is none left it returns
// false and how long to wait.
func (l *limiter) allow(addr string) (bool, time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[addr]
	if !ok {
		if len(l.buckets) >= l.maxAddrs {
			l.evict(now)
		}
		b = &bucket{tokens: l.perMinute, last: now}
		l.buckets[addr] = b
	} else {
		l.refill(b, now)
	}
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	perSecond := l.perMinute / 60
	wait := time.Duration((1 - b.tokens) / perSecond * float64(time.Second))
	return false, wait
}

// level is the tokens the bucket holds at now, with the refill since it was
// last used.
func (l *limiter) level(b *bucket, now time.Time) float64 {
	if elapsed := now.Sub(b.last); elapsed > 0 {
		return min(l.perMinute, b.tokens+elapsed.Seconds()*l.perMinute/60)
	}
	return b.tokens
}

// refill brings the bucket up to date.
func (l *limiter) refill(b *bucket, now time.Time) {
	b.tokens = l.level(b, now)
	if now.After(b.last) {
		b.last = now
	}
}

// evict makes room for one more address. The caller holds l.mu.
func (l *limiter) evict(now time.Time) {
	var oldest string
	var oldestTime time.Time
	for addr, b := range l.buckets {
		if l.level(b, now) >= l.perMinute { // refilled: same as no bucket
			delete(l.buckets, addr)
			continue
		}
		if oldest == "" || b.last.Before(oldestTime) {
			oldest, oldestTime = addr, b.last
		}
	}
	if len(l.buckets) >= l.maxAddrs {
		delete(l.buckets, oldest)
	}
}

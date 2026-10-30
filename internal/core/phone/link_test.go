package phone

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
)

// getState asks for the state of a link, knowing version v (0: no version).
func (ts *testServer) getState(t *testing.T, token string, v uint64) (int, stateView) {
	t.Helper()
	path := "/p/" + token + "/state"
	if v > 0 {
		path += "?v=" + strconv.FormatUint(v, 10)
	}
	status, body, hdr := ts.get(t, path)
	if ct := hdr.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("state Content-Type = %q", ct)
	}
	var st stateView
	if err := json.Unmarshal([]byte(body), &st); err != nil {
		t.Fatalf("state %q is not JSON: %v", body, err)
	}
	return status, st
}

func TestLinkIsReusedAcrossOffers(t *testing.T) {
	ts := startServer(t, Options{})
	k := ts.newLink(t)
	ev1, ev2 := &events{}, &events{}

	if status, st := ts.getState(t, k.Token, 0); status != 200 || st.State != "waiting" || st.V != 1 {
		t.Fatalf("a new link: %d %+v", status, st)
	}
	ts.offer(t, k, ev1.handlers())
	status, st := ts.getState(t, k.Token, 0)
	if status != 200 || st.State != "ready" || st.Offer != 1 || st.Title != "Cassette" || len(st.Exercises) != 1 {
		t.Fatalf("after the first offer: %d %+v", status, st)
	}
	if code, body := ts.upload(t, k.Token, photoParts(testJPEG(t, 20, 20), 1)); code != 200 {
		t.Fatalf("first upload = %d %s", code, body)
	}
	_, st = ts.getState(t, k.Token, 0)
	if st.State != "waiting" || st.Title != "Cassette" || st.V <= 2 {
		t.Fatalf("after the first upload the page waits: %+v", st)
	}

	// The same link takes the next submission, with no new token.
	ts.offer(t, k, ev2.handlers())
	_, st = ts.getState(t, k.Token, 0)
	if st.State != "ready" || st.Offer != 2 {
		t.Fatalf("after the second offer: %+v", st)
	}
	if code, body := ts.upload(t, k.Token, photoParts(testJPEG(t, 20, 20), 2)); code != 200 {
		t.Fatalf("second upload = %d %s", code, body)
	}
	if _, _, r1 := ev1.counts(); r1 != 1 {
		t.Errorf("first handlers received %d uploads", r1)
	}
	if _, _, r2 := ev2.counts(); r2 != 1 || len(ev2.received[0]) != 2 {
		t.Errorf("second handlers received %d uploads", r2)
	}
	if !k.Alive() {
		t.Error("the link ended")
	}
}

func TestVersionGrowsWithEveryChange(t *testing.T) {
	ts := startServer(t, Options{})
	k := ts.newLink(t)
	var seen []uint64
	note := func() {
		_, st := ts.getState(t, k.Token, 0)
		seen = append(seen, st.V)
	}
	note()
	o1 := ts.offer(t, k, Handlers{})
	note()
	o1.Withdraw()
	note()
	o1.Withdraw() // nothing changes
	note()
	ts.offer(t, k, Handlers{})
	note()
	ts.offer(t, k, Handlers{}) // replaces the waiting one
	note()
	want := []uint64{1, 2, 3, 3, 4, 5}
	if len(seen) != len(want) {
		t.Fatalf("seen %v", seen)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Errorf("versions = %v, want %v", seen, want)
			break
		}
	}
}

func TestLinkIdleTime(t *testing.T) {
	const ttl = 60 * time.Minute
	ts := startServer(t, Options{})
	alive := func(k *Link) bool {
		status, _, _ := ts.get(t, "/p/"+k.Token)
		return status == http.StatusOK
	}

	t.Run("an unused link", func(t *testing.T) {
		k := ts.newLink(t)
		ts.clock.Advance(ttl - time.Second)
		if !alive(k) || !k.Alive() {
			t.Fatal("the link ended before 60 minutes")
		}
		ts.clock.Advance(time.Second)
		if alive(k) || k.Alive() {
			t.Fatal("the link is alive after 60 minutes")
		}
	})
	t.Run("a waiting offer keeps it", func(t *testing.T) {
		k := ts.newLink(t)
		ts.offer(t, k, Handlers{})
		ts.clock.Advance(3 * ttl)
		if !alive(k) {
			t.Fatal("the link ended with an offer waiting")
		}
	})
	t.Run("the time counts from the last send", func(t *testing.T) {
		k := ts.newLink(t)
		ts.clock.Advance(ttl - time.Minute)
		ts.offer(t, k, Handlers{})
		ts.clock.Advance(20 * time.Minute) // photos take a while
		if code, body := ts.upload(t, k.Token, photoParts(testJPEG(t, 20, 20), 1)); code != 200 {
			t.Fatalf("upload = %d %s", code, body)
		}
		ts.clock.Advance(ttl - time.Second)
		if !alive(k) {
			t.Fatal("the link ended before 60 minutes after the send")
		}
		ts.clock.Advance(time.Second)
		if alive(k) {
			t.Fatal("the link is alive 60 minutes after the send")
		}
	})
	t.Run("a withdrawn offer starts the time too", func(t *testing.T) {
		k := ts.newLink(t)
		o := ts.offer(t, k, Handlers{})
		ts.clock.Advance(2 * ttl)
		o.Withdraw()
		ts.clock.Advance(ttl - time.Second)
		if !alive(k) {
			t.Fatal("the link ended early")
		}
		ts.clock.Advance(time.Second)
		if alive(k) {
			t.Fatal("the link is alive 60 minutes after the withdrawal")
		}
	})
}

func TestLinkEnd(t *testing.T) {
	ts := startServer(t, Options{})
	k := ts.newLink(t)
	ts.offer(t, k, Handlers{})
	k.End()
	k.End() // does nothing
	if k.Alive() || k.Seen() {
		t.Error("an ended link is alive")
	}
	if status, body, _ := ts.get(t, "/p/"+k.Token); status != http.StatusGone || !strings.Contains(body, "This link has expired") {
		t.Errorf("page = %d %q", status, body)
	}
	if status, st := ts.getState(t, k.Token, 0); status != http.StatusGone || !st.Expired {
		t.Errorf("state = %d %+v", status, st)
	}
	if status, body := ts.upload(t, k.Token, photoParts(testJPEG(t, 20, 20), 1)); status != http.StatusGone || !strings.Contains(body, `"expired":true`) {
		t.Errorf("upload = %d %s", status, body)
	}
	_, _, err := k.Offer(testPage, Handlers{})
	wantRoute(t, err, "Link.Offer.PhoneLinkEnded")
}

func TestOfferOnAnIdleLink(t *testing.T) {
	ts := startServer(t, Options{LinkIdleTTL: time.Minute})
	k := ts.newLink(t)
	ts.clock.Advance(time.Minute)
	_, _, err := k.Offer(testPage, Handlers{})
	wantRoute(t, err, "Link.Offer.PhoneLinkEnded")
}

func TestStaleUploads(t *testing.T) {
	jpg := testJPEG(t, 20, 20)
	tests := []struct {
		name  string
		setup func(ts *testServer, k *Link) (offer uint64) // returns the offer number to upload for
	}{
		{"cancelled", func(ts *testServer, k *Link) uint64 {
			o := ts.offer(t, k, Handlers{})
			o.Withdraw()
			return o.ID()
		}},
		{"already sent", func(ts *testServer, k *Link) uint64 {
			o := ts.offer(t, k, Handlers{})
			if status, _ := ts.uploadOffer(t, k.Token, o.ID(), photoParts(jpg, 1)); status != 200 {
				t.Fatalf("first upload = %d", status)
			}
			return o.ID()
		}},
		{"replaced by a newer one", func(ts *testServer, k *Link) uint64 {
			old := ts.offer(t, k, Handlers{})
			ts.offer(t, k, Handlers{})
			return old.ID()
		}},
		{"an offer that never was", func(ts *testServer, k *Link) uint64 {
			ts.offer(t, k, Handlers{})
			return 99
		}},
		{"nothing is waiting", func(ts *testServer, k *Link) uint64 { return 1 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := startServer(t, Options{})
			k := ts.newLink(t)
			id := tc.setup(ts, k)
			status, body := ts.uploadOffer(t, k.Token, id, photoParts(jpg, 1))
			if status != http.StatusConflict || !strings.Contains(body, `"stale":true`) || !strings.Contains(body, "not waiting for photos any more") {
				t.Errorf("upload = %d %s, want 409 stale", status, body)
			}
		})
	}
	t.Run("no offer number", func(t *testing.T) {
		ts := startServer(t, Options{})
		k := ts.newLink(t)
		ts.offer(t, k, Handlers{})
		body, ct := multipartBody(t, photoParts(jpg, 1))
		if status, _ := ts.post(t, "/p/"+k.Token+"/photos", ct, body); status != http.StatusConflict {
			t.Errorf("upload without an offer = %d, want 409", status)
		}
	})
	t.Run("the current one still works after a stale try", func(t *testing.T) {
		ts := startServer(t, Options{})
		k := ts.newLink(t)
		ev := &events{}
		o := ts.offer(t, k, ev.handlers())
		if status, _ := ts.uploadOffer(t, k.Token, o.ID()+1, photoParts(jpg, 1)); status != http.StatusConflict {
			t.Fatalf("stale upload = %d", status)
		}
		if status, _ := ts.uploadOffer(t, k.Token, o.ID(), photoParts(jpg, 1)); status != http.StatusOK {
			t.Errorf("current upload = %d, want 200", status)
		}
	})
}

func TestLongPoll(t *testing.T) {
	t.Run("answers at once when the version differs", func(t *testing.T) {
		ts := startServer(t, Options{PollTimeout: time.Minute})
		k := ts.newLink(t)
		ts.offer(t, k, Handlers{}) // version 2
		start := time.Now()
		status, st := ts.getState(t, k.Token, 1)
		if status != 200 || st.V != 2 || st.State != "ready" {
			t.Errorf("state = %d %+v", status, st)
		}
		if time.Since(start) > 5*time.Second {
			t.Error("the request was held")
		}
		// A version that is not a number is "unknown": answered at once.
		if status, body, _ := ts.get(t, "/p/"+k.Token+"/state?v=abc"); status != 200 || !strings.Contains(body, `"v":2`) {
			t.Errorf("bad version = %d %s", status, body)
		}
	})
	t.Run("holds until the state changes", func(t *testing.T) {
		ts := startServer(t, Options{PollTimeout: time.Minute})
		k := ts.newLink(t)
		type answer struct {
			status int
			st     stateView
		}
		got := make(chan answer, 1)
		go func() {
			status, st := ts.getState(t, k.Token, 1)
			got <- answer{status, st}
		}()
		waitFor(t, func() bool { return k.Seen() }) // the request is in
		select {
		case a := <-got:
			t.Fatalf("answered before anything changed: %+v", a)
		case <-time.After(100 * time.Millisecond):
		}
		ts.offer(t, k, Handlers{})
		select {
		case a := <-got:
			if a.status != 200 || a.st.State != "ready" || a.st.V != 2 {
				t.Errorf("answer = %+v", a)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the held request was not woken by the offer")
		}
	})
	t.Run("answers the same state after the timeout", func(t *testing.T) {
		ts := startServer(t, Options{PollTimeout: 50 * time.Millisecond})
		k := ts.newLink(t)
		start := time.Now()
		status, st := ts.getState(t, k.Token, 1)
		if status != 200 || st.V != 1 || st.State != "waiting" {
			t.Errorf("state = %d %+v", status, st)
		}
		if d := time.Since(start); d < 40*time.Millisecond {
			t.Errorf("answered after %v, before the timeout", d)
		}
	})
	t.Run("a link that went idle during the hold answers 410", func(t *testing.T) {
		ts := startServer(t, Options{PollTimeout: 200 * time.Millisecond, LinkIdleTTL: time.Hour})
		k := ts.newLink(t)
		got := make(chan int, 1)
		go func() {
			status, _ := ts.getState(t, k.Token, 1)
			got <- status
		}()
		waitFor(t, func() bool { return k.Seen() })
		ts.clock.Advance(time.Hour)
		if status := <-got; status != http.StatusGone {
			t.Errorf("status = %d, want 410", status)
		}
	})
	t.Run("woken by the end of the link", func(t *testing.T) {
		ts := startServer(t, Options{PollTimeout: time.Minute})
		k := ts.newLink(t)
		got := make(chan int, 1)
		go func() {
			status, _ := ts.getState(t, k.Token, 1)
			got <- status
		}()
		waitFor(t, func() bool { return k.Seen() })
		k.End()
		select {
		case status := <-got:
			if status != http.StatusGone {
				t.Errorf("status = %d, want 410", status)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("not woken by End")
		}
	})
	t.Run("woken by a withdrawn offer", func(t *testing.T) {
		ts := startServer(t, Options{PollTimeout: time.Minute})
		k := ts.newLink(t)
		o := ts.offer(t, k, Handlers{}) // version 2
		got := make(chan stateView, 1)
		go func() {
			_, st := ts.getState(t, k.Token, 2)
			got <- st
		}()
		waitFor(t, func() bool { return k.Seen() })
		o.Withdraw()
		select {
		case st := <-got:
			if st.State != "waiting" || st.V != 3 {
				t.Errorf("state = %+v", st)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("not woken by Withdraw")
		}
	})
	t.Run("a phone that hangs up frees the request", func(t *testing.T) {
		ts := startServer(t, Options{PollTimeout: time.Minute})
		k := ts.newLink(t)
		req, err := http.NewRequest(http.MethodGet, ts.base+"/p/"+k.Token+"/state?v=1", nil)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := contextWithCancel(t)
		req = req.WithContext(ctx)
		done := make(chan struct{})
		go func() {
			defer close(done)
			if resp, err := http.DefaultClient.Do(req); err == nil {
				_ = resp.Body.Close()
			}
		}()
		waitFor(t, func() bool { k.l.mu.Lock(); defer k.l.mu.Unlock(); return k.l.polling == 1 })
		cancel()
		<-done
		waitFor(t, func() bool { k.l.mu.Lock(); defer k.l.mu.Unlock(); return k.l.polling == 0 })
	})
}

func TestSeen(t *testing.T) {
	ts := startServer(t, Options{PollTimeout: time.Minute})
	k := ts.newLink(t)
	if k.Seen() {
		t.Fatal("a phone was seen before any request")
	}
	ts.get(t, "/p/"+k.Token)
	if !k.Seen() {
		t.Error("the page was opened but no phone is seen")
	}
	ts.clock.Advance(SeenWithin)
	if !k.Seen() {
		t.Error("30 seconds after the last request the phone is still seen")
	}
	ts.clock.Advance(time.Second)
	if k.Seen() {
		t.Error("the phone is seen 31 seconds after its last request")
	}
	// A request that is held counts as a phone that is there, however long.
	got := make(chan struct{})
	go func() {
		ts.getState(t, k.Token, 1)
		close(got)
	}()
	waitFor(t, func() bool { return k.Seen() })
	ts.clock.Advance(10 * time.Minute)
	if !k.Seen() {
		t.Error("a phone that is waiting for news is not seen")
	}
	k.End()
	<-got
}

func TestOpenedIsCalledOncePerOffer(t *testing.T) {
	ts := startServer(t, Options{})
	k := ts.newLink(t)

	// A phone that is not there: the first request after the offer calls it.
	var n atomic.Int32
	h := Handlers{Opened: func() { n.Add(1) }}
	_, seen, err := k.Offer(testPage, h)
	if err != nil || seen {
		t.Fatalf("Offer: seen %v, err %v", seen, err)
	}
	ts.get(t, "/p/"+k.Token)
	ts.getState(t, k.Token, 1)
	ts.get(t, "/p/"+k.Token)
	if n.Load() != 1 {
		t.Errorf("Opened called %d times, want 1", n.Load())
	}

	// A phone that is there already when the offer is made: seen, and Opened
	// is not called.
	var m atomic.Int32
	_, seen, err = k.Offer(testPage, Handlers{Opened: func() { m.Add(1) }})
	if err != nil || !seen {
		t.Fatalf("Offer: seen %v, err %v", seen, err)
	}
	ts.get(t, "/p/"+k.Token)
	if m.Load() != 0 {
		t.Errorf("Opened called %d times for an offer made with a phone there", m.Load())
	}

	// A phone that comes while the offer waits, on a held request, calls it.
	ts.clock.Advance(time.Hour)
	k2 := ts.newLink(t)
	var p atomic.Int32
	ts.offer(t, k2, Handlers{Opened: func() { p.Add(1) }})
	ts.getState(t, k2.Token, 99)
	if p.Load() != 1 {
		t.Errorf("Opened called %d times from a state request, want 1", p.Load())
	}
}

func TestAnOpenPageStaysFarBelowTheRateLimit(t *testing.T) {
	// A page that is only open asks for news about every PollTimeout. Over ten
	// minutes of the fake clock, with a limit of 5 requests a minute (the real
	// one is 30), a phone that is open makes no request that is refused, and
	// the limit still bites a client that asks as fast as it can.
	ts := startServer(t, Options{PollTimeout: 5 * time.Millisecond, RequestsPerMinute: 5})
	k := ts.newLink(t)
	if status, _, _ := ts.get(t, "/p/"+k.Token); status != 200 {
		t.Fatalf("page = %d", status)
	}
	v := uint64(1)
	var requests, limited int
	var perMinute [11]int
	start := ts.clock.Now()
	for ts.clock.Now().Sub(start) < 10*time.Minute {
		status, st := ts.getState(t, k.Token, v)
		requests++
		perMinute[int(ts.clock.Now().Sub(start)/time.Minute)]++
		if status == http.StatusTooManyRequests {
			limited++
			continue
		}
		v = st.V
		ts.clock.Advance(PollTimeout) // the real hold is 25 s; the test does not wait for it
	}
	if limited != 0 {
		t.Errorf("%d of %d requests were refused", limited, requests)
	}
	for minute, n := range perMinute {
		if n > 3 {
			t.Errorf("minute %d: %d requests", minute, n)
		}
	}
	// The same bucket refuses a client that does not wait.
	for range 10 {
		if status, _, _ := ts.get(t, "/p/"+k.Token+"/state?v=1"); status == http.StatusTooManyRequests {
			return
		}
	}
	t.Error("a client that asks without waiting was never limited")
}

func TestPhoneTextIsInTheLinksLanguage(t *testing.T) {
	ts := startServer(t, Options{})
	k, err := ts.OpenLink("127.0.0.1", "xx") // an invalid language falls back to English
	if err != nil {
		t.Fatal(err)
	}
	_, body, _ := ts.get(t, "/p/"+k.Token)
	if !strings.Contains(body, `<html lang="en">`) {
		t.Errorf("invalid language did not fall back to English: %.200s", body)
	}
	if k.l.lang != i18n.EN {
		t.Errorf("lang = %q", k.l.lang)
	}
}

//go:build !windows

// These tests talk to the fake PulseAudio server of pulsetest over a Unix socket.
// Radio Tutor plays through winmm on Windows (SPEC D23), so the PulseAudio client is
// not used there, and Wine's AF_UNIX sockets reset the connection of this server.

package pulse

import (
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/pulse/proto"
	"github.com/rm4n0s/radio-tutor/internal/core/pulse/pulsetest"
)

// waitFor is how long a test waits for something that must happen. A
// regression of the deadlocks these tests guard against shows up as a failure
// after this time, not as a hanging test.
const waitFor = 5 * time.Second

// newTestClient connects a client to the fake server. The cookie is looked up
// in the test's own folder, never in the user's.
func newTestClient(t *testing.T, srv *pulsetest.Server, timeout time.Duration) *Client {
	t.Helper()
	t.Setenv("PULSE_COOKIE", filepath.Join(t.TempDir(), "no-cookie"))
	c, err := NewClient(ClientServerString(srv.Addr()), ClientTimeout(timeout), ClientApplicationName("pulse test"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(c.Close)
	return c
}

// within runs f and fails the test if it does not return in time.
func within(t *testing.T, what string, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(waitFor):
		t.Fatalf("%s did not return within %v", what, waitFor)
	}
}

// eventually polls cond until it is true or fails the test.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitFor)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// silence is a reader that always has data.
func silence() Reader {
	return Float32Reader(func(buf []float32) (int, error) {
		clear(buf)
		return len(buf), nil
	})
}

// queryLatency is a request that needs the server's answer: it only returns
// nil when the connection's read loop is alive.
func queryLatency(c *Client, s *PlaybackStream) error {
	var rep proto.GetPlaybackLatencyReply
	return c.RawRequest(&proto.GetPlaybackLatency{StreamIndex: s.StreamIndex()}, &rep)
}

func TestPlaybackStreamControl(t *testing.T) {
	cases := []struct {
		name string
		ops  []string
	}{
		{"start", []string{"start"}},
		{"pause and resume", []string{"start", "pause", "resume"}},
		// The regression: the server answers every uncork with a "started" event.
		{"two pauses and resumes", []string{"start", "pause", "resume", "pause", "resume"}},
		{"many pauses and resumes", []string{"start", "pause", "resume", "pause", "resume", "pause", "resume", "pause", "resume"}},
		{"stop and start again", []string{"start", "stop", "start", "pause", "resume"}},
		{"resume without pause", []string{"start", "resume", "resume"}},
		{"pause twice", []string{"start", "pause", "pause", "resume"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := pulsetest.NewServer(t)
			// A real server sends "started" a moment after it answered the uncork.
			srv.StartedDelay(5 * time.Millisecond)
			c := newTestClient(t, srv, 2*time.Second)
			stream, err := c.NewPlayback(silence())
			if err != nil {
				t.Fatalf("NewPlayback: %v", err)
			}
			t.Cleanup(stream.Close)

			for _, op := range tc.ops {
				within(t, op, func() {
					switch op {
					case "start":
						stream.Start()
					case "stop":
						stream.Stop()
					case "pause":
						stream.Pause()
					case "resume":
						stream.Resume()
					}
				})
				// Let the "started" event of an uncork arrive.
				time.Sleep(20 * time.Millisecond)
			}

			within(t, "the latency query", func() {
				if err := queryLatency(c, stream); err != nil {
					t.Errorf("the connection does not answer after %v: %v", tc.ops, err)
				}
			})
			if err := stream.Error(); err != nil {
				t.Errorf("stream error = %v", err)
			}
		})
	}
}

func TestPlaybackPauseCorksAndResumeUncorks(t *testing.T) {
	srv := pulsetest.NewServer(t)
	c := newTestClient(t, srv, 2*time.Second)
	stream, err := c.NewPlayback(silence())
	if err != nil {
		t.Fatalf("NewPlayback: %v", err)
	}
	t.Cleanup(stream.Close)
	idx := stream.StreamIndex()

	cases := []struct {
		name       string
		do         func()
		wantCorked bool
		wantState  func() bool
	}{
		{"created corked", func() {}, true, func() bool { return !stream.Running() }},
		{"start", stream.Start, false, stream.Running},
		{"pause", stream.Pause, true, func() bool { return !stream.Running() }},
		{"resume", stream.Resume, false, stream.Running},
		{"pause again", stream.Pause, true, func() bool { return !stream.Running() }},
	}
	for _, tc := range cases {
		within(t, tc.name, tc.do)
		if got := srv.Corked(idx); got != tc.wantCorked {
			t.Errorf("%s: server corked = %v, want %v", tc.name, got, tc.wantCorked)
		}
		if !tc.wantState() {
			t.Errorf("%s: unexpected Running() = %v", tc.name, stream.Running())
		}
	}
}

func TestPlaybackPauseHoldsTheServersRequest(t *testing.T) {
	srv := pulsetest.NewServer(t)
	c := newTestClient(t, srv, 2*time.Second)
	var reads atomic.Int64
	stream, err := c.NewPlayback(Float32Reader(func(buf []float32) (int, error) {
		reads.Add(1)
		clear(buf)
		return len(buf), nil
	}))
	if err != nil {
		t.Fatalf("NewPlayback: %v", err)
	}
	t.Cleanup(stream.Close)
	idx := stream.StreamIndex()

	within(t, "Start", stream.Start)
	target := stream.BufferSizeBytes()
	eventually(t, "the first data", func() bool { return srv.Received(idx) >= target })

	within(t, "Pause", stream.Pause)
	before := srv.Received(idx)
	srv.SendRequest(idx, 1000)
	time.Sleep(50 * time.Millisecond)
	if got := srv.Received(idx); got != before {
		t.Fatalf("data was written while the stream was paused: %d bytes, then %d", before, got)
	}

	// The server asked once and does not ask again: the bytes it asked for
	// while the stream was corked are written when it is uncorked.
	within(t, "Resume", stream.Resume)
	eventually(t, "the held request to be written", func() bool { return srv.Received(idx) >= before+1000 })
	if got := srv.Received(idx); got != before+1000 {
		t.Errorf("received %d bytes, want %d", got, before+1000)
	}
}

func TestPlaybackStopDropsTheServersRequest(t *testing.T) {
	srv := pulsetest.NewServer(t)
	c := newTestClient(t, srv, 2*time.Second)
	stream, err := c.NewPlayback(silence())
	if err != nil {
		t.Fatalf("NewPlayback: %v", err)
	}
	t.Cleanup(stream.Close)
	idx := stream.StreamIndex()

	within(t, "Start", stream.Start)
	eventually(t, "the first data", func() bool { return srv.Received(idx) >= stream.BufferSizeBytes() })
	within(t, "Stop", stream.Stop)
	before := srv.Received(idx)
	srv.SendRequest(idx, 1000)
	time.Sleep(50 * time.Millisecond)
	if got := srv.Received(idx); got != before {
		t.Errorf("data was written after Stop: %d bytes, then %d", before, got)
	}
}

// TestPlaybackReaderDoesNotBlockTheConnection is the second form of the
// deadlock: the server's requests for data were handed to the stream's
// goroutine over an unbuffered channel, so a reader that takes its time (or
// is stuck) stopped the connection's read loop.
func TestPlaybackReaderDoesNotBlockTheConnection(t *testing.T) {
	srv := pulsetest.NewServer(t)
	c := newTestClient(t, srv, 2*time.Second)
	gate := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(gate) }) })
	inRead := make(chan struct{}, 1)
	var calls atomic.Int64
	stream, err := c.NewPlayback(Float32Reader(func(buf []float32) (int, error) {
		if calls.Add(1) == 2 {
			inRead <- struct{}{}
			<-gate
		}
		clear(buf)
		return len(buf), nil
	}))
	if err != nil {
		t.Fatalf("NewPlayback: %v", err)
	}
	t.Cleanup(stream.Close)
	idx := stream.StreamIndex()

	within(t, "Start", stream.Start)
	eventually(t, "the first data", func() bool { return srv.Received(idx) >= stream.BufferSizeBytes() })
	srv.SendRequest(idx, 512)
	select {
	case <-inRead:
	case <-time.After(waitFor):
		t.Fatal("the reader was not called again")
	}
	// The reader is stuck; the server keeps asking.
	for range 50 {
		srv.SendRequest(idx, 512)
	}
	within(t, "the latency query while the reader is stuck", func() {
		if err := queryLatency(c, stream); err != nil {
			t.Errorf("the connection does not answer while the reader is stuck: %v", err)
		}
	})
	once.Do(func() { close(gate) })
	// All the requests are written once the reader is free.
	want := stream.BufferSizeBytes() + 512 + 50*512
	eventually(t, "the requests to be written", func() bool { return srv.Received(idx) >= want })
}

func TestPlaybackEvents(t *testing.T) {
	cases := []struct {
		name string
		send func(srv *pulsetest.Server, idx uint32)
	}{
		{"many started events", func(srv *pulsetest.Server, idx uint32) {
			for range 20 {
				srv.SendStarted(idx)
			}
		}},
		{"underflow then started", func(srv *pulsetest.Server, idx uint32) {
			srv.SendUnderflow(idx)
			for range 5 {
				srv.SendStarted(idx)
			}
		}},
		{"events for an unknown stream", func(srv *pulsetest.Server, idx uint32) {
			srv.SendStarted(idx + 100)
			srv.SendRequest(idx+100, 100)
			srv.SendUnderflow(idx + 100)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := pulsetest.NewServer(t)
			c := newTestClient(t, srv, 2*time.Second)
			stream, err := c.NewPlayback(silence())
			if err != nil {
				t.Fatalf("NewPlayback: %v", err)
			}
			t.Cleanup(stream.Close)
			within(t, "Start", stream.Start)
			tc.send(srv, stream.StreamIndex())
			within(t, "the latency query", func() {
				if err := queryLatency(c, stream); err != nil {
					t.Errorf("the connection does not answer: %v", err)
				}
			})
		})
	}
}

func TestPlaybackReaderResults(t *testing.T) {
	boom := errors.New("ReaderBoom", "the reader failed")
	cases := []struct {
		name    string
		result  error
		wantErr error
	}{
		{"end of data", EndOfData, nil},
		{"reader error", boom, boom},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := pulsetest.NewServer(t)
			c := newTestClient(t, srv, 2*time.Second)
			stream, err := c.NewPlayback(Float32Reader(func(buf []float32) (int, error) { return 0, tc.result }))
			if err != nil {
				t.Fatalf("NewPlayback: %v", err)
			}
			t.Cleanup(stream.Close)
			within(t, "Start", stream.Start)
			eventually(t, "the stream to stop", func() bool { return !stream.Running() })
			if got := stream.Error(); got != tc.wantErr {
				t.Errorf("Error() = %v, want %v", got, tc.wantErr)
			}
			// The stream can be started again.
			within(t, "the second Start", stream.Start)
			eventually(t, "the second stop", func() bool { return !stream.Running() })
		})
	}
}

func TestPlaybackConnectionLost(t *testing.T) {
	srv := pulsetest.NewServer(t)
	// The server never says "started", so Start waits.
	srv.StartedDelay(time.Hour)
	c := newTestClient(t, srv, 2*time.Second)
	stream, err := c.NewPlayback(silence())
	if err != nil {
		t.Fatalf("NewPlayback: %v", err)
	}
	t.Cleanup(stream.Close)

	started := make(chan struct{})
	go func() {
		defer close(started)
		stream.Start()
	}()
	eventually(t, "the stream to run", stream.Running)
	srv.DropConnections()

	select {
	case <-started:
	case <-time.After(waitFor):
		t.Fatal("Start still waits after the connection was lost")
	}
	eventually(t, "the stream to see the loss", stream.Closed)
	if err := stream.Error(); !errors.Is(err, ErrConnectionClosed) {
		t.Errorf("Error() = %v, want ErrConnectionClosed", err)
	}
	err = c.RawRequest(&proto.Subscribe{Mask: proto.SubscriptionMaskSinkInput}, nil)
	appErr, ok := errors.FromError(err)
	if !ok || !appErr.HasRoute("PulseConnectionLost") {
		t.Errorf("a request after the loss failed with %v, want PulseConnectionLost", err)
	}
	// Closing a lost stream is fine.
	within(t, "Close", stream.Close)
}

func TestPlaybackCloseWhileStarting(t *testing.T) {
	srv := pulsetest.NewServer(t)
	srv.StartedDelay(time.Hour)
	c := newTestClient(t, srv, 2*time.Second)
	stream, err := c.NewPlayback(silence())
	if err != nil {
		t.Fatalf("NewPlayback: %v", err)
	}
	started := make(chan struct{})
	go func() {
		defer close(started)
		stream.Start()
	}()
	eventually(t, "the stream to run", stream.Running)
	within(t, "Close", stream.Close)
	select {
	case <-started:
	case <-time.After(waitFor):
		t.Fatal("Start still waits after Close")
	}
	// Closing twice, from several goroutines, is fine.
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stream.Close()
		}()
	}
	within(t, "the repeated Close", wg.Wait)
}

func TestClientRequestErrors(t *testing.T) {
	cases := []struct {
		name      string
		setup     func(srv *pulsetest.Server)
		wantRoute string
		wantIs    error
	}{
		{"the server rejects the command", func(srv *pulsetest.Server) { srv.Reject(proto.OpStat, proto.ErrNoSuchEntity) },
			"Client.Request.PulseRequestRejected", proto.ErrNoSuchEntity},
		{"the server does not answer", func(srv *pulsetest.Server) { srv.Ignore(proto.OpStat) },
			"Client.Request.PulseRequestTimeout", nil},
		{"the server does not know the command", func(srv *pulsetest.Server) {},
			"Client.Request.PulseRequestRejected", proto.ErrNotSupported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := pulsetest.NewServer(t)
			tc.setup(srv)
			c := newTestClient(t, srv, 100*time.Millisecond)
			err := c.RawRequest(&proto.Stat{}, &proto.StatReply{})
			appErr, ok := errors.FromError(err)
			if !ok {
				t.Fatalf("expected *errors.Error, got %T (%v)", err, err)
			}
			if !appErr.HasRoute(tc.wantRoute) {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
			if tc.wantIs != nil && !errors.Is(err, tc.wantIs) {
				t.Errorf("errors.Is(err, %v) = false", tc.wantIs)
			}
			// The connection is still usable.
			if err := c.RawRequest(&proto.Subscribe{Mask: proto.SubscriptionMaskSinkInput}, nil); err != nil {
				t.Errorf("a later request failed: %v", err)
			}
		})
	}
}

func TestNewClientErrors(t *testing.T) {
	t.Run("no server", func(t *testing.T) {
		t.Setenv("PULSE_COOKIE", filepath.Join(t.TempDir(), "no-cookie"))
		_, err := NewClient(ClientServerString("unix:" + filepath.Join(t.TempDir(), "missing")))
		appErr, ok := errors.FromError(err)
		if !ok {
			t.Fatalf("expected *errors.Error, got %T (%v)", err, err)
		}
		if !appErr.HasRoute("NewClient->Connect->connect.PulseDialFailed") {
			t.Errorf("unexpected failure path: %s", appErr.Route())
		}
	})
	t.Run("the server rejects the client name", func(t *testing.T) {
		t.Setenv("PULSE_COOKIE", filepath.Join(t.TempDir(), "no-cookie"))
		srv := pulsetest.NewServer(t)
		srv.Reject(proto.OpSetClientName, proto.ErrAccessDenied)
		_, err := NewClient(ClientServerString(srv.Addr()))
		appErr, ok := errors.FromError(err)
		if !ok {
			t.Fatalf("expected *errors.Error, got %T (%v)", err, err)
		}
		if !appErr.HasRoute("PulseRequestRejected") || !errors.Is(err, proto.ErrAccessDenied) {
			t.Errorf("unexpected failure: %s", appErr.Route())
		}
	})
	t.Run("the server rejects the subscription", func(t *testing.T) {
		t.Setenv("PULSE_COOKIE", filepath.Join(t.TempDir(), "no-cookie"))
		srv := pulsetest.NewServer(t)
		srv.Reject(proto.OpSubscribe, proto.ErrAccessDenied)
		_, err := NewClient(ClientServerString(srv.Addr()))
		if !errors.Is(err, proto.ErrAccessDenied) {
			t.Errorf("err = %v, want access denied", err)
		}
	})
}

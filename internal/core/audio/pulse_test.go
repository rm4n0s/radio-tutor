//go:build !windows

package audio

import (
	"context"
	"net"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
)

// fakeBackend is a pulseBackend without a server.
type fakeBackend struct {
	mu        sync.Mutex
	openErr   error
	resumeErr error         // when set, streams' Resume fails with it
	gate      chan struct{} // when set, streams' Start blocks until it is closed
	streams   []*fakeStream
	closed    bool
}

func (b *fakeBackend) open(read func([]float32) (int, error), f Format) (pulseStream, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.openErr != nil {
		return nil, b.openErr
	}
	s := &fakeStream{read: read, format: f, gate: b.gate, backend: b}
	b.streams = append(b.streams, s)
	return s, nil
}

func (b *fakeBackend) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
}

func (b *fakeBackend) stream(i int) *fakeStream {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.streams[i]
}

// fakeStream is a pulseStream that records calls and answers played with a
// scripted position.
type fakeStream struct {
	read    func([]float32) (int, error)
	format  Format
	gate    chan struct{}
	backend *fakeBackend

	mu       sync.Mutex
	calls    []string
	answer   func(now time.Time) (float64, bool) // nil: no answer
	askCount int
}

func (s *fakeStream) record(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, name)
}

func (s *fakeStream) Start() {
	s.record("start")
	if s.gate != nil {
		<-s.gate
	}
}
func (s *fakeStream) Pause() { s.record("pause") }

func (s *fakeStream) Resume() error {
	s.backend.mu.Lock()
	err := s.backend.resumeErr
	s.backend.mu.Unlock()
	if err != nil {
		s.record("resume failed")
		return err
	}
	s.record("resume")
	return nil
}

func (s *fakeStream) Close() { s.record("close") }

func (s *fakeStream) Calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.calls)
}

func (s *fakeStream) asked() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.askCount
}

func (s *fakeStream) setAnswer(f func(now time.Time) (float64, bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.answer = f
}

func (s *fakeStream) played(c Clock) (float64, time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.askCount++
	if s.answer == nil {
		return 0, time.Time{}, false
	}
	now := c.Now()
	pos, ok := s.answer(now)
	return pos, now, ok
}

// newFakePulse returns a PulseSink over a fakeBackend whose poll loop is
// effectively off (tests call observe themselves).
func newFakePulse(clock Clock) (*PulseSink, *fakeBackend) {
	b := &fakeBackend{}
	s := newPulseSink(b, clock)
	s.pollInterval = time.Hour
	return s, b
}

var mono1k = Format{SampleRate: 1000, Channels: 1}

func TestPulseSinkPosition(t *testing.T) {
	clock := NewFakeClock(epoch)
	s, b := newFakePulse(clock)
	if s.Name() != "pulse" {
		t.Fatalf("name = %q", s.Name())
	}
	src := &clipSource{samples: make([]float32, 1000), channels: 1}
	if err := s.Start(context.Background(), src, mono1k); err != nil {
		t.Fatal(err)
	}
	st := b.stream(0)

	// Nothing is said by the server yet: the clock extrapolates.
	clock.Advance(ms(200))
	if !near(s.Position(), 0.2) {
		t.Fatalf("extrapolated position = %v", s.Position())
	}

	// The server says 0.15 s were played 0 ms ago: behind the extrapolation,
	// so the position holds, then goes on once time has caught up.
	st.setAnswer(func(time.Time) (float64, bool) { return 0.15, true })
	s.observe(st)
	if !near(s.Position(), 0.2) {
		t.Fatalf("position went back to %v", s.Position())
	}
	clock.Advance(ms(30))
	if !near(s.Position(), 0.2) {
		t.Fatalf("position = %v, want it held at 0.2", s.Position())
	}
	clock.Advance(ms(50))
	if !near(s.Position(), 0.23) {
		t.Fatalf("position = %v, want 0.23", s.Position())
	}

	// A server ahead of the clock moves the position forward at once.
	st.setAnswer(func(time.Time) (float64, bool) { return 0.5, true })
	s.observe(st)
	if !near(s.Position(), 0.5) {
		t.Fatalf("position = %v, want 0.5", s.Position())
	}

	// No answer: the clock goes on.
	st.setAnswer(func(time.Time) (float64, bool) { return 0, false })
	s.observe(st)
	clock.Advance(ms(100))
	if !near(s.Position(), 0.6) {
		t.Fatalf("position = %v, want 0.6", s.Position())
	}

	// Pause holds, even when the server still answers; the stream is corked,
	// not closed.
	s.Pause()
	st.setAnswer(func(time.Time) (float64, bool) { return 0.9, true })
	s.observe(st)
	clock.Advance(ms(1000))
	if !near(s.Position(), 0.6) {
		t.Fatalf("position while paused = %v", s.Position())
	}
	if got := st.Calls(); !slices.Equal(got, []string{"start", "pause"}) {
		t.Fatalf("stream calls after Pause = %v", got)
	}

	// Resume uncorks the same stream; the source is not moved, and the clock
	// goes on from the held position.
	srcPos := src.pos
	st.setAnswer(func(time.Time) (float64, bool) { return 0, false })
	if err := s.Resume(context.Background()); err != nil {
		t.Fatal(err)
	}
	if src.pos != srcPos {
		t.Fatalf("the source was moved from %d to %d", srcPos, src.pos)
	}
	if len(b.streams) != 1 {
		t.Fatalf("%d streams opened, want the one", len(b.streams))
	}
	clock.Advance(ms(100))
	if !near(s.Position(), 0.7) {
		t.Fatalf("position after resume = %v", s.Position())
	}
	// The stream counts from its own start and not while corked: 0.72 s into
	// it is 0.72 s of the clip.
	st.setAnswer(func(time.Time) (float64, bool) { return 0.72, true })
	s.observe(st)
	if !near(s.Position(), 0.72) {
		t.Fatalf("position after an observation of the stream = %v", s.Position())
	}

	s.Stop()
	if s.Position() != 0 {
		t.Fatalf("position after Stop = %v", s.Position())
	}
	if got := st.Calls(); !slices.Equal(got, []string{"start", "pause", "resume", "close"}) {
		t.Fatalf("stream calls = %v", got)
	}
	// After Stop there is nothing to resume.
	if err := s.Resume(context.Background()); err != nil || len(b.streams) != 1 {
		t.Fatalf("Resume after Stop: %v, %d streams", err, len(b.streams))
	}
}

func TestPulseSinkPauseResumeCycles(t *testing.T) {
	// Many pauses and resumes keep to one stream, and the position holds at
	// every pause.
	tests := []struct {
		name   string
		cycles int
	}{
		{"one", 1},
		{"two", 2},
		{"many", 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := NewFakeClock(epoch)
			s, b := newFakePulse(clock)
			src := &clipSource{samples: make([]float32, 100000), channels: 1}
			if err := s.Start(context.Background(), src, mono1k); err != nil {
				t.Fatal(err)
			}
			want := []string{"start"}
			played := 0.0
			for range tt.cycles {
				clock.Advance(ms(100))
				played += 0.1
				s.Pause()
				clock.Advance(ms(500)) // not counted
				if !near(s.Position(), played) {
					t.Fatalf("position while paused = %v, want %v", s.Position(), played)
				}
				if err := s.Resume(context.Background()); err != nil {
					t.Fatal(err)
				}
				want = append(want, "pause", "resume")
			}
			clock.Advance(ms(100))
			if !near(s.Position(), played+0.1) {
				t.Fatalf("position = %v, want %v", s.Position(), played+0.1)
			}
			if len(b.streams) != 1 {
				t.Fatalf("%d streams opened, want 1", len(b.streams))
			}
			if got := b.stream(0).Calls(); !slices.Equal(got, want) {
				t.Fatalf("stream calls = %v, want %v", got, want)
			}
			// Pause twice and Resume twice do nothing the second time.
			s.Pause()
			s.Pause()
			if err := s.Resume(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := s.Resume(context.Background()); err != nil {
				t.Fatal(err)
			}
			if got := b.stream(0).Calls(); !slices.Equal(got, append(want, "pause", "resume")) {
				t.Fatalf("stream calls after the doubled calls = %v", got)
			}
		})
	}
}

func TestPulseSinkStartAsksTheServerAtOnce(t *testing.T) {
	// The first measurement is taken before Start returns, so the position
	// does not begin with a jump.
	clock := NewFakeClock(epoch)
	s, b := newFakePulse(clock)
	b.gate = nil
	if err := s.Start(context.Background(), &clipSource{channels: 1}, mono1k); err != nil {
		t.Fatal(err)
	}
	if got := b.stream(0).asked(); got != 1 {
		t.Fatalf("the server was asked %d times during Start, want 1", got)
	}
}

func TestPulseSinkReader(t *testing.T) {
	s, b := newFakePulse(NewFakeClock(epoch))
	src := &clipSource{samples: []float32{1, 2, 3, 4, 5}, channels: 1}
	if err := s.Start(context.Background(), src, mono1k); err != nil {
		t.Fatal(err)
	}
	read := b.stream(0).read
	tests := []struct {
		size int
		want []float32
	}{
		{3, []float32{1, 2, 3}},
		{4, []float32{4, 5, 0, 0}}, // the end of the source, then silence
		{2, []float32{0, 0}},
	}
	for i, tt := range tests {
		buf := []float32{9, 9, 9, 9, 9, 9}[:tt.size]
		n, err := read(buf)
		if err != nil || n != tt.size || !slices.Equal(buf, tt.want) {
			t.Fatalf("read %d = %d %v %v, want %d %v", i, n, err, buf, tt.size, tt.want)
		}
	}
	if b.stream(0).format != mono1k {
		t.Fatalf("stream format = %+v", b.stream(0).format)
	}
}

func TestPulseSinkStartReplacesTheStream(t *testing.T) {
	s, b := newFakePulse(NewFakeClock(epoch))
	for range 2 {
		if err := s.Start(context.Background(), &clipSource{channels: 1}, mono1k); err != nil {
			t.Fatal(err)
		}
	}
	if got := b.stream(0).Calls(); !slices.Equal(got, []string{"start", "close"}) {
		t.Fatalf("first stream calls = %v", got)
	}
	if got := b.stream(1).Calls(); !slices.Equal(got, []string{"start"}) {
		t.Fatalf("second stream calls = %v", got)
	}
	s.Close()
	s.Close() // idempotent
	if got := b.stream(1).Calls(); !slices.Equal(got, []string{"start", "close"}) {
		t.Fatalf("second stream calls after Close = %v", got)
	}
	if !b.closed {
		t.Fatal("the backend was not closed")
	}
	// Pause, Resume and Stop with nothing open do nothing.
	s.Pause()
	if err := s.Resume(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.Stop()
}

func TestPulseSinkPolls(t *testing.T) {
	s, b := newFakePulse(SystemClock())
	s.pollInterval = time.Millisecond
	if err := s.Start(context.Background(), &clipSource{channels: 1}, mono1k); err != nil {
		t.Fatal(err)
	}
	st := b.stream(0)
	st.setAnswer(func(time.Time) (float64, bool) { return 0, true })
	deadline := time.Now().Add(5 * time.Second)
	for st.asked() < 4 {
		if time.Now().After(deadline) {
			t.Fatal("the sink does not poll the server")
		}
		time.Sleep(time.Millisecond)
	}
	s.Stop()
	time.Sleep(20 * time.Millisecond) // let a tick that was already due finish
	n := st.asked()
	time.Sleep(30 * time.Millisecond)
	if st.asked() != n {
		t.Fatal("the sink polls after Stop")
	}
}

func TestPulseSinkErrors(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	downErr := errors.New("TestServerDown", "down")

	tests := []struct {
		name  string
		setup func(s *PulseSink, b *fakeBackend)
		ctx   func() (context.Context, context.CancelFunc)
		f     Format
		route string
		is    error // when set, the error is this one, passed up from the library untouched
	}{
		{"zero sample rate", nil, nil, Format{Channels: 1}, "PulseSink.Start.PulseFormatUnsupported", nil},
		{"three channels", nil, nil, Format{SampleRate: 1000, Channels: 3}, "PulseSink.Start.PulseFormatUnsupported", nil},
		{"closed", func(s *PulseSink, _ *fakeBackend) { s.Close() }, nil, mono1k, "PulseSink.Start.PulseClosed", nil},
		{"cancelled before", nil, func() (context.Context, context.CancelFunc) { return cancelled, func() {} }, mono1k, "PulseSink.Start.PulseStartCanceled", nil},
		{"the server refuses the stream", func(_ *PulseSink, b *fakeBackend) { b.openErr = downErr }, nil, mono1k, "", downErr},
		{"the stream never starts", func(s *PulseSink, b *fakeBackend) {
			b.gate = make(chan struct{})
			s.startTimeout = 20 * time.Millisecond
		}, nil, mono1k, "PulseSink.Start->PulseSink.openStream.PulseStartTimeout", nil},
		{"cancelled while starting", func(_ *PulseSink, b *fakeBackend) { b.gate = make(chan struct{}) },
			func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				time.AfterFunc(20*time.Millisecond, cancel)
				return ctx, cancel
			}, mono1k, "PulseSink.Start->PulseSink.openStream.PulseStartCanceled", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, b := newFakePulse(NewFakeClock(epoch))
			if tt.setup != nil {
				tt.setup(s, b)
			}
			if gate := b.gate; gate != nil {
				t.Cleanup(func() { close(gate) })
			}
			ctx, cancel := context.Background(), context.CancelFunc(func() {})
			if tt.ctx != nil {
				ctx, cancel = tt.ctx()
			}
			defer cancel()
			err := s.Start(ctx, &clipSource{channels: 1}, tt.f)
			if tt.is != nil {
				// Tagged by the library and passed up as it is: not wrapped again.
				if err != tt.is {
					t.Fatalf("err = %v, want the library's error %v", err, tt.is)
				}
			} else {
				wantRoute(t, err, tt.route)
			}
			if s.Position() != 0 {
				t.Fatalf("position after a failed Start = %v", s.Position())
			}
			// The streams that were opened are closed again.
			for i := range b.streams {
				if calls := b.stream(i).Calls(); calls[len(calls)-1] != "close" {
					t.Fatalf("stream %d left open: %v", i, calls)
				}
			}
		})
	}

	t.Run("the cause of a refused stream is kept", func(t *testing.T) {
		s, b := newFakePulse(NewFakeClock(epoch))
		b.openErr = downErr
		err := s.Start(context.Background(), &clipSource{channels: 1}, mono1k)
		if !errors.Is(err, downErr) {
			t.Fatalf("errors.Is(err, cause) = false for %v", err)
		}
	})
}

func TestPulseSinkResumeErrors(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	downErr := errors.New("TestServerDown", "down")
	tests := []struct {
		name  string
		setup func(s *PulseSink, b *fakeBackend)
		ctx   context.Context
		route string
		is    error // when set, the error is this one, passed up from the library untouched
	}{
		{"the server does not answer the uncork", func(_ *PulseSink, b *fakeBackend) { b.resumeErr = downErr },
			context.Background(), "", downErr},
		{"cancelled", nil, cancelled, "PulseSink.Resume.PulseStartCanceled", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := NewFakeClock(epoch)
			s, b := newFakePulse(clock)
			if err := s.Start(context.Background(), &clipSource{samples: make([]float32, 1000), channels: 1}, mono1k); err != nil {
				t.Fatal(err)
			}
			clock.Advance(ms(300))
			s.Pause()
			if tt.setup != nil {
				tt.setup(s, b)
			}
			err := s.Resume(tt.ctx)
			if tt.is != nil {
				if err != tt.is {
					t.Fatalf("err = %v, want the library's error %v", err, tt.is)
				}
			} else {
				wantRoute(t, err, tt.route)
			}
			// Still paused, and a later Resume can succeed.
			clock.Advance(ms(1000))
			if !near(s.Position(), 0.3) {
				t.Fatalf("position after a failed Resume = %v", s.Position())
			}
			b.resumeErr = nil
			if err := s.Resume(context.Background()); err != nil {
				t.Fatalf("Resume after the failure: %v", err)
			}
			clock.Advance(ms(100))
			if !near(s.Position(), 0.4) {
				t.Fatalf("position after the second Resume = %v", s.Position())
			}
			if len(b.streams) != 1 {
				t.Fatalf("%d streams opened, want 1", len(b.streams))
			}
		})
	}
}

func TestPulseSinkInAPlayer(t *testing.T) {
	// The Player over a PulseSink whose server answers a little behind the
	// clock (the device latency): the clip still ends, the position never
	// goes back, and the end waits for the sink's position.
	clock := NewFakeClock(epoch)
	s, b := newFakePulse(clock)
	p := NewPlayer(s, clock)
	if err := p.Load(rampClip(1000)); err != nil {
		t.Fatal(err)
	}
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	st := b.stream(0)
	start := clock.Now()
	st.setAnswer(func(now time.Time) (float64, bool) { return max(now.Sub(start).Seconds()-0.1, 0), true })
	last := 0.0
	for range 100 {
		clock.Advance(ms(5))
		s.observe(st)
		pos := p.Position()
		if pos < last {
			t.Fatalf("position went back from %v to %v", last, pos)
		}
		last = pos
	}
	if p.State() != StatePlaying {
		t.Fatalf("state = %v at 500 ms", p.State())
	}
	clock.Advance(ms(500))
	if p.State() != StatePlaying || !near(p.Position(), 0.9) {
		t.Fatalf("at the clip's length the device has played %v s of it; state = %v", p.Position(), p.State())
	}
	clock.Advance(ms(100))
	if p.State() != StateEnded {
		t.Fatalf("state = %v after the device latency", p.State())
	}
	if got := st.Calls(); !slices.Equal(got, []string{"start", "close"}) {
		t.Fatalf("stream calls = %v (the stream must be released at the end)", got)
	}
}

func TestDefaultServer(t *testing.T) {
	tests := []struct {
		name   string
		env    map[string]string
		want   string
		wantOK bool
	}{
		{"PULSE_SERVER wins", map[string]string{"PULSE_SERVER": "tcp:host:4713", "XDG_RUNTIME_DIR": "/run/user/1000"}, "tcp:host:4713", true},
		{"the runtime dir socket", map[string]string{"XDG_RUNTIME_DIR": "/run/user/1000"}, "unix:/run/user/1000/pulse/native", true},
		{"empty PULSE_SERVER is unset", map[string]string{"PULSE_SERVER": "", "XDG_RUNTIME_DIR": "/run/user/7"}, "unix:/run/user/7/pulse/native", true},
		{"nothing", map[string]string{}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := defaultServer(func(k string) string { return tt.env[k] })
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("defaultServer = %q, %v; want %q, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestNewPulseSinkErrors(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	dir := t.TempDir()

	// A socket that accepts and never answers.
	silent := filepath.Join(dir, "silent")
	ln, err := net.Listen("unix", silent)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = c.Close() })
		}
	}()

	tests := []struct {
		name  string
		env   map[string]string
		ctx   context.Context
		opts  PulseOptions
		wait  time.Duration
		route string
	}{
		{"no server known", map[string]string{"PULSE_SERVER": "", "XDG_RUNTIME_DIR": ""}, context.Background(), PulseOptions{}, 0,
			"NewPulseSink->dialPulse.PulseNoServer"},
		{"no socket", map[string]string{"PULSE_SERVER": "", "XDG_RUNTIME_DIR": filepath.Join(dir, "missing")}, context.Background(), PulseOptions{}, 0,
			"NewClient->Connect->connect.PulseDialFailed"},
		{"explicit server without a socket", nil, context.Background(), PulseOptions{Server: "unix:" + filepath.Join(dir, "none")}, 0,
			"NewClient->Connect->connect.PulseDialFailed"},
		{"cancelled", nil, cancelled, PulseOptions{Server: "unix:" + silent}, 0,
			"NewPulseSink->dialPulse.PulseConnectCanceled"},
		{"cancelled while connecting", nil, nil, PulseOptions{Server: "unix:" + silent}, 0,
			"NewPulseSink->dialPulse.PulseConnectCanceled"},
		{"a server that does not answer", nil, context.Background(), PulseOptions{Server: "unix:" + silent}, 30 * time.Millisecond,
			"dialPulse.PulseConnectTimeout"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			ctx := tt.ctx
			if ctx == nil {
				var cancelLater context.CancelFunc
				ctx, cancelLater = context.WithTimeout(context.Background(), 30*time.Millisecond)
				defer cancelLater()
			}
			var err error
			if tt.wait > 0 {
				_, err = dialPulse(ctx, tt.wait, tt.opts)
			} else {
				_, err = NewPulseSink(ctx, tt.opts)
			}
			wantRoute(t, err, tt.route)
		})
	}
}

func TestOpenDefaultSinkFallsBackToSilent(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name  string
		env   map[string]string
		route string
	}{
		{"no server known", map[string]string{"PULSE_SERVER": "", "XDG_RUNTIME_DIR": ""}, "OpenDefaultSink->NewPulseSink->dialPulse.PulseNoServer"},
		{"no socket at PULSE_SERVER", map[string]string{"PULSE_SERVER": "unix:" + filepath.Join(dir, "none")}, "NewClient->Connect->connect.PulseDialFailed"},
		{"no socket in the runtime dir", map[string]string{"PULSE_SERVER": "", "XDG_RUNTIME_DIR": dir}, "NewClient->Connect->connect.PulseDialFailed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			sink, reason := OpenDefaultSink(context.Background())
			if sink == nil || sink.Name() != "silent" {
				t.Fatalf("sink = %v, want the silent fallback", sink)
			}
			wantRoute(t, reason, tt.route)
			// The fallback plays: a lesson can be completed without sound.
			p := NewPlayer(sink, nil)
			if err := p.Load(rampClip(30)); err != nil {
				t.Fatal(err)
			}
			if err := p.Play(context.Background()); err != nil {
				t.Fatal(err)
			}
			select {
			case <-p.Ended():
			case <-time.After(5 * time.Second):
				t.Fatal("the silent fallback did not end the clip")
			}
			sink.Close()
		})
	}
}

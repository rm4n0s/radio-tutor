package audio

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
)

// newTestWaveOut returns a sink over a fake winmm and a fake clock. The
// buffers are 50 ms: 50 frames at the 1000 Hz of the ramp clips.
func newTestWaveOut(t *testing.T) (*WaveOutSink, *fakeWave, *FakeClock, *capturedLog) {
	t.Helper()
	b := newFakeWave()
	clock := NewFakeClock(epoch)
	l := &capturedLog{}
	s := newWaveOutSink(b, WaveOutOptions{Clock: clock, Logger: slog.New(l)})
	s.joinTimeout = 200 * time.Millisecond
	t.Cleanup(s.Close)
	return s, b, clock, l
}

// capturedLog is a slog.Handler that keeps the error attributes.
type capturedLog struct {
	mu   sync.Mutex
	errs []error
}

func (c *capturedLog) Enabled(context.Context, slog.Level) bool { return true }
func (c *capturedLog) WithAttrs([]slog.Attr) slog.Handler       { return c }
func (c *capturedLog) WithGroup(string) slog.Handler            { return c }
func (c *capturedLog) Handle(_ context.Context, r slog.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	r.Attrs(func(a slog.Attr) bool {
		if err, ok := a.Value.Any().(error); ok {
			c.errs = append(c.errs, err)
		}
		return true
	})
	return nil
}

func (c *capturedLog) errors() []error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]error(nil), c.errs...)
}

// eventually polls cond for two seconds.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func clipSourceOf(c *Clip) *clipSource {
	return &clipSource{samples: c.Samples, channels: c.Channels}
}

func clipFormat(c *Clip) Format { return Format{SampleRate: c.SampleRate, Channels: c.Channels} }

// pcmOf is the 16-bit PCM of the clip's samples.
func pcmOf(c *Clip) []byte {
	b := make([]byte, 2*len(c.Samples))
	encodePCM16(b, c.Samples)
	return b
}

func TestWaveOutName(t *testing.T) {
	s, _, _, _ := newTestWaveOut(t)
	if s.Name() != "waveout" {
		t.Fatalf("name = %q", s.Name())
	}
}

func TestEncodePCM16(t *testing.T) {
	nan := float32(0)
	nan /= nan
	tests := []struct {
		name string
		in   float32
		want int16
	}{
		{"zero", 0, 0},
		{"full", 1, 32767},
		{"minus full", -1, -32767},
		{"over", 3, 32767},
		{"under", -3, -32767},
		{"half", 0.5, 16384},
		{"small", 1.0 / 32767, 1},
		{"nan", nan, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var b [2]byte
			encodePCM16(b[:], []float32{tc.in})
			got := int16(uint16(b[0]) | uint16(b[1])<<8)
			if got != tc.want {
				t.Fatalf("%v -> %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestWaveOutPlaysTheClip(t *testing.T) {
	tests := []struct {
		name string
		clip *Clip
	}{
		{"mono", rampClip(430)},
		{"stereo", stereoRampClip(430)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, b, _, _ := newTestWaveOut(t)
			if err := s.Start(context.Background(), clipSourceOf(tc.clip), clipFormat(tc.clip)); err != nil {
				t.Fatal(err)
			}
			dev := b.device()
			if b.format != clipFormat(tc.clip) {
				t.Fatalf("opened %+v", b.format)
			}
			// The ring is full and the device runs after the ring was filled.
			if got := dev.queued(); got != 4 {
				t.Fatalf("queued after Start = %d, want 4", got)
			}
			calls := dev.callList()
			if calls[len(calls)-1] != "restart" || dev.isPaused() {
				t.Fatalf("the device was not restarted last: %v", calls)
			}
			bufBytes := 50 * 2 * tc.clip.Channels
			want := pcmOf(tc.clip)
			total := len(want) + 6*bufBytes // the clip, then silence
			for played := 0; played < total; {
				played += dev.play(bufBytes)
				eventually(t, "the ring to be filled again", func() bool { return dev.queued() == 4 })
			}
			got := dev.playedBytes()
			if !bytes.Equal(got[:len(want)], want) {
				t.Fatalf("the played samples differ from the clip")
			}
			if len(got[len(want):]) == 0 || !bytes.Equal(got[len(want):], make([]byte, len(got)-len(want))) {
				t.Fatalf("what follows the end is not silence")
			}
		})
	}
}

func TestWaveOutPauseResume(t *testing.T) {
	clip := rampClip(1000)
	s, b, clock, _ := newTestWaveOut(t)
	if err := s.Start(context.Background(), clipSourceOf(clip), clipFormat(clip)); err != nil {
		t.Fatal(err)
	}
	dev := b.device()
	step := func(frames int) {
		t.Helper()
		before := len(dev.playedBytes())
		dev.play(frames * 2)
		eventually(t, "the ring to be filled again", func() bool { return dev.queued() == 4 })
		if got := len(dev.playedBytes()) - before; got != frames*2 {
			t.Fatalf("played %d bytes, want %d", got, frames*2)
		}
	}
	step(120)
	eventually(t, "the position to follow the device", func() bool { return near(s.Position(), 0.12) })

	s.Pause()
	if !dev.isPaused() {
		t.Fatal("the device is not paused")
	}
	clock.Advance(ms(5000))
	if got := dev.play(1000); got != 0 {
		t.Fatalf("a paused device played %d bytes", got)
	}
	if !near(s.Position(), 0.12) {
		t.Fatalf("position while paused = %v", s.Position())
	}

	if err := s.Resume(context.Background()); err != nil {
		t.Fatal(err)
	}
	if dev.isPaused() {
		t.Fatal("the device is still paused")
	}
	step(80)
	eventually(t, "the position to go on", func() bool { return near(s.Position(), 0.2) })
	if b.opened != 1 {
		t.Fatalf("the device was opened %d times; Pause and Resume must keep it", b.opened)
	}

	// Nothing was lost or repeated around the pause.
	want := pcmOf(clip)[:400]
	if got := dev.playedBytes(); !bytes.Equal(got, want) {
		t.Fatalf("the played samples differ from the clip around the pause")
	}

	// Pause and Resume are no-ops when they do not apply.
	s.Pause()
	s.Pause()
	if err := s.Resume(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Resume(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestWaveOutPositionNeverGoesBack(t *testing.T) {
	clip := rampClip(1000)
	s, b, clock, _ := newTestWaveOut(t)
	if err := s.Start(context.Background(), clipSourceOf(clip), clipFormat(clip)); err != nil {
		t.Fatal(err)
	}
	dev := b.device()
	clock.Advance(ms(300)) // the clock runs ahead of a device that plays nothing
	high := s.Position()
	if !near(high, 0.3) {
		t.Fatalf("position = %v, want 0.3 by the clock", high)
	}
	dev.play(20) // the device reports 10 frames: behind the extrapolation
	eventually(t, "a measurement", func() bool { return dev.queued() == 4 })
	time.Sleep(20 * time.Millisecond)
	if got := s.Position(); got < high {
		t.Fatalf("position went back from %v to %v", high, got)
	}
}

func TestWaveOutStopAndRestart(t *testing.T) {
	clip := rampClip(1000)
	s, b, clock, _ := newTestWaveOut(t)
	if err := s.Start(context.Background(), clipSourceOf(clip), clipFormat(clip)); err != nil {
		t.Fatal(err)
	}
	first := b.device()
	clock.Advance(ms(250))
	s.Pause()
	s.Stop()
	if s.Position() != 0 {
		t.Fatalf("position after Stop = %v", s.Position())
	}
	calls := first.callList()
	if n := len(calls); calls[n-2] != "reset" || calls[n-1] != "close" || !first.isClosed() {
		t.Fatalf("Stop did not reset and close the device: %v", calls)
	}
	// Resume after Stop does nothing; a new Start opens a new device at 0.
	if err := s.Resume(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(context.Background(), clipSourceOf(clip), clipFormat(clip)); err != nil {
		t.Fatal(err)
	}
	if b.device() == first || b.opened != 2 {
		t.Fatalf("Start did not open a new device (%d opens)", b.opened)
	}
	if s.Position() != 0 {
		t.Fatalf("position after the second Start = %v", s.Position())
	}
	// Start while playing stops the first play.
	second := b.device()
	if err := s.Start(context.Background(), clipSourceOf(clip), clipFormat(clip)); err != nil {
		t.Fatal(err)
	}
	if !second.isClosed() {
		t.Fatal("Start left the earlier device open")
	}
	s.Close()
	if !b.device().isClosed() {
		t.Fatal("Close left the device open")
	}
	s.Close() // twice is fine
}

func TestWaveOutPlayerEnds(t *testing.T) {
	clip := rampClip(500)
	s, b, clock, _ := newTestWaveOut(t)
	p := NewPlayer(s, clock)
	defer p.Close()
	if err := p.Load(clip); err != nil {
		t.Fatal(err)
	}
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(ms(200))
	p.Pause()
	clock.Advance(ms(5000))
	if !near(p.Position(), 0.2) || p.State() != StatePaused {
		t.Fatalf("paused at %v, state %v", p.Position(), p.State())
	}
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(ms(300))
	select {
	case <-p.Ended():
	case <-time.After(2 * time.Second):
		t.Fatal("the clip did not end")
	}
	if p.State() != StateEnded || !b.device().isClosed() {
		t.Fatalf("state %v, device closed %v", p.State(), b.device().isClosed())
	}
}

func TestWaveOutStartErrors(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	clip := rampClip(200)
	good := clipFormat(clip)
	tests := []struct {
		name  string
		setup func(s *WaveOutSink, b *fakeWave)
		ctx   context.Context
		f     Format
		route string
	}{
		{"closed", func(s *WaveOutSink, _ *fakeWave) { s.Close() }, context.Background(), good, "WaveOutSink.Start.WaveOutClosed"},
		{"zero rate", nil, context.Background(), Format{Channels: 1}, "WaveOutSink.Start.WaveOutFormatUnsupported"},
		{"three channels", nil, context.Background(), Format{SampleRate: 1000, Channels: 3}, "WaveOutSink.Start.WaveOutFormatUnsupported"},
		{"cancelled", nil, cancelled, good, "WaveOutSink.Start.WaveOutStartCanceled"},
		{"no device", func(_ *WaveOutSink, b *fakeWave) { b.devices = 0 }, context.Background(), good, "WaveOutSink.Start.WaveOutNoDevice"},
		{"open fails", func(_ *WaveOutSink, b *fakeWave) { b.openErr = errWave("waveOutOpen") }, context.Background(), good, "WaveOutSink.Start.WaveOutOpenFailed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, b, _, _ := newTestWaveOut(t)
			if tc.setup != nil {
				tc.setup(s, b)
			}
			err := s.Start(tc.ctx, clipSourceOf(clip), tc.f)
			assertRoute(t, err, tc.route)
			if s.Position() != 0 {
				t.Fatalf("position after a failed Start = %v", s.Position())
			}
		})
	}
}

func assertRoute(t *testing.T, err error, route string) {
	t.Helper()
	if err == nil {
		t.Fatalf("no error, want %s", route)
	}
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T", err)
	}
	if !appErr.HasRoute(route) {
		t.Fatalf("unexpected failure path: %s, want %s", appErr.Route(), route)
	}
}

func TestWaveOutDeviceFailures(t *testing.T) {
	clip := rampClip(1000)
	tests := []struct {
		name  string
		run   func(t *testing.T, s *WaveOutSink, b *fakeWave, l *capturedLog) error
		route string
	}{
		{"write in Start", func(t *testing.T, s *WaveOutSink, b *fakeWave, _ *capturedLog) error {
			s.backend = &failingOpen{fakeWave: b, mutate: func(d *fakeWaveDev) { d.errWrite = errWave("waveOutWrite") }}
			return s.Start(context.Background(), clipSourceOf(clip), clipFormat(clip))
		}, "WaveOutSink.Start.WaveOutWriteFailed"},
		{"restart in Start", func(t *testing.T, s *WaveOutSink, b *fakeWave, _ *capturedLog) error {
			s.backend = &failingOpen{fakeWave: b, mutate: func(d *fakeWaveDev) { d.errRestart = errWave("waveOutRestart") }}
			return s.Start(context.Background(), clipSourceOf(clip), clipFormat(clip))
		}, "WaveOutSink.Start.WaveOutRestartFailed"},
		{"restart in Resume", func(t *testing.T, s *WaveOutSink, b *fakeWave, _ *capturedLog) error {
			if err := s.Start(context.Background(), clipSourceOf(clip), clipFormat(clip)); err != nil {
				t.Fatal(err)
			}
			s.Pause()
			dev := b.device()
			dev.mu.Lock()
			dev.errRestart = errWave("waveOutRestart")
			dev.mu.Unlock()
			err := s.Resume(context.Background())
			if err == nil {
				t.Fatal("Resume succeeded")
			}
			// The sink stays paused and a later Resume can work.
			dev.mu.Lock()
			dev.errRestart = nil
			dev.mu.Unlock()
			if rerr := s.Resume(context.Background()); rerr != nil {
				t.Fatalf("second Resume: %v", rerr)
			}
			return err
		}, "WaveOutSink.Resume.WaveOutRestartFailed"},
		{"cancelled Resume", func(t *testing.T, s *WaveOutSink, b *fakeWave, _ *capturedLog) error {
			if err := s.Start(context.Background(), clipSourceOf(clip), clipFormat(clip)); err != nil {
				t.Fatal(err)
			}
			s.Pause()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return s.Resume(ctx)
		}, "WaveOutSink.Resume.WaveOutStartCanceled"},
		{"pause fails, Resume reports it", func(t *testing.T, s *WaveOutSink, b *fakeWave, l *capturedLog) error {
			if err := s.Start(context.Background(), clipSourceOf(clip), clipFormat(clip)); err != nil {
				t.Fatal(err)
			}
			dev := b.device()
			dev.mu.Lock()
			dev.errPause = errWave("waveOutPause")
			dev.mu.Unlock()
			s.Pause()
			errs := l.errors()
			if len(errs) != 1 {
				t.Fatalf("logged %d errors", len(errs))
			}
			assertRoute(t, errs[0], "WaveOutSink.Pause.WaveOutPauseFailed")
			return s.Resume(context.Background())
		}, "WaveOutSink.Pause.WaveOutPauseFailed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, b, _, l := newTestWaveOut(t)
			err := tc.run(t, s, b, l)
			assertRoute(t, err, tc.route)
		})
	}
}

// failingOpen is a backend whose next device is changed after it is made.
type failingOpen struct {
	*fakeWave
	mutate func(*fakeWaveDev)
}

func (f *failingOpen) open(fm Format, buffers, bufBytes int) (waveDevice, error) {
	d, err := f.fakeWave.open(fm, buffers, bufBytes)
	if err != nil {
		return nil, err
	}
	f.mutate(d.(*fakeWaveDev))
	return d, nil
}

func TestWaveOutStopFailuresAreLogged(t *testing.T) {
	clip := rampClip(1000)
	tests := []struct {
		name  string
		fail  func(d *fakeWaveDev)
		route string
	}{
		{"reset", func(d *fakeWaveDev) { d.errReset = errWave("waveOutReset") }, "WaveOutSink.release->WaveOutSink.logReset.WaveOutResetFailed"},
		{"close", func(d *fakeWaveDev) { d.errClose = errWave("waveOutClose") }, "WaveOutSink.release.WaveOutCloseFailed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, b, _, l := newTestWaveOut(t)
			if err := s.Start(context.Background(), clipSourceOf(clip), clipFormat(clip)); err != nil {
				t.Fatal(err)
			}
			dev := b.device()
			dev.mu.Lock()
			tc.fail(dev)
			dev.mu.Unlock()
			s.Stop()
			errs := l.errors()
			if len(errs) == 0 {
				t.Fatal("nothing was logged")
			}
			assertRoute(t, errs[0], tc.route)
		})
	}
}

func TestWaveOutWriteFailsInTheBackground(t *testing.T) {
	clip := rampClip(1000)
	s, b, clock, l := newTestWaveOut(t)
	if err := s.Start(context.Background(), clipSourceOf(clip), clipFormat(clip)); err != nil {
		t.Fatal(err)
	}
	dev := b.device()
	dev.mu.Lock()
	dev.errWrite = errWave("waveOutWrite")
	dev.mu.Unlock()
	dev.play(100)
	eventually(t, "the failure to be logged", func() bool { return len(l.errors()) > 0 })
	assertRoute(t, l.errors()[0], "WaveOutSink.feed.WaveOutWriteFailed")
	// The position goes on by the clock, so that the lesson ends.
	clock.Advance(ms(400))
	if !near(s.Position(), 0.4) {
		t.Fatalf("position = %v, want 0.4", s.Position())
	}
	s.Stop() // joins the goroutine that has ended
	if !dev.isClosed() {
		t.Fatal("the device was not closed")
	}
}

// blockedSource blocks in Read until released.
type blockedSource struct {
	inner   Source
	block   chan struct{}
	entered chan struct{}
	once    sync.Once
	armed   bool
	mu      sync.Mutex
}

func (b *blockedSource) arm() {
	b.mu.Lock()
	b.armed = true
	b.mu.Unlock()
}

func (b *blockedSource) Read(dst []float32) int {
	b.mu.Lock()
	armed := b.armed
	b.mu.Unlock()
	if armed {
		b.once.Do(func() { close(b.entered) })
		<-b.block
	}
	return b.inner.Read(dst)
}

func (b *blockedSource) SeekFrame(f int64) { b.inner.SeekFrame(f) }

func TestWaveOutStopDoesNotWaitForeverForTheSource(t *testing.T) {
	clip := rampClip(1000)
	s, b, _, _ := newTestWaveOut(t)
	src := &blockedSource{inner: clipSourceOf(clip), block: make(chan struct{}), entered: make(chan struct{})}
	if err := s.Start(context.Background(), src, clipFormat(clip)); err != nil {
		t.Fatal(err)
	}
	dev := b.device()
	src.arm()
	dev.play(100) // the goroutine refills and sticks in Read
	select {
	case <-src.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("the goroutine did not read")
	}
	began := time.Now()
	s.Stop()
	if time.Since(began) > time.Second {
		t.Fatalf("Stop took %v", time.Since(began))
	}
	if dev.isClosed() {
		t.Fatal("the device was closed under a goroutine that may still write to it")
	}
	if !contains(dev.callList(), "reset") {
		t.Fatal("the device was not silenced")
	}
	close(src.block)
	eventually(t, "the device to be closed when the goroutine returns", dev.isClosed)
}

func contains(l []string, s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

func TestWaveOutMeasuresWithoutDevicePosition(t *testing.T) {
	clip := rampClip(1000)
	s, b, clock, _ := newTestWaveOut(t)
	if err := s.Start(context.Background(), clipSourceOf(clip), clipFormat(clip)); err != nil {
		t.Fatal(err)
	}
	dev := b.device()
	dev.mu.Lock()
	dev.posOK = false
	dev.mu.Unlock()
	dev.play(100)
	eventually(t, "the ring to be filled again", func() bool { return dev.queued() == 4 })
	clock.Advance(ms(250))
	if !near(s.Position(), 0.25) {
		t.Fatalf("position = %v, want the clock's 0.25", s.Position())
	}
}

func TestWaveBackendErrorText(t *testing.T) {
	err := &waveError{Func: "waveOutOpen", Code: 6}
	if got := err.Error(); got != "waveOutOpen failed: MMSYSERR_NODRIVER (6)" {
		t.Fatalf("got %q", got)
	}
	for code := uint32(0); code < 40; code++ {
		if mmResultName(code) == "" {
			t.Fatalf("no name for %d", code)
		}
	}
}

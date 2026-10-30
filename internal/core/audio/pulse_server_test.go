package audio

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/pulse"
	"github.com/rm4n0s/radio-tutor/internal/core/pulse/proto"
	"github.com/rm4n0s/radio-tutor/internal/core/pulse/pulsetest"
)

// bytesPerSecond is what the fake server's "device" plays: 8000 mono float32
// frames a second.
const bytesPerSecond = 8000 * 4

var rate8k = Format{SampleRate: 8000, Channels: 1}

// openFakeServerSink connects a PulseSink to the fake PulseAudio server.
func openFakeServerSink(t *testing.T, srv *pulsetest.Server) *PulseSink {
	t.Helper()
	t.Setenv("PULSE_COOKIE", filepath.Join(t.TempDir(), "no-cookie"))
	sink, err := dialPulse(context.Background(), 5*time.Second, PulseOptions{Server: srv.Addr()})
	if err != nil {
		t.Fatalf("dialPulse: %v", err)
	}
	sink.pollInterval = 5 * time.Millisecond
	t.Cleanup(sink.Close)
	return sink
}

// countingSource is an endless Source of silence that counts the samples read
// and notices a SeekFrame (a corked stream continues, it is never moved).
type countingSource struct {
	read   atomic.Int64
	seeked atomic.Bool
}

func (s *countingSource) Read(dst []float32) int {
	clear(dst)
	s.read.Add(int64(len(dst)))
	return len(dst)
}

func (s *countingSource) SeekFrame(int64) { s.seeked.Store(true) }

func countCommands(commands []uint32, op uint32) int {
	n := 0
	for _, c := range commands {
		if c == op {
			n++
		}
	}
	return n
}

// waitUntil polls cond for up to 5 seconds.
func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// TestPulseSinkPauseResumeAgainstTheServer drives the real library against a
// fake server: Pause corks and Resume uncorks the one stream, the server
// answers every uncork with a "started" event (which used to wedge the
// library), the position holds while paused and never goes back.
func TestPulseSinkPauseResumeAgainstTheServer(t *testing.T) {
	srv := pulsetest.NewServer(t)
	srv.StartedDelay(5 * time.Millisecond)
	srv.Flow(bytesPerSecond)
	sink := openFakeServerSink(t, srv)

	src := &countingSource{}
	ctx := context.Background()
	if err := sink.Start(ctx, src, rate8k); err != nil {
		t.Fatal(err)
	}
	const stream = 0
	waitUntil(t, "the stream to be uncorked", func() bool { return !srv.Corked(stream) })

	last := 0.0
	check := func() float64 {
		t.Helper()
		pos := sink.Position()
		if pos < last {
			t.Fatalf("position went back from %v to %v", last, pos)
		}
		last = pos
		return pos
	}
	waitUntil(t, "the position to move", func() bool { return check() > 0.05 })

	for cycle := range 4 {
		sink.Pause()
		if !srv.Corked(stream) {
			t.Fatalf("cycle %d: the server stream is not corked after Pause", cycle)
		}
		held := check()
		time.Sleep(30 * time.Millisecond)
		if got := check(); got != held {
			t.Fatalf("cycle %d: position moved from %v to %v while paused", cycle, held, got)
		}
		srcRead := src.read.Load()

		if err := sink.Resume(ctx); err != nil {
			t.Fatalf("cycle %d: Resume: %v", cycle, err)
		}
		if srv.Corked(stream) {
			t.Fatalf("cycle %d: the server stream is still corked after Resume", cycle)
		}
		waitUntil(t, "the source to be read again", func() bool { return src.read.Load() > srcRead })
		waitUntil(t, "the position to move after Resume", func() bool { return check() > held+0.02 })
		// Let the "started" event of the uncork arrive.
		time.Sleep(15 * time.Millisecond)
	}

	if src.seeked.Load() {
		t.Error("the source was moved")
	}
	commands := srv.Commands()
	if got := countCommands(commands, proto.OpCreatePlaybackStream); got != 1 {
		t.Errorf("%d streams were created, want 1", got)
	}
	if got := countCommands(commands, proto.OpDeletePlaybackStream); got != 0 {
		t.Errorf("%d streams were deleted before Stop, want 0", got)
	}
	sink.Stop()
	if sink.Position() != 0 {
		t.Errorf("position after Stop = %v", sink.Position())
	}
	if got := countCommands(srv.Commands(), proto.OpDeletePlaybackStream); got != 1 {
		t.Errorf("%d streams were deleted after Stop, want 1", got)
	}
}

func TestPulseSinkResumeWhenTheServerIsGone(t *testing.T) {
	srv := pulsetest.NewServer(t)
	sink := openFakeServerSink(t, srv)
	ctx := context.Background()
	if err := sink.Start(ctx, &countingSource{}, rate8k); err != nil {
		t.Fatal(err)
	}
	sink.Pause()
	held := sink.Position()

	srv.DropConnections()
	err := sink.Resume(ctx)
	if err == nil {
		t.Fatal("Resume succeeded after the server went away")
	}
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T: %v", err, err)
	}
	// Depending on the moment, the uncork cannot be sent, its answer does not
	// come, or the stream already knows that the connection is closed.
	if !appErr.HasRoute("PulseSendFailed") && !appErr.HasRoute("PulseConnectionLost") && !errors.Is(err, pulse.ErrConnectionClosed) {
		t.Errorf("unexpected failure path: %s", appErr.Route())
	}
	// Still paused: the position holds.
	time.Sleep(20 * time.Millisecond)
	if got := sink.Position(); got != held {
		t.Errorf("position moved from %v to %v after a failed Resume", held, got)
	}
}

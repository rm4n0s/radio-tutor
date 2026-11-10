package app

import (
	"context"
	"math"
	"sync"
	"testing"

	"github.com/rm4n0s/errors"
	gaudio "github.com/rm4n0s/glutter/audio"
	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"

	"github.com/rm4n0s/radio-tutor/internal/core/audio"
)

// rampSource gives frames of a rising ramp in stereo: the frame n is n/1000 on both channels.
type rampSource struct {
	mu   sync.Mutex
	next int
	end  int
}

func (r *rampSource) Read(dst []float32) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for n+2 <= len(dst) && r.next < r.end {
		v := float32(r.next) / 1000
		dst[n], dst[n+1] = v, v
		n += 2
		r.next++
	}
	return n
}

func (r *rampSource) SeekFrame(int64) {}

// glutterSystem is the audio of a test window, with a scripted speaker.
func glutterSystem(t *testing.T) (*gaudio.System, *gluttertest.TestAudio) {
	t.Helper()
	ts := gluttertest.New(t, gluttertest.Opts{})
	var binding *m.WidgetsBinding
	ts.PumpWidget(m.NewMaterialApp(m.MaterialAppOpts{Home: m.NewBuilder(func(bc m.BuildContext) m.Widget {
		binding = m.WidgetsBindingOf(bc)
		return m.NewText("window")
	})}))
	ts.PumpAndSettle()
	return gaudio.ForBinding(binding), ts.Audio()
}

func TestGlutterSinkPlaysTheSourceAndCountsFrames(t *testing.T) {
	sys, speaker := glutterSystem(t)
	sink := newGlutterSink(sys)
	defer sink.Close()
	if sink.Name() != "glutter" || sink.Position() != 0 {
		t.Fatalf("name %q, position %v", sink.Name(), sink.Position())
	}
	src := &rampSource{end: 4800}
	if err := sink.Start(context.Background(), src, audio.Format{SampleRate: 48000, Channels: 2}); err != nil {
		t.Fatal(err)
	}
	dev := speaker.Playback()
	if dev == nil {
		t.Fatal("the sink did not open the speaker")
	}
	out := dev.Pull(480, 480)
	if len(out) != 960 {
		t.Fatalf("the device pulled %d samples, want 960", len(out))
	}
	// the device may play at another rate: the samples go through its converter, so only the order and the count are checked
	if out[0] != 0 || out[len(out)-1] <= out[0] {
		t.Errorf("the device got %v ... %v, want a rising ramp from 0", out[0], out[len(out)-1])
	}
	if got := sink.Position(); math.Abs(got-0.01) > 0.005 {
		t.Errorf("Position = %v after 10 ms, want about 0.01", got)
	}
	sink.Pause()
	held := sink.Position()
	dev.Pull(480, 480)
	if got := sink.Position(); got != held {
		t.Errorf("Position moved from %v to %v while paused", held, got)
	}
	if err := sink.Resume(context.Background()); err != nil {
		t.Fatal(err)
	}
	dev.Pull(480, 480)
	if got := sink.Position(); got <= held {
		t.Errorf("Position = %v after Resume, want more than %v", got, held)
	}
	sink.Stop()
	if got := sink.Position(); got != 0 {
		t.Errorf("Position = %v after Stop, want 0", got)
	}
}

func TestGlutterSinkStartAgainRestartsThePosition(t *testing.T) {
	sys, speaker := glutterSystem(t)
	sink := newGlutterSink(sys)
	defer sink.Close()
	for range 2 {
		if err := sink.Start(context.Background(), &rampSource{end: 48000}, audio.Format{SampleRate: 48000, Channels: 2}); err != nil {
			t.Fatal(err)
		}
		if got := sink.Position(); got != 0 {
			t.Errorf("Position = %v at Start, want 0", got)
		}
		speaker.Playback().Pull(480, 480)
		if sink.Position() <= 0 {
			t.Error("Position did not move")
		}
	}
}

func TestGlutterSinkErrors(t *testing.T) {
	t.Run("the speaker cannot be opened", func(t *testing.T) {
		sys, speaker := glutterSystem(t)
		speaker.SetUnavailable(true)
		sink := newGlutterSink(sys)
		defer sink.Close()
		err := sink.Start(context.Background(), &rampSource{end: 10}, audio.Format{SampleRate: 48000, Channels: 2})
		appErr, ok := errors.FromError(err)
		if !ok {
			t.Fatalf("expected *errors.Error, got %T (%v)", err, err)
		}
		if !appErr.HasRoute("glutterSink.Start.GlutterSinkOpenFailed") {
			t.Errorf("unexpected failure path: %s", appErr.Route())
		}
	})
	t.Run("started after Close", func(t *testing.T) {
		sys, _ := glutterSystem(t)
		sink := newGlutterSink(sys)
		sink.Close()
		err := sink.Start(context.Background(), &rampSource{end: 10}, audio.Format{SampleRate: 48000, Channels: 2})
		appErr, ok := errors.FromError(err)
		if !ok || !appErr.HasRoute("glutterSink.Start.GlutterSinkClosed") {
			t.Errorf("error %v", err)
		}
	})
}

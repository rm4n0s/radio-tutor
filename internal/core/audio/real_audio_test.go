//go:build radiotutor_audio

package audio

import (
	"context"
	"math"
	"testing"
	"time"
)

// TestRealServerTone plays a 1 second 440 Hz tone through the real PulseAudio
// (or PipeWire) server, with a pause of 300 ms in the middle that does not
// count in the second. Run it by hand (docs/MANUAL-CHECKS.md, step 5.1):
//
//	go test -tags radiotutor_audio -run TestRealServerTone -v ./internal/core/audio
//
// You should hear a 1 s tone, with a short gap in the middle. The test fails
// if no server answers, if the position runs backwards, if the pause does not
// hold the position, or if the clip does not end after about 1 s of playing.
func TestRealServerTone(t *testing.T) {
	const rate = 44100
	clip := &Clip{SampleRate: rate, Channels: 1, Samples: make([]float32, rate)}
	for i := range clip.Samples {
		clip.Samples[i] = 0.2 * float32(math.Sin(2*math.Pi*440*float64(i)/rate))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sink, reason := OpenDefaultSink(ctx)
	if reason != nil {
		sink.Close()
		t.Fatalf("no PulseAudio server to play on (the sink fell back to %q): %v", sink.Name(), reason)
	}
	defer sink.Close()
	t.Logf("playing a 1 s tone through the %s sink: you should hear it", sink.Name())

	p := NewPlayer(sink, nil)
	defer p.Close()
	if err := p.Load(clip); err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	if err := p.Play(ctx); err != nil {
		t.Fatal(err)
	}

	last := 0.0
	check := func() float64 {
		pos := p.Position()
		if pos < last {
			t.Fatalf("position went back from %v to %v", last, pos)
		}
		last = pos
		return pos
	}
	for check() < 0.3 {
		time.Sleep(10 * time.Millisecond)
	}
	p.Pause()
	held := check()
	time.Sleep(300 * time.Millisecond)
	if got := check(); got != held {
		t.Fatalf("position moved from %v to %v while paused", held, got)
	}
	if err := p.Play(ctx); err != nil {
		t.Fatal(err)
	}

	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-p.Ended():
			if got := check(); got != 1 {
				t.Fatalf("position at the end = %v, want 1", got)
			}
			// 1 s of sound plus 0.3 s paused, plus the time Play and Pause took.
			if took := time.Since(began); took < 1200*time.Millisecond || took > 3*time.Second {
				t.Fatalf("the clip took %v of wall time, want about 1.3 s", took)
			}
			return
		case <-tick.C:
			check()
		case <-ctx.Done():
			t.Fatalf("the clip did not end; position %v, state %v", p.Position(), p.State())
		}
	}
}

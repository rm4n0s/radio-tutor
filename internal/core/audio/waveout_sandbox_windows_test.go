package audio

import (
	"context"
	"math"
	"os"
	"testing"
	"time"
)

// TestWaveOutSandboxDevice plays through the real winmm (under Wine) into a
// device that goes nowhere. It runs only when RADIO_TUTOR_SILENT_AUDIO_SANDBOX=1
// is set, which the script that runs the tests under Wine in a sandbox with no
// sound device nodes and no audio server sets; anywhere else it is skipped, so
// that it never plays on a real speaker.
func TestWaveOutSandboxDevice(t *testing.T) {
	if os.Getenv("RADIO_TUTOR_SILENT_AUDIO_SANDBOX") != "1" {
		t.Skip("needs the sandbox that has no real sound device (see docs/MANUAL-CHECKS.md for the real one)")
	}
	const rate = 22050
	clip := &Clip{SampleRate: rate, Channels: 1, Samples: make([]float32, 2*rate)}
	for i := range clip.Samples {
		clip.Samples[i] = 0.2 * float32(math.Sin(2*math.Pi*440*float64(i)/rate))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sink, err := NewWaveOutSink(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()
	p := NewPlayer(sink, nil)
	defer p.Close()
	if err := p.Load(clip); err != nil {
		t.Fatal(err)
	}
	if err := p.Play(ctx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(400 * time.Millisecond)
	p.Pause()
	held := p.Position()
	t.Logf("position at the pause: %.3f s", held)
	if held <= 0 {
		t.Fatalf("the position did not advance while playing: %v", held)
	}
	time.Sleep(300 * time.Millisecond)
	if got := p.Position(); got != held {
		t.Fatalf("the position moved while paused: %v -> %v", held, got)
	}
	if err := p.Play(ctx); err != nil {
		t.Fatal(err)
	}
	last := held
	deadline := time.After(10 * time.Second)
	for done := false; !done; {
		select {
		case <-p.Ended():
			done = true
		case <-deadline:
			t.Fatalf("the clip did not end; position %v of %v", p.Position(), clip.Seconds())
		case <-time.After(20 * time.Millisecond):
			pos := p.Position()
			if pos < last {
				t.Fatalf("the position went back from %v to %v", last, pos)
			}
			last = pos
		}
	}
	p.Stop()
}

// TestWaveOutSandboxStress pauses, resumes, stops and restarts many times on
// the real winmm, to catch a device or a buffer that is released too early.
func TestWaveOutSandboxStress(t *testing.T) {
	if os.Getenv("RADIO_TUTOR_SILENT_AUDIO_SANDBOX") != "1" {
		t.Skip("needs the sandbox that has no real sound device")
	}
	const rate = 8000
	clip := &Clip{SampleRate: rate, Channels: 2, Samples: make([]float32, 2*3*rate)}
	for i := range clip.Samples {
		clip.Samples[i] = 0.1 * float32(math.Sin(float64(i)/20))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sink, err := NewWaveOutSink(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()
	p := NewPlayer(sink, nil)
	defer p.Close()
	if err := p.Load(clip); err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 5; round++ {
		if err := p.Play(ctx); err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		for i := 0; i < 10; i++ {
			time.Sleep(15 * time.Millisecond)
			p.Pause()
			time.Sleep(5 * time.Millisecond)
			if err := p.Play(ctx); err != nil {
				t.Fatalf("round %d: resume: %v", round, err)
			}
		}
		p.SetVolume(30)
		p.SetMuted(round%2 == 0)
		time.Sleep(60 * time.Millisecond)
		p.Stop()
		if p.Position() != 0 {
			t.Fatalf("position after Stop = %v", p.Position())
		}
	}
}

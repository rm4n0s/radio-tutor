package audio

import (
	"context"
	"testing"
)

func TestGain(t *testing.T) {
	tests := []struct {
		volume int
		want   float32
	}{
		{100, 1},
		{50, 0.125},
		{0, 0},
		{10, 0.001},
		{-5, 0},
		{150, 1},
	}
	for _, tt := range tests {
		if got := Gain(tt.volume); got < tt.want-1e-6 || got > tt.want+1e-6 {
			t.Errorf("Gain(%d) = %v, want %v", tt.volume, got, tt.want)
		}
	}
}

func TestPlayerVolumeScalesTheSamples(t *testing.T) {
	tests := []struct {
		name   string
		volume int // -1: SetVolume is never called
		factor float32
	}{
		{"never set is unchanged", -1, 1},
		{"100 is unchanged", 100, 1},
		{"50 is an eighth", 50, 0.125},
		{"0 is silence", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clip := rampClip(500)
			p, sink, clock := newTestPlayer(t, clip)
			if tt.volume >= 0 {
				p.SetVolume(tt.volume)
			}
			if err := p.Play(context.Background()); err != nil {
				t.Fatal(err)
			}
			clock.Advance(ms(400))
			got := sink.Played()
			if len(got) != 400 {
				t.Fatalf("the device took %d samples, want 400", len(got))
			}
			for i, v := range got {
				want := clip.Samples[i] * tt.factor
				if v < want-1e-7 || v > want+1e-7 {
					t.Fatalf("sample %d = %v, want %v", i, v, want)
				}
			}
			// The clip still runs to its end, whatever the volume.
			clock.Advance(ms(200))
			if p.State() != StateEnded {
				t.Errorf("state = %v after the clip's length", p.State())
			}
			if endedCount(p) != 1 {
				t.Error("the end was not signalled")
			}
		})
	}
}

func TestPlayerVolumeTakesEffectOnTheNextStart(t *testing.T) {
	clip := rampClip(1000)
	p, sink, clock := newTestPlayer(t, clip)
	p.SetVolume(100)
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(ms(200))
	p.SetVolume(0)
	p.Pause()
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(ms(200))
	played := sink.Played()
	if len(played) != 400 {
		t.Fatalf("the device took %d samples, want 400", len(played))
	}
	for i, v := range played {
		if v != clip.Samples[i] {
			t.Fatalf("sample %d changed by a volume set while it played: %v", i, v)
		}
	}
	p.Stop()
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(ms(300))
	for i, v := range sink.Played() {
		if v != 0 {
			t.Fatalf("sample %d = %v after the volume went to 0", i, v)
		}
	}
}

func TestClipSourceClampsAfterTheGain(t *testing.T) {
	s := &clipSource{samples: []float32{0.25, -0.25, 0.9, -0.9}, channels: 2, scaled: true, gain: 2}
	dst := make([]float32, 4)
	if n := s.Read(dst); n != 4 {
		t.Fatalf("read %d samples", n)
	}
	want := []float32{0.5, -0.5, 1, -1}
	for i := range want {
		if dst[i] != want[i] {
			t.Errorf("sample %d = %v, want %v", i, dst[i], want[i])
		}
	}
}

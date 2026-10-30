package audio

import (
	"context"
	"math"
	"sync/atomic"
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

func TestPlayerVolumeChangesTheSoundThatPlays(t *testing.T) {
	// Every step plays 100 ms of the clip (100 samples), then changes the sound.
	type step struct {
		name   string
		change func(p *Player)
		factor float32 // of the 100 samples played after the change
	}
	steps := []step{
		{"starts at full volume", func(*Player) {}, 1},
		{"volume to 50 is an eighth", func(p *Player) { p.SetVolume(50) }, 0.125},
		{"mute is silence", func(p *Player) { p.SetMuted(true) }, 0},
		{"a volume set while muted stays silent", func(p *Player) { p.SetVolume(80) }, 0},
		{"unmute restores the volume that was set", func(p *Player) { p.SetMuted(false) }, Gain(80)},
		{"volume 0 is silence", func(p *Player) { p.SetVolume(0) }, 0},
		{"full volume again", func(p *Player) { p.SetVolume(100) }, 1},
	}
	clip := rampClip(1000)
	p, sink, clock := newTestPlayer(t, clip)
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	for i, st := range steps {
		st.change(p)
		clock.Advance(ms(100))
		p.Position() // pulls the samples that played
		got := sink.Played()
		if len(got) != 100*(i+1) {
			t.Fatalf("%s: the device took %d samples, want %d", st.name, len(got), 100*(i+1))
		}
		for j := 100 * i; j < len(got); j++ {
			want := clip.Samples[j] * st.factor
			if got[j] < want-1e-6 || got[j] > want+1e-6 {
				t.Fatalf("%s: sample %d = %v, want %v", st.name, j, got[j], want)
			}
		}
	}
	// The clip ran on all the way, and still ends.
	clock.Advance(ms(400))
	if p.State() != StateEnded {
		t.Errorf("state = %v after the clip's length", p.State())
	}
}

func TestPlayerMutedClipStillRunsAndEnds(t *testing.T) {
	clip := rampClip(500)
	p, sink, clock := newTestPlayer(t, clip)
	p.SetMuted(true)
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(ms(600))
	for i, v := range sink.Played() {
		if v != 0 {
			t.Fatalf("sample %d = %v while muted", i, v)
		}
	}
	if p.State() != StateEnded || endedCount(p) != 1 {
		t.Errorf("state = %v, ends = %d", p.State(), endedCount(p))
	}
}

func TestPlayerVolumeSetWhilePausedAppliesOnResume(t *testing.T) {
	clip := rampClip(1000)
	p, sink, clock := newTestPlayer(t, clip)
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(ms(200))
	p.Pause()
	p.SetMuted(true)
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(ms(200))
	p.Position()
	played := sink.Played()
	if len(played) != 400 {
		t.Fatalf("the device took %d samples, want 400", len(played))
	}
	for i, v := range played {
		want := clip.Samples[i]
		if i >= 200 {
			want = 0
		}
		if v != want {
			t.Fatalf("sample %d = %v, want %v", i, v, want)
		}
	}
}

func TestPlayerVolumeIsSafeAcrossGoroutines(t *testing.T) {
	clip := rampClip(1000)
	p, _, clock := newTestPlayer(t, clip)
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 200 {
			p.SetVolume(i % 101)
			p.SetMuted(i%7 == 0)
		}
	}()
	for range 10 {
		clock.Advance(ms(50))
		p.Position()
	}
	<-done
}

func TestClipSourceClampsAfterTheGain(t *testing.T) {
	level := new(atomic.Uint32)
	level.Store(math.Float32bits(2))
	s := &clipSource{samples: []float32{0.25, -0.25, 0.9, -0.9}, channels: 2, level: level}
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

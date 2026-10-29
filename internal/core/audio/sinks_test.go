package audio

import (
	"context"
	"slices"
	"testing"
)

func TestSilentSink(t *testing.T) {
	clock := NewFakeClock(epoch)
	s := NewSilentSink(clock)
	if s.Name() != "silent" {
		t.Fatalf("name = %q", s.Name())
	}
	if s.Position() != 0 {
		t.Fatalf("position before Start = %v", s.Position())
	}
	if err := s.Start(context.Background(), nil, Format{SampleRate: 1000, Channels: 1}); err != nil {
		t.Fatal(err)
	}
	clock.Advance(ms(250))
	if !near(s.Position(), 0.25) {
		t.Fatalf("position = %v", s.Position())
	}
	s.Pause()
	clock.Advance(ms(1000))
	if !near(s.Position(), 0.25) {
		t.Fatalf("position while paused = %v", s.Position())
	}
	if err := s.Resume(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(ms(250))
	if !near(s.Position(), 0.5) {
		t.Fatalf("position after resume = %v", s.Position())
	}
	s.Stop()
	if s.Position() != 0 {
		t.Fatalf("position after Stop = %v", s.Position())
	}
	if err := s.Start(context.Background(), nil, Format{}); err != nil {
		t.Fatal(err)
	}
	clock.Advance(ms(100))
	s.Close()
	if s.Position() != 0 {
		t.Fatalf("position after Close = %v", s.Position())
	}
}

func TestSinkStartCancelled(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name  string
		sink  Sink
		route string
	}{
		{"silent", NewSilentSink(NewFakeClock(epoch)), "SilentSink.Start.SilentStartCanceled"},
		{"fake", NewFakeSink(NewFakeClock(epoch)), "FakeSink.Start.FakeStartCanceled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantRoute(t, tt.sink.Start(cancelled, nil, Format{SampleRate: 1000, Channels: 1}), tt.route)
			if tt.sink.Position() != 0 {
				t.Fatalf("position = %v", tt.sink.Position())
			}
		})
	}
}

func TestFakeSink(t *testing.T) {
	clock := NewFakeClock(epoch)
	s := NewFakeSink(clock)
	if s.Name() != "fake" {
		t.Fatalf("name = %q", s.Name())
	}
	clip := rampClip(100)
	src := &clipSource{samples: clip.Samples, channels: 1}
	if err := s.Start(context.Background(), src, Format{SampleRate: 1000, Channels: 1}); err != nil {
		t.Fatal(err)
	}
	clock.Advance(ms(40))
	if got := len(s.Played()); got != 40 {
		t.Fatalf("played %d samples after 40 ms", got)
	}
	// Past the end of the source nothing more is taken.
	clock.Advance(ms(500))
	if got := len(s.Played()); got != 100 {
		t.Fatalf("played %d samples, want the 100 of the clip", got)
	}
	s.Close()
	if got, want := s.Calls(), []string{"start", "close"}; len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("calls = %v", got)
	}
}

func TestClipSource(t *testing.T) {
	tests := []struct {
		name     string
		channels int
		samples  int
		dst      int
		want     []int // sizes returned by successive reads
	}{
		{"mono in blocks", 1, 10, 4, []int{4, 4, 2, 0}},
		{"stereo reads whole frames", 2, 10, 5, []int{4, 4, 2, 0}},
		{"empty", 1, 0, 4, []int{0}},
		{"destination smaller than a frame", 2, 4, 1, []int{0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := &clipSource{samples: make([]float32, tt.samples), channels: tt.channels}
			dst := make([]float32, tt.dst)
			for i, want := range tt.want {
				if got := src.Read(dst); got != want {
					t.Fatalf("read %d = %d, want %d", i, got, want)
				}
			}
		})
	}
}

func TestClipSourceSeek(t *testing.T) {
	tests := []struct {
		name     string
		channels int
		frame    int64
		want     []float32 // what a read of 4 samples gives next
	}{
		{"mono middle", 1, 3, []float32{3, 4, 5, 6}},
		{"mono start", 1, 0, []float32{0, 1, 2, 3}},
		{"mono negative is the start", 1, -5, []float32{0, 1, 2, 3}},
		{"mono past the end is the end", 1, 99, nil},
		{"stereo middle", 2, 2, []float32{4, 5, 6, 7}},
		{"stereo past the end is the end", 2, 99, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			samples := make([]float32, 10)
			for i := range samples {
				samples[i] = float32(i)
			}
			src := &clipSource{samples: samples, channels: tt.channels}
			src.Read(make([]float32, 6)) // somewhere else
			src.SeekFrame(tt.frame)
			dst := make([]float32, 4)
			n := src.Read(dst)
			if !slices.Equal(dst[:n], tt.want) {
				t.Fatalf("read %v after SeekFrame(%d), want %v", dst[:n], tt.frame, tt.want)
			}
		})
	}
}

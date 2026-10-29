package audio

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/problem"
)

func TestCheckNarration(t *testing.T) {
	ok := Info{SampleRate: 44100, Channels: 1, Frames: 44100 * 60, Bytes: 1 << 20}
	with := func(f func(*Info)) Info {
		i := ok
		f(&i)
		return i
	}
	tests := []struct {
		name string
		info Info
		max  float64
		want []string // a substring of each expected problem, in order
	}{
		{"valid mono 44100", ok, MaxNarrationSeconds, nil},
		{"valid stereo 48000", with(func(i *Info) { i.Channels, i.SampleRate = 2, 48000 }), MaxNarrationSeconds, nil},
		{"valid 22050", with(func(i *Info) { i.SampleRate = 22050 }), MaxNarrationSeconds, nil},
		{"exactly the limit", with(func(i *Info) { i.Frames = 44100 * 600 }), MaxNarrationSeconds, nil},
		{"unknown size", with(func(i *Info) { i.Bytes = 0 }), MaxNarrationSeconds, nil},
		{"exactly 20 MiB", with(func(i *Info) { i.Bytes = MaxBytes }), MaxNarrationSeconds, nil},
		{"16000 Hz", with(func(i *Info) { i.SampleRate = 16000 }), MaxNarrationSeconds, []string{"16000 Hz"}},
		{"96000 Hz", with(func(i *Info) { i.SampleRate = 96000 }), MaxNarrationSeconds, []string{"96000 Hz"}},
		{"surround", with(func(i *Info) { i.Channels = 6 }), MaxNarrationSeconds, []string{"6 channels"}},
		{"no channels", with(func(i *Info) { i.Channels = 0 }), MaxNarrationSeconds, []string{"0 channels"}},
		{"empty", with(func(i *Info) { i.Frames = 0 }), MaxNarrationSeconds, []string{"empty"}},
		{"too long narration", with(func(i *Info) { i.Frames = 44100*600 + 1 }), MaxNarrationSeconds, []string{"600.0 seconds long; the limit is 600"}},
		{"prompt over a minute", with(func(i *Info) { i.Frames = 44100 * 61 }), MaxPromptSeconds, []string{"61.0 seconds long; the limit is 60"}},
		{"prompt of a minute", ok, MaxPromptSeconds, nil},
		{"too big", with(func(i *Info) { i.Bytes = MaxBytes + 1 }), MaxNarrationSeconds, []string{"20971521 bytes"}},
		{"everything wrong", Info{SampleRate: 8000, Channels: 3, Frames: 8000 * 700, Bytes: 30 << 20}, MaxNarrationSeconds,
			[]string{"8000 Hz", "3 channels", "700.0 seconds", "31457280 bytes"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckNarration(tt.info, tt.max)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d problems %v, want %d", len(got), got, len(tt.want))
			}
			for i, p := range got {
				if p.Warning || p.Path != "" {
					t.Errorf("problem %d = %+v, want an error with an empty path", i, p)
				}
				if !strings.Contains(p.Message, tt.want[i]) {
					t.Errorf("problem %d = %q, want it to contain %q", i, p.Message, tt.want[i])
				}
			}
		})
	}
}

// The probed test files go through CheckNarration as the validator will do.
func TestCheckProbedFiles(t *testing.T) {
	tests := []struct {
		file string
		want int // number of problems
	}{
		{"tone-mono-44100.ogg", 0},
		{"tone-stereo-48000.ogg", 0},
		{"tone-mono-22050.ogg", 0},
		{"tone-mono-16000.ogg", 1},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tt.file))
			if err != nil {
				t.Fatal(err)
			}
			info, err := Probe(context.Background(), bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			ps := problem.InFile("lessons/l1/narration.ogg", CheckNarration(info, MaxNarrationSeconds))
			if len(ps) != tt.want {
				t.Errorf("got %v, want %d problems", ps, tt.want)
			}
			if tt.want > 0 && !strings.HasPrefix(ps[0].String(), "lessons/l1/narration.ogg: the sample rate is 16000 Hz") {
				t.Errorf("problem = %q", ps[0])
			}
		})
	}
}

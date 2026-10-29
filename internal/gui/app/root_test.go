package app

import (
	"testing"

	"github.com/rm4n0s/glutter/foundation"
	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"
)

func TestRootShowsTitle(t *testing.T) {
	tests := []struct {
		name string
		size m.Size
		text string
		want int
	}{
		{"minimum window", m.Size{Width: MinWindowWidth, Height: MinWindowHeight}, Title, 1},
		{"larger window", m.Size{Width: 1280, Height: 800}, Title, 1},
		{"other text is absent", m.Size{Width: MinWindowWidth, Height: MinWindowHeight}, "Something else", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !foundation.DebugMode {
				t.Skip("the platform override is a debug-only feature")
			}
			foundation.SetDebugDefaultTargetPlatformOverride(foundation.Some(foundation.TargetPlatformLinux))
			t.Cleanup(func() {
				foundation.SetDebugDefaultTargetPlatformOverride(foundation.None[foundation.TargetPlatform]())
			})
			ts := gluttertest.New(t, gluttertest.Opts{Size: tt.size})
			ts.PumpWidget(NewRoot())
			if n := ts.Count(gluttertest.ByText(tt.text)); n != tt.want {
				t.Errorf("the text %q is shown %d times, want %d", tt.text, n, tt.want)
			}
		})
	}
}

package app

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/rm4n0s/errors"
)

func TestWindowIconHasAllSizes(t *testing.T) {
	icon, err := windowIcon()
	if err != nil {
		t.Fatal(err)
	}
	if err := icon.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(icon.Images) != len(iconSizes) {
		t.Fatalf("%d pictures, want %d", len(icon.Images), len(iconSizes))
	}
	for i, img := range icon.Images {
		if img.Size != iconSizes[i] {
			t.Errorf("picture %d has size %d, want %d", i, img.Size, iconSizes[i])
		}
		opaque := 0
		for p := 3; p < len(img.RGBA); p += 4 {
			if img.RGBA[p] == 255 {
				opaque++
			}
		}
		if opaque < len(img.RGBA)/4/2 {
			t.Errorf("picture %d is mostly transparent (%d opaque pixels)", i, opaque)
		}
	}
}

func TestWindowOptsCarryTheIcon(t *testing.T) {
	opts := windowOpts(Title, AlreadyRunningSize, AlreadyRunningSize, nil)
	if opts.Window.Icon.IsZero() {
		t.Fatal("the window has no icon")
	}
}

func TestIconFromPNGFailures(t *testing.T) {
	tests := []struct {
		name  string
		data  []byte
		route string
	}{
		{"not a png", []byte("nope"), "iconFromPNG.WindowIconDecode"},
		{"not square", wideImage(t), "iconFromPNG.WindowIconNotSquare"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := iconFromPNG(tt.data, []int{32})
			appErr, ok := errors.FromError(err)
			if !ok {
				t.Fatalf("expected *errors.Error, got %T", err)
			}
			if !appErr.HasRoute(tt.route) {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
		})
	}
}

// wideImage returns a PNG that is 4 pixels wide and 2 high.
func wideImage(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 4, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

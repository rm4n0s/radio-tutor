package main

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/rm4n0s/errors"
)

func TestIconIsSquareRoundedAndColourful(t *testing.T) {
	data, err := encode()
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds(); got != image.Rect(0, 0, 256, 256) {
		t.Fatalf("bounds = %v, want 256x256", got)
	}
	tests := []struct {
		name       string
		x, y       int
		wantOpaque bool
	}{
		{"corner is transparent", 0, 0, false},
		{"edge middle is opaque", 128, 4, true},
		{"center is opaque", 128, 128, true},
	}
	for _, tt := range tests {
		_, _, _, a := img.At(tt.x, tt.y).RGBA()
		if (a == 0xffff) != tt.wantOpaque {
			t.Errorf("%s: alpha = %#x", tt.name, a)
		}
	}
	colours := map[[4]uint32]bool{}
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			colours[[4]uint32{r, g, b, a}] = true
		}
	}
	if len(colours) < 20 {
		t.Errorf("only %d colours, the icon looks blank", len(colours))
	}
}

func TestRunWritesTheFileAndFailsInAMissingFolder(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "icon.png")
	if err := run(ok); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(ok); err != nil || st.Size() == 0 {
		t.Fatalf("icon not written: %v", err)
	}
	err := run(filepath.Join(dir, "missing", "icon.png"))
	appErr, isApp := errors.FromError(err)
	if !isApp {
		t.Fatalf("expected *errors.Error, got %T", err)
	}
	if !appErr.HasRoute("run.IconGenWrite") {
		t.Errorf("unexpected failure path: %s", appErr.Route())
	}
}

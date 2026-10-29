package testimage

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestSumPNG(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(SumPNG()))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b != image.Rect(0, 0, 600, 200) {
		t.Errorf("size = %v, want 600x200", b)
	}
	// Black strokes on white: both colors are present.
	var dark, light int
	for y := 0; y < 200; y++ {
		for x := 0; x < 600; x++ {
			r, _, _, _ := img.At(x, y).RGBA()
			switch {
			case r < 0x2000:
				dark++
			case r > 0xe000:
				light++
			}
		}
	}
	if dark < 1500 || light < 60000 {
		t.Errorf("dark pixels = %d, light pixels = %d; want a drawing on white", dark, light)
	}
	a, b := SumPNG(), SumPNG()
	a[0] ^= 0xff
	if bytes.Equal(a, b) {
		t.Error("SumPNG returned a shared slice")
	}
}

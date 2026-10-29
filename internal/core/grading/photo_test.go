package grading

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"testing"

	"github.com/rm4n0s/errors"
)

// The corner colors of the test photos.
var (
	red    = color.RGBA{220, 30, 30, 255}
	green  = color.RGBA{30, 200, 30, 255}
	blue   = color.RGBA{30, 30, 220, 255}
	yellow = color.RGBA{230, 220, 30, 255}
)

// cornerImage is a w by h image with a block of its own color in each corner
// (red top-left, green top-right, blue bottom-left, yellow bottom-right) on a
// white background, so every orientation can be told apart.
func cornerImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.RGBA{255, 255, 255, 255}
			left, top := x < w/4, y < h/4
			right, bottom := x >= w-w/4, y >= h-h/4
			switch {
			case left && top:
				c = red
			case right && top:
				c = green
			case left && bottom:
				c = blue
			case right && bottom:
				c = yellow
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// exifSegment builds an APP1 segment holding an orientation tag, in the given
// byte order.
func exifSegment(order binary.ByteOrder, value uint16) []byte {
	var tiff bytes.Buffer
	if order == binary.LittleEndian {
		tiff.WriteString("II")
	} else {
		tiff.WriteString("MM")
	}
	put16 := func(v uint16) { _ = binary.Write(&tiff, order, v) }
	put32 := func(v uint32) { _ = binary.Write(&tiff, order, v) }
	put16(42)
	put32(8)
	put16(1) // one entry
	put16(exifTagOrientation)
	put16(3) // SHORT
	put32(1)
	put16(value)
	put16(0)
	put32(0) // no next IFD
	return appSegment(0xE1, append([]byte("Exif\x00\x00"), tiff.Bytes()...))
}

// appSegment builds a JPEG marker segment 0xFF marker, length, payload.
func appSegment(marker byte, payload []byte) []byte {
	seg := []byte{0xFF, marker, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	return append(seg, payload...)
}

// withSegment inserts a segment right after the SOI marker of a JPEG.
func withSegment(jpg, seg []byte) []byte {
	out := append([]byte(nil), jpg[:2]...)
	out = append(out, seg...)
	return append(out, jpg[2:]...)
}

// nearest names the corner color that c is closest to, or "" when none is
// close enough.
func nearest(c color.Color) string {
	r, g, b, _ := c.RGBA()
	got := [3]int{int(r >> 8), int(g >> 8), int(b >> 8)}
	best, bestDist := "", 60*60*3
	for name, ref := range map[string]color.RGBA{"red": red, "green": green, "blue": blue, "yellow": yellow, "white": {255, 255, 255, 255}} {
		d := sq(got[0]-int(ref.R)) + sq(got[1]-int(ref.G)) + sq(got[2]-int(ref.B))
		if d < bestDist {
			best, bestDist = name, d
		}
	}
	return best
}

func sq(v int) int { return v * v }

// corners returns the colors found at the four corners of the image (a little
// inside, to stay clear of the border), top-left first.
func corners(img image.Image) [4]string {
	b := img.Bounds()
	dx, dy := max(2, b.Dx()/16), max(2, b.Dy()/16)
	return [4]string{
		nearest(img.At(b.Min.X+dx, b.Min.Y+dy)),
		nearest(img.At(b.Max.X-1-dx, b.Min.Y+dy)),
		nearest(img.At(b.Min.X+dx, b.Max.Y-1-dy)),
		nearest(img.At(b.Max.X-1-dx, b.Max.Y-1-dy)),
	}
}

func decodeJPEG(t *testing.T, data []byte) image.Image {
	t.Helper()
	if !bytes.HasPrefix(data, jpegMagic) {
		t.Fatalf("output is not a JPEG: % x", data[:min(8, len(data))])
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestNormalizePhotoOrientation(t *testing.T) {
	// The corner colors (top-left, top-right, bottom-left, bottom-right) of a
	// photo whose EXIF orientation is k, once turned upright, by the source
	// corners TL=red TR=green BL=blue BR=yellow (the EXIF specification's
	// table).
	tests := []struct {
		orientation int
		want        [4]string
		swapped     bool
	}{
		{1, [4]string{"red", "green", "blue", "yellow"}, false},
		{2, [4]string{"green", "red", "yellow", "blue"}, false},
		{3, [4]string{"yellow", "blue", "green", "red"}, false},
		{4, [4]string{"blue", "yellow", "red", "green"}, false},
		{5, [4]string{"red", "blue", "green", "yellow"}, true},
		{6, [4]string{"blue", "red", "yellow", "green"}, true},
		{7, [4]string{"yellow", "green", "blue", "red"}, true},
		{8, [4]string{"green", "yellow", "red", "blue"}, true},
	}
	src := encodeJPEG(t, cornerImage(160, 96))
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		for _, tt := range tests {
			name := "orientation " + string(rune('0'+tt.orientation)) + " " + order.String()
			t.Run(name, func(t *testing.T) {
				data := withSegment(src, exifSegment(order, uint16(tt.orientation)))
				out, err := NormalizePhoto(context.Background(), data)
				if err != nil {
					t.Fatal(err)
				}
				img := decodeJPEG(t, out)
				w, h := img.Bounds().Dx(), img.Bounds().Dy()
				if tt.swapped && (w != 96 || h != 160) || !tt.swapped && (w != 160 || h != 96) {
					t.Errorf("size = %dx%d", w, h)
				}
				if got := corners(img); got != tt.want {
					t.Errorf("corners = %v, want %v", got, tt.want)
				}
			})
		}
	}
}

func TestNormalizePhotoScaling(t *testing.T) {
	tests := []struct {
		name     string
		w, h     int
		exif     uint16
		wantW    int
		wantH    int
		wantCorn [4]string
	}{
		{"landscape scaled down", 3000, 1000, 0, 2048, 682, [4]string{"red", "green", "blue", "yellow"}},
		{"portrait scaled down", 1000, 3000, 0, 682, 2048, [4]string{"red", "green", "blue", "yellow"}},
		{"exactly the limit", 2048, 1024, 0, 2048, 1024, [4]string{"red", "green", "blue", "yellow"}},
		{"one over the limit", 2049, 1024, 0, 2048, 1023, [4]string{"red", "green", "blue", "yellow"}},
		{"small is not enlarged", 100, 50, 0, 100, 50, [4]string{"red", "green", "blue", "yellow"}},
		{"rotated after scaling", 3000, 1000, 6, 682, 2048, [4]string{"blue", "red", "yellow", "green"}},
		{"very thin stays one pixel", 20000, 1, 0, 2048, 1, [4]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := encodeJPEG(t, cornerImage(tt.w, tt.h))
			if tt.exif != 0 {
				data = withSegment(data, exifSegment(binary.BigEndian, tt.exif))
			}
			out, err := NormalizePhoto(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			img := decodeJPEG(t, out)
			if w, h := img.Bounds().Dx(), img.Bounds().Dy(); w != tt.wantW || h != tt.wantH {
				t.Errorf("size = %dx%d, want %dx%d", w, h, tt.wantW, tt.wantH)
			}
			if tt.h > 1 {
				if got := corners(img); got != tt.wantCorn {
					t.Errorf("corners = %v, want %v", got, tt.wantCorn)
				}
			}
		})
	}
}

func TestNormalizePhotoQuality85(t *testing.T) {
	out, err := NormalizePhoto(context.Background(), encodeJPEG(t, cornerImage(64, 64)))
	if err != nil {
		t.Fatal(err)
	}
	// The first quantization table is the luminance table: its first entry
	// is 16 for quality 50 and 5 for quality 85 (jpeg.Encode's scaling).
	i := bytes.Index(out, []byte{0xFF, 0xDB})
	if i < 0 || len(out) < i+6 {
		t.Fatal("no quantization table")
	}
	if got := out[i+5]; got != 5 {
		t.Errorf("first luminance quantizer = %d, want 5 (quality 85)", got)
	}
}

func TestNormalizePhotoPNG(t *testing.T) {
	t.Run("opaque png", func(t *testing.T) {
		out, err := NormalizePhoto(context.Background(), encodePNG(t, cornerImage(120, 80)))
		if err != nil {
			t.Fatal(err)
		}
		img := decodeJPEG(t, out)
		if got := corners(img); got != [4]string{"red", "green", "blue", "yellow"} {
			t.Errorf("corners = %v", got)
		}
	})
	t.Run("transparent areas become white", func(t *testing.T) {
		img := image.NewNRGBA(image.Rect(0, 0, 64, 64)) // fully transparent black
		out, err := NormalizePhoto(context.Background(), encodePNG(t, img))
		if err != nil {
			t.Fatal(err)
		}
		if got := nearest(decodeJPEG(t, out).At(32, 32)); got != "white" {
			t.Errorf("transparent pixel became %s", got)
		}
	})
	t.Run("large png is scaled", func(t *testing.T) {
		out, err := NormalizePhoto(context.Background(), encodePNG(t, cornerImage(2600, 400)))
		if err != nil {
			t.Fatal(err)
		}
		if b := decodeJPEG(t, out).Bounds(); b.Dx() != 2048 || b.Dy() != 315 {
			t.Errorf("size = %v", b)
		}
	})
}

// patchPNGSize rewrites the width and height in a PNG's IHDR chunk and fixes
// the chunk's checksum.
func patchPNGSize(data []byte, w, h uint32) []byte {
	out := append([]byte(nil), data...)
	binary.BigEndian.PutUint32(out[16:], w)
	binary.BigEndian.PutUint32(out[20:], h)
	binary.BigEndian.PutUint32(out[29:], crc32.ChecksumIEEE(out[12:29]))
	return out
}

// patchJPEGSize rewrites the dimensions in the first SOF0 marker of a JPEG.
func patchJPEGSize(t *testing.T, data []byte, w, h uint16) []byte {
	t.Helper()
	i := bytes.Index(data, []byte{0xFF, 0xC0})
	if i < 0 {
		t.Fatal("no SOF0 marker")
	}
	out := append([]byte(nil), data...)
	binary.BigEndian.PutUint16(out[i+5:], h)
	binary.BigEndian.PutUint16(out[i+7:], w)
	return out
}

func TestNormalizePhotoRejects(t *testing.T) {
	good := encodeJPEG(t, cornerImage(64, 64))
	goodPNG := encodePNG(t, cornerImage(64, 64))
	tests := []struct {
		name  string
		data  []byte
		route string
	}{
		{"empty", nil, "normalizePhoto.PhotoUnsupported"},
		{"text", []byte("hello, this is not a photo"), "normalizePhoto.PhotoUnsupported"},
		{"gif", []byte("GIF89a\x01\x00\x01\x00"), "normalizePhoto.PhotoUnsupported"},
		{"webp", []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), "normalizePhoto.PhotoUnsupported"},
		{"one byte of a jpeg", good[:1], "normalizePhoto.PhotoUnsupported"},
		{"jpeg magic and junk", append([]byte{0xFF, 0xD8, 0xFF}, bytes.Repeat([]byte{0x12}, 40)...), "normalizePhoto.PhotoUndecodable"},
		{"png magic and junk", append(append([]byte(nil), pngMagic...), bytes.Repeat([]byte{0x12}, 40)...), "normalizePhoto.PhotoUndecodable"},
		{"jpeg cut inside its header", good[:len(good)/2], "normalizePhoto.PhotoUndecodable"},
		{"jpeg cut inside its data", good[:len(good)-20], "normalizePhoto.PhotoDecodeFailed"},
		{"truncated png", goodPNG[:len(goodPNG)-40], "normalizePhoto.PhotoDecodeFailed"},
		{"png of 60 megapixels", patchPNGSize(goodPNG, 10000, 6000), "normalizePhoto.PhotoTooLarge"},
		{"png taller than wide, 50 megapixels plus one", patchPNGSize(goodPNG, 1, 50_000_001), "normalizePhoto.PhotoTooLarge"},
		{"png with absurd sides", patchPNGSize(goodPNG, 1<<30, 1<<30), "normalizePhoto.PhotoUndecodable"},
		{"jpeg of 60 megapixels", patchJPEGSize(t, good, 10000, 6000), "normalizePhoto.PhotoTooLarge"},
		{"jpeg of 65535 by 65535", patchJPEGSize(t, good, 65535, 65535), "normalizePhoto.PhotoTooLarge"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := NormalizePhoto(context.Background(), tt.data)
			if err == nil {
				t.Fatalf("accepted; got %d bytes", len(out))
			}
			wantTag(t, err, tt.route)
		})
	}
}

func TestNormalizePhotoAtTheLimit(t *testing.T) {
	// 50,000,000 pixels is allowed by the header check; the (tiny) data then
	// fails to decode, which is a different failure from PhotoTooLarge.
	good := encodePNG(t, cornerImage(64, 64))
	_, err := NormalizePhoto(context.Background(), patchPNGSize(good, 10000, 5000))
	appErr, ok := errors.FromError(err)
	if !ok || appErr.Tag == "PhotoTooLarge" {
		t.Errorf("50 megapixels exactly was refused as too large: %v", err)
	}
}

func TestNormalizePhotoCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NormalizePhoto(ctx, encodeJPEG(t, cornerImage(32, 32)))
	wantTag(t, err, "NormalizePhoto->normalizePhoto.PhotoCanceled")
	if !errors.Is(err, context.Canceled) {
		t.Error("the context error is lost")
	}
}

func TestNormalizePhotoEncodeFailure(t *testing.T) {
	failing := func(io.Writer, image.Image, *jpeg.Options) error { return io.ErrClosedPipe }
	_, err := normalizePhoto(context.Background(), encodeJPEG(t, cornerImage(32, 32)), failing)
	wantTag(t, err, "normalizePhoto.PhotoEncodeFailed")
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Error("the encoder error is lost")
	}
}

func TestOrientIgnoresOutOfRangeValues(t *testing.T) {
	img := cornerImage(20, 10)
	for _, o := range []int{-1, 0, 1, 9, 100} {
		if got := orient(img, o); got != img {
			t.Errorf("orient(%d) changed the image", o)
		}
	}
}

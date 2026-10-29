package grading

import (
	"bytes"
	"context"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/rm4n0s/errors"
	"golang.org/x/image/draw"
)

// Photo limits (SPEC §10.3).
const (
	// MaxPhotoSide is the longest side, in pixels, of a normalized photo.
	MaxPhotoSide = 2048
	// MaxPhotoPixels is the most pixels (width times height) of a photo that
	// is decoded; larger ones are refused before decoding, because their
	// decoded size would exhaust memory.
	MaxPhotoPixels = 50_000_000
	// PhotoQuality is the JPEG quality of a normalized photo.
	PhotoQuality = 85
)

var (
	jpegMagic = []byte{0xFF, 0xD8, 0xFF}
	pngMagic  = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}
)

// NormalizePhoto turns an uploaded JPEG or PNG into the photo that is stored
// and sent to the grader (SPEC §10.3): decoded, turned upright by its EXIF
// orientation (JPEG), scaled so that its long side is at most MaxPhotoSide
// (never enlarged), and encoded as JPEG with quality PhotoQuality. The kind of
// file is decided by its first bytes, not by any name. The image's size is
// read before it is decoded and a photo above MaxPhotoPixels is refused.
func NormalizePhoto(ctx context.Context, data []byte) ([]byte, error) {
	return normalizePhoto(ctx, data, jpeg.Encode)
}

// normalizePhoto is NormalizePhoto with the JPEG encoder passed in, so that a
// test can make the encoding fail.
func normalizePhoto(ctx context.Context, data []byte, encode func(io.Writer, image.Image, *jpeg.Options) error) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.NewErr("PhotoCanceled", err)
	}
	isJPEG := bytes.HasPrefix(data, jpegMagic)
	if !isJPEG && !bytes.HasPrefix(data, pngMagic) {
		return nil, errors.New("PhotoUnsupported", "only JPEG and PNG photos are supported")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errors.NewErr("PhotoUndecodable", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, errors.New("PhotoUndecodable", "the photo has no size", "width", cfg.Width, "height", cfg.Height)
	}
	if int64(cfg.Width)*int64(cfg.Height) > MaxPhotoPixels {
		return nil, errors.New("PhotoTooLarge", "the photo has too many pixels", "width", cfg.Width, "height", cfg.Height, "max_pixels", MaxPhotoPixels)
	}

	var img image.Image
	if isJPEG {
		img, err = jpeg.Decode(bytes.NewReader(data))
	} else {
		img, err = png.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return nil, errors.NewErr("PhotoDecodeFailed", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.NewErr("PhotoCanceled", err)
	}

	upright := fitOpaque(img, MaxPhotoSide)
	if isJPEG {
		upright = orient(upright, jpegOrientation(data))
	}

	var out bytes.Buffer
	if err := encode(&out, upright, &jpeg.Options{Quality: PhotoQuality}); err != nil {
		return nil, errors.NewErr("PhotoEncodeFailed", err)
	}
	return out.Bytes(), nil
}

// fitOpaque returns the image as an opaque RGBA image whose long side is at
// most maxSide, keeping its proportions (a smaller image keeps its size).
// Transparent areas of a PNG become white: JPEG has no alpha, and black would
// hide the paper. Scaling reads the decoded image directly, so only the
// result is allocated in RGBA.
func fitOpaque(src image.Image, maxSide int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if long := max(w, h); long > maxSide {
		w, h = max(1, w*maxSide/long), max(1, h*maxSide/long)
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	stddraw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, stddraw.Src)
	if w == b.Dx() && h == b.Dy() {
		stddraw.Draw(dst, dst.Bounds(), src, b.Min, stddraw.Over)
	} else {
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	}
	return dst
}

// orient returns the image turned upright for the EXIF orientation 1 to 8
// (1 is upright). Other values return the image as it is.
func orient(src *image.RGBA, o int) *image.RGBA {
	if o < 2 || o > 8 {
		return src
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for dy := range dh {
		for dx := range dw {
			var sx, sy int
			switch o {
			case 2: // mirrored horizontally
				sx, sy = w-1-dx, dy
			case 3: // rotated 180
				sx, sy = w-1-dx, h-1-dy
			case 4: // mirrored vertically
				sx, sy = dx, h-1-dy
			case 5: // transposed
				sx, sy = dy, dx
			case 6: // rotated 90 clockwise
				sx, sy = dy, h-1-dx
			case 7: // transversed
				sx, sy = w-1-dy, h-1-dx
			case 8: // rotated 90 counter-clockwise
				sx, sy = w-1-dy, dx
			}
			si := src.PixOffset(sx, sy)
			di := dst.PixOffset(dx, dy)
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	return dst
}

//go:build !windows

package menuentry

import (
	"bytes"
	"image"
	"image/png"
	"io"

	"github.com/rm4n0s/errors"
	xdraw "golang.org/x/image/draw"
)

// scaleIcons decodes the square PNG and returns it scaled to each size, as PNG
// files. It fails with the tags IconDecodeFailed, IconNotSquare, IconTooSmall and
// IconEncodeFailed.
func scaleIcons(data []byte, sizes []int) ([][]byte, error) {
	return scaleIconsWith(data, sizes, png.Encode)
}

// scaleIconsWith is scaleIcons with the PNG encoder given as a function.
func scaleIconsWith(data []byte, sizes []int, encode func(io.Writer, image.Image) error) ([][]byte, error) {
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.NewErr("IconDecodeFailed", err)
	}
	b := src.Bounds()
	if b.Dx() != b.Dy() || b.Dx() == 0 {
		return nil, errors.New("IconNotSquare", "the icon picture must be square", "width", b.Dx(), "height", b.Dy())
	}
	largest := 0
	for _, s := range sizes {
		largest = max(largest, s)
	}
	if b.Dx() < largest {
		return nil, errors.New("IconTooSmall", "the icon picture is smaller than the largest icon size", "width", b.Dx(), "needed", largest)
	}
	out := make([][]byte, 0, len(sizes))
	for _, s := range sizes {
		dst := image.NewNRGBA(image.Rect(0, 0, s, s))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
		var buf bytes.Buffer
		if err := encode(&buf, dst); err != nil {
			return nil, errors.NewErr("IconEncodeFailed", err).SetMetadata("size", s)
		}
		out = append(out, buf.Bytes())
	}
	return out, nil
}

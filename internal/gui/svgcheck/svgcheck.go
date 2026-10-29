// Package svgcheck checks that Radio Tutor can draw an SVG, with the parser
// and the painter that the lesson player uses (CASSETTE-FORMAT §10 item 5).
//
// The core validates the rules of the cassette format but does not know the
// renderer; this package gives the core's archive validation a
// RenderChecker made with Glutter's svg package, so that `radio-tutor validate`
// and the importer report a picture that Glutter cannot draw. It needs no
// window, no binding and no display: svg.Parse and the recording of a picture
// are plain Go, and text is set in the fonts that Glutter bundles.
package svgcheck

import (
	"context"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/glutter/svg"
)

// Checker implements archive.RenderChecker with Glutter's SVG parser. The zero
// value is ready to use and is safe for concurrent use.
type Checker struct{}

// CheckSVG parses data with svg.Parse and records a picture of it, which draws
// every shape, so that what Glutter finds wrong while drawing comes up too. It
// returns an error when the document cannot be parsed (the XML is not well
// formed, the root is not svg, or a value that must be valid is not, such as a
// viewBox with a negative size) or has no area to draw. name says where the
// document is from and is kept in the error's metadata. Tags: SvgCheckCanceled,
// SvgCheckParseFailed (the Glutter error is the original error),
// SvgCheckNoSize.
func (Checker) CheckSVG(ctx context.Context, name string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return errors.NewErr("SvgCheckCanceled", err).SetMetadata("name", name)
	}
	doc, err := svg.Parse(data)
	if err != nil {
		return errors.NewErr("SvgCheckParseFailed", err).SetMetadata("name", name)
	}
	size := doc.Size()
	if size.Width <= 0 || size.Height <= 0 {
		return errors.New("SvgCheckNoSize", "the picture has no width or height to draw", "name", name)
	}
	// Recording walks the whole tree and draws every element into a picture;
	// the picture itself is not needed.
	doc.Picture().Dispose()
	return nil
}

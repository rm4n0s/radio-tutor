package worksheet

import (
	"math"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/qrcode"
)

// MinQRSize is the smallest side, in logical pixels, of the QR code with its
// quiet zone (SPEC §10.1).
const MinQRSize = 280.0

// SmallQRSize is the smallest side of the QR code when a phone has the link
// open already and the code is only for another phone (SPEC §10.1).
const SmallQRSize = 180.0

// qrModuleSize is the side of one module: a whole number of logical pixels, so
// that the squares are crisp at the usual scale factors, and large enough for
// the whole code to be at least minSide.
func qrModuleSize(total int, minSide float64) float64 {
	if total <= 0 {
		return 1
	}
	return math.Ceil(minSide / float64(total))
}

// qrPainter draws a QR matrix: a light square with the quiet zone, and the dark
// modules as rectangles (a run of dark modules in a row is one rectangle).
type qrPainter struct {
	m.CustomPainterBase
	matrix qrcode.Matrix
	dark   m.Color
	light  m.Color
}

func newQRPainter(matrix qrcode.Matrix) *qrPainter {
	return &qrPainter{
		CustomPainterBase: m.NewCustomPainterBase(nil),
		matrix:            matrix,
		dark:              m.NewColor(0xFF000000),
		light:             m.NewColor(0xFFFFFFFF),
	}
}

// Paint draws the code to fill the box; the box is square (see qrCode).
func (p *qrPainter) Paint(canvas *m.Canvas, size m.Size) {
	total := p.matrix.Total()
	if total <= 0 {
		return
	}
	module := size.Width / float64(total)
	paint := m.NewPaint()
	paint.SetColor(p.light)
	canvas.DrawRect(m.NewRectFromLTWH(0, 0, size.Width, size.Height), paint)
	paint.SetColor(p.dark)
	for y := range total {
		for x := 0; x < total; x++ {
			if !p.matrix.DarkQuiet(x, y) {
				continue
			}
			run := 1
			for x+run < total && p.matrix.DarkQuiet(x+run, y) {
				run++
			}
			canvas.DrawRect(m.NewRectFromLTWH(float64(x)*module, float64(y)*module, float64(run)*module, module), paint)
			x += run - 1
		}
	}
}

// ShouldRepaint is true when the other painter draws another matrix.
func (p *qrPainter) ShouldRepaint(old m.CustomPainter) bool {
	o, ok := old.(*qrPainter)
	return !ok || o.matrix.String() != p.matrix.String()
}

// qrCode is the widget of a QR matrix: a square of whole-pixel modules, at
// least minSide wide, with the quiet zone in white.
func qrCode(key string, matrix qrcode.Matrix, minSide float64) m.Widget {
	side := qrModuleSize(matrix.Total(), minSide) * float64(matrix.Total())
	return m.NewSizedBox(
		m.NewCustomPaint(nil, m.CustomPaintOpts{Painter: newQRPainter(matrix), Size: m.Size{Width: side, Height: side}}),
		m.SizedBoxOpts{Key: m.NewValueKey(key), Width: m.Some(side), Height: m.Some(side)},
	)
}

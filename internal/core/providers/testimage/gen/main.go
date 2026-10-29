// Command gen draws sum.png, the test image of the provider test (SPEC
// §11.6): "2 + 3 = 5" in a handwriting-like style, black on white, 600x200.
//
// Run it from the module root:
//
//	go run ./internal/core/providers/testimage/gen
//
// It writes internal/core/providers/testimage/sum.png (or the path given as
// the only argument). The drawing is deterministic: the same program always
// makes the same file. The result is committed; the program is only run again
// to change the picture.
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"os"
)

const (
	width, height = 600, 200
	defaultOut    = "internal/core/providers/testimage/sum.png"
	penRadius     = 3.4
)

// pt is a point in the unit box of a glyph (x to the right, y down).
type pt struct{ x, y float64 }

// stroke is one pen stroke; smooth strokes are drawn through their points as a
// Catmull-Rom spline, the others as straight lines.
type stroke struct {
	pts    []pt
	smooth bool
}

// glyphs are the strokes of the characters, in a box 1 wide and 1.4 high.
var glyphs = map[rune][]stroke{
	'2': {{pts: []pt{{0.05, 0.3}, {0.2, 0.08}, {0.5, 0.02}, {0.8, 0.15}, {0.85, 0.4}, {0.6, 0.72}, {0.2, 1.08}, {0.04, 1.3}}, smooth: true},
		{pts: []pt{{0.04, 1.3}, {0.5, 1.28}, {0.95, 1.32}}}},
	'3': {{pts: []pt{{0.1, 0.22}, {0.4, 0.03}, {0.75, 0.12}, {0.8, 0.4}, {0.45, 0.64}, {0.85, 0.86}, {0.85, 1.14}, {0.5, 1.35}, {0.1, 1.2}}, smooth: true}},
	'5': {{pts: []pt{{0.85, 0.05}, {0.22, 0.07}, {0.14, 0.62}}},
		{pts: []pt{{0.14, 0.62}, {0.5, 0.55}, {0.85, 0.82}, {0.8, 1.15}, {0.45, 1.36}, {0.1, 1.22}}, smooth: true}},
	'+': {{pts: []pt{{0.1, 0.7}, {0.9, 0.68}}}, {pts: []pt{{0.5, 0.3}, {0.52, 1.1}}}},
	'=': {{pts: []pt{{0.08, 0.5}, {0.92, 0.48}}}, {pts: []pt{{0.1, 0.92}, {0.9, 0.95}}}},
}

// placement is where a character goes: left edge and baseline in pixels and the
// scale of its unit box.
type placement struct {
	r     rune
	x, y  float64
	scale float64
}

var layout = []placement{
	{'2', 40, 40, 100},
	{'+', 160, 55, 80},
	{'3', 265, 40, 100},
	{'=', 385, 55, 80},
	{'5', 490, 40, 100},
}

func main() {
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./internal/core/providers/testimage/gen [out.png]")
		os.Exit(2)
	}
	out := defaultOut
	if len(os.Args) == 2 {
		out = os.Args[1]
	}
	data, err := render()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

// render draws the picture and returns it as PNG bytes.
func render() ([]byte, error) {
	rng := rand.New(rand.NewSource(7))
	var segs []seg
	for _, p := range layout {
		angle := (rng.Float64() - 0.5) * 0.14 // up to about 4 degrees
		sin, cos := math.Sin(angle), math.Cos(angle)
		for _, s := range glyphs[p.r] {
			pts := s.pts
			if s.smooth {
				pts = spline(pts)
			}
			var prev pt
			for i, q := range pts {
				// Rotate around the glyph's centre, scale, add a small wobble.
				cx, cy := q.x-0.5, q.y-0.7
				rx, ry := cx*cos-cy*sin, cx*sin+cy*cos
				cur := pt{
					p.x + (rx+0.5)*p.scale + (rng.Float64()-0.5)*2,
					p.y + (ry+0.7)*p.scale + (rng.Float64()-0.5)*2,
				}
				if i > 0 {
					segs = append(segs, seg{prev, cur})
				}
				prev = cur
			}
		}
	}
	img := image.NewGray(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			d := math.MaxFloat64
			c := pt{float64(x) + 0.5, float64(y) + 0.5}
			for _, s := range segs {
				d = math.Min(d, s.dist(c))
			}
			cov := math.Max(0, math.Min(1, penRadius+0.5-d))
			img.SetGray(x, y, color.Gray{Y: uint8(math.Round(255 * (1 - cov)))})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// seg is a straight piece of a stroke, in pixels.
type seg struct{ a, b pt }

// dist is the distance from c to the segment.
func (s seg) dist(c pt) float64 {
	dx, dy := s.b.x-s.a.x, s.b.y-s.a.y
	l2 := dx*dx + dy*dy
	t := 0.0
	if l2 > 0 {
		t = math.Max(0, math.Min(1, ((c.x-s.a.x)*dx+(c.y-s.a.y)*dy)/l2))
	}
	return math.Hypot(c.x-(s.a.x+t*dx), c.y-(s.a.y+t*dy))
}

// spline samples the Catmull-Rom spline through pts, 12 points per piece.
func spline(pts []pt) []pt {
	at := func(i int) pt { return pts[max(0, min(len(pts)-1, i))] }
	var out []pt
	for i := 0; i < len(pts)-1; i++ {
		p0, p1, p2, p3 := at(i-1), at(i), at(i+1), at(i+2)
		for k := 0; k < 12; k++ {
			t := float64(k) / 12
			out = append(out, pt{cr(p0.x, p1.x, p2.x, p3.x, t), cr(p0.y, p1.y, p2.y, p3.y, t)})
		}
	}
	return append(out, pts[len(pts)-1])
}

// cr is one coordinate of the Catmull-Rom spline between p1 and p2.
func cr(p0, p1, p2, p3, t float64) float64 {
	return 0.5 * (2*p1 + (-p0+p2)*t + (2*p0-5*p1+4*p2-p3)*t*t + (-p0+3*p1-3*p2+p3)*t*t*t)
}

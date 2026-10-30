// Command icongen draws the icon of Radio Tutor: a friendly cassette on a blue
// rounded square, 256 by 256 pixels, as a PNG. It uses only the standard library
// (image, image/draw, image/png): every shape is a signed distance function, which
// gives smooth edges without any anti-aliasing library.
//
// The committed result is internal/gui/app/icon.png, which the app embeds (the
// window icon on Linux) and scripts/build-windows.sh gives to glutter-windows (the
// icon of the .exe). To redraw it after a change here:
//
//	go run ./scripts/icongen                       writes internal/gui/app/icon.png
//	go run ./scripts/icongen -o other.png          writes another file
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"

	"github.com/rm4n0s/errors"
)

// size is the side of the icon in pixels.
const size = 256

// shape is a filled shape with a color: dist returns the signed distance of a
// point to its edge (negative inside), color gives the color at a point.
type shape struct {
	dist  func(x, y float64) float64
	color func(x, y float64) color.NRGBA
}

// solid is a color that does not depend on the point.
func solid(c color.NRGBA) func(x, y float64) color.NRGBA {
	return func(float64, float64) color.NRGBA { return c }
}

// roundRect is the distance function of a rounded rectangle with the corners
// (x0,y0) and (x1,y1) and the corner radius r.
func roundRect(x0, y0, x1, y1, r float64) func(x, y float64) float64 {
	cx, cy := (x0+x1)/2, (y0+y1)/2
	hx, hy := (x1-x0)/2-r, (y1-y0)/2-r
	return func(x, y float64) float64 {
		qx := math.Abs(x-cx) - hx
		qy := math.Abs(y-cy) - hy
		return math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - r
	}
}

// circle is the distance function of a circle.
func circle(cx, cy, r float64) func(x, y float64) float64 {
	return func(x, y float64) float64 { return math.Hypot(x-cx, y-cy) - r }
}

// ring is the distance function of a ring between the radii r0 and r1.
func ring(cx, cy, r0, r1 float64) func(x, y float64) float64 {
	return func(x, y float64) float64 {
		d := math.Hypot(x-cx, y-cy)
		return math.Max(r0-d, d-r1)
	}
}

// spoke is the distance function of a bar of the given half width that runs from
// the center of a reel outward at the angle a, between the radii r0 and r1.
func spoke(cx, cy, a, r0, r1, half float64) func(x, y float64) float64 {
	cs, sn := math.Cos(a), math.Sin(a)
	return func(x, y float64) float64 {
		dx, dy := x-cx, y-cy
		along := dx*cs + dy*sn
		across := -dx*sn + dy*cs
		qx := math.Max(r0-along, along-r1)
		qy := math.Abs(across) - half
		return math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0)
	}
}

// gradient is a vertical gradient from top to bottom over the icon's height.
func gradient(top, bottom color.NRGBA) func(x, y float64) color.NRGBA {
	return func(_, y float64) color.NRGBA {
		t := math.Min(math.Max(y/size, 0), 1)
		mix := func(a, b uint8) uint8 { return uint8(math.Round(float64(a)*(1-t) + float64(b)*t)) }
		return color.NRGBA{mix(top.R, bottom.R), mix(top.G, bottom.G), mix(top.B, bottom.B), 255}
	}
}

// shapes lists the shapes of the icon from the back to the front.
func shapes() []shape {
	var (
		cream  = color.NRGBA{255, 248, 231, 255}
		yellow = color.NRGBA{255, 209, 102, 255}
		red    = color.NRGBA{239, 83, 80, 255}
		dark   = color.NRGBA{40, 52, 86, 255}
		white  = color.NRGBA{255, 255, 255, 255}
		shadow = color.NRGBA{15, 30, 90, 70}
		grey   = color.NRGBA{120, 130, 150, 255}
	)
	list := []shape{
		{roundRect(0, 0, size, size, 56), gradient(color.NRGBA{79, 142, 247, 255}, color.NRGBA{47, 91, 211, 255})},
		{roundRect(36, 74, 220, 198, 16), solid(shadow)},
		{roundRect(36, 64, 220, 188, 16), solid(cream)},
		{roundRect(50, 76, 206, 122, 9), solid(yellow)},
		{roundRect(50, 100, 206, 108, 0), solid(red)},
		{roundRect(80, 130, 176, 176, 23), solid(dark)},
	}
	for _, cx := range []float64{105, 151} {
		list = append(list,
			shape{circle(cx, 153, 15), solid(white)},
			shape{circle(cx, 153, 6), solid(dark)},
		)
		for i := 0; i < 6; i++ {
			list = append(list, shape{spoke(cx, 153, float64(i)*math.Pi/3, 5, 12, 1.6), solid(white)})
		}
		list = append(list, shape{ring(cx, 153, 5, 7), solid(white)})
	}
	for _, c := range [][2]float64{{46, 74}, {210, 74}, {46, 178}, {210, 178}} {
		list = append(list, shape{circle(c[0], c[1], 3.2), solid(grey)})
	}
	return list
}

// over composites the color c with the coverage a (0 to 1) over the pixel p
// (straight, not premultiplied, alpha).
func over(p color.NRGBA, c color.NRGBA, a float64) color.NRGBA {
	sa := float64(c.A) / 255 * a
	da := float64(p.A) / 255
	oa := sa + da*(1-sa)
	if oa == 0 {
		return color.NRGBA{}
	}
	mix := func(s, d uint8) uint8 {
		return uint8(math.Round((float64(s)*sa + float64(d)*da*(1-sa)) / oa))
	}
	return color.NRGBA{mix(c.R, p.R), mix(c.G, p.G), mix(c.B, p.B), uint8(math.Round(oa * 255))}
}

// draw returns the icon.
func draw() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	list := shapes()
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			x, y := float64(px)+0.5, float64(py)+0.5
			var p color.NRGBA
			for _, s := range list {
				cov := math.Min(math.Max(0.5-s.dist(x, y), 0), 1)
				if cov > 0 {
					p = over(p, s.color(x, y), cov)
				}
			}
			img.SetNRGBA(px, py, p)
		}
	}
	return img
}

// encode returns the PNG of the icon.
func encode() ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, draw()); err != nil {
		return nil, errors.NewErr("IconGenEncode", err)
	}
	return buf.Bytes(), nil
}

// run writes the icon to the file out.
func run(out string) error {
	data, err := encode()
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return errors.NewErr("IconGenWrite", err).SetMetadata("path", out)
	}
	return nil
}

func main() {
	out := flag.String("o", "internal/gui/app/icon.png", "the PNG to write")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "icongen:", err)
		os.Exit(1)
	}
}

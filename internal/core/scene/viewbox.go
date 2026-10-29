package scene

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// The aspect ratio limits of a scene's view box (CASSETTE-FORMAT §5.1) and the
// aspect ratio of a cover (§4).
const (
	// MinAspect is the narrowest scene: 4:3.
	MinAspect = 4.0 / 3.0
	// MaxAspect is the widest scene: 21:9.
	MaxAspect = 21.0 / 9.0
	// CoverAspect is the aspect ratio (width over height) a cover should have:
	// 3:4.
	CoverAspect = 3.0 / 4.0
)

// aspectTolerance is the relative slack when comparing an aspect ratio with a
// limit, so that 2100x900 counts as exactly 21:9.
const aspectTolerance = 1e-9

// coverTolerance is the relative slack for the cover's 3:4 (a warning only).
const coverTolerance = 0.01

// ViewBox is the root element's viewBox: the coordinate system of the scene,
// which the timeline's origins and reveal boxes use.
type ViewBox struct {
	// X and Y are the top-left corner.
	X, Y float64
	// Width and Height are the size; both are greater than 0.
	Width, Height float64
}

// Aspect returns the width divided by the height.
func (v ViewBox) Aspect() float64 {
	return v.Width / v.Height
}

// numberPattern is an SVG number: an optional sign, digits with an optional
// fraction, an optional exponent. It is stricter than strconv.ParseFloat,
// which also takes "inf", "nan" and hexadecimal floats.
var numberPattern = regexp.MustCompile(`^[+-]?([0-9]+\.?[0-9]*|\.[0-9]+)([eE][+-]?[0-9]+)?$`)

// parseViewBox reads "min-x min-y width height" (numbers separated by commas
// and/or white space). On failure it returns a message for the author.
func parseViewBox(s string) (ViewBox, string) {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f'
	})
	if len(fields) != 4 {
		return ViewBox{}, "the viewBox must have four numbers, \"min-x min-y width height\", such as \"0 0 1600 900\""
	}
	var n [4]float64
	for i, f := range fields {
		if !numberPattern.MatchString(f) {
			return ViewBox{}, "the viewBox has \"" + f + "\", which is not a number"
		}
		v, err := strconv.ParseFloat(f, 64)
		if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
			return ViewBox{}, "the viewBox has \"" + f + "\", which is not a usable number"
		}
		n[i] = v
	}
	if n[2] <= 0 || n[3] <= 0 {
		return ViewBox{}, "the viewBox width and height must be greater than 0"
	}
	return ViewBox{X: n[0], Y: n[1], Width: n[2], Height: n[3]}, ""
}

// aspectInRange reports whether a view box's aspect ratio is within the scene
// limits.
func aspectInRange(a float64) bool {
	return a >= MinAspect*(1-aspectTolerance) && a <= MaxAspect*(1+aspectTolerance)
}

// aspectIsCover reports whether a is 3:4 up to a small tolerance.
func aspectIsCover(a float64) bool {
	return math.Abs(a-CoverAspect) <= CoverAspect*coverTolerance
}

// formatNumber formats a number for a message without trailing zeros.
func formatNumber(v float64) string {
	return strconv.FormatFloat(v, 'g', 10, 64)
}

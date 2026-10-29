// Package qrcode encodes text as a QR code and exposes the result as a module
// matrix that the GUI draws itself (crisp squares at any size). The encoding is
// done by rsc.io/qr; none of its types appear in this package's API.
package qrcode

import (
	"strconv"
	"strings"

	"github.com/rm4n0s/errors"
	"rsc.io/qr"
)

// QuietZone is the width, in modules, of the light margin the QR standard
// requires around a code. Matrix.Dark never includes it: the caller draws the
// background QuietZone modules wider on every side (see Matrix.Total).
const QuietZone = 4

// Level is the error correction level of a QR code. A higher level survives
// more damage and makes the code denser.
type Level int

const (
	// LevelL recovers about 7% of damaged modules.
	LevelL Level = iota
	// LevelM recovers about 15% of damaged modules. It is the default for the
	// phone URL.
	LevelM
	// LevelQ recovers about 25% of damaged modules.
	LevelQ
	// LevelH recovers about 30% of damaged modules.
	LevelH
)

// DefaultLevel is the level used for the phone URL.
const DefaultLevel = LevelM

// String returns "L", "M", "Q" or "H", or "Level(n)" for an invalid level.
func (l Level) String() string {
	switch l {
	case LevelL:
		return "L"
	case LevelM:
		return "M"
	case LevelQ:
		return "Q"
	case LevelH:
		return "H"
	}
	return "Level(" + strconv.Itoa(int(l)) + ")"
}

// Matrix is a square grid of QR modules without the quiet zone. The zero
// Matrix is empty (Size 0).
type Matrix struct {
	// Size is the number of modules on each side, without the quiet zone.
	Size int
	dark []bool // row-major, Size*Size
}

// Dark reports whether the module at column x, row y is dark. (0, 0) is the
// top-left module. Coordinates outside the matrix, which includes the quiet
// zone, are light.
func (m Matrix) Dark(x, y int) bool {
	if x < 0 || y < 0 || x >= m.Size || y >= m.Size {
		return false
	}
	return m.dark[y*m.Size+x]
}

// Total returns the number of modules on each side including the quiet zone.
func (m Matrix) Total() int {
	return m.Size + 2*QuietZone
}

// DarkQuiet is like Dark but addresses the matrix with the quiet zone
// included: (0, 0) is the top-left module of the quiet zone, and the valid
// range is 0 to Total()-1.
func (m Matrix) DarkQuiet(x, y int) bool {
	return m.Dark(x-QuietZone, y-QuietZone)
}

// String renders the modules without the quiet zone, one line per row, with
// '#' for a dark module and '.' for a light one. Every line ends in a newline.
func (m Matrix) String() string {
	var b strings.Builder
	b.Grow((m.Size + 1) * m.Size)
	for y := 0; y < m.Size; y++ {
		for x := 0; x < m.Size; x++ {
			if m.Dark(x, y) {
				b.WriteByte('#')
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// Encode encodes text as a QR code at the given error correction level. The
// smallest QR version that holds the text is used.
func Encode(text string, level Level) (Matrix, error) {
	if text == "" {
		return Matrix{}, errors.New("QRTextEmpty", "the text to encode is empty")
	}
	var lib qr.Level
	switch level {
	case LevelL:
		lib = qr.L
	case LevelM:
		lib = qr.M
	case LevelQ:
		lib = qr.Q
	case LevelH:
		lib = qr.H
	default:
		return Matrix{}, errors.New("QRLevelInvalid", "unknown error correction level", "level", int(level))
	}
	code, err := qr.Encode(text, lib)
	if err != nil {
		return Matrix{}, errors.NewErr("QRTextTooLong", err).SetMetadata("length", len(text), "level", level.String())
	}
	m := Matrix{Size: code.Size, dark: make([]bool, code.Size*code.Size)}
	for y := 0; y < code.Size; y++ {
		for x := 0; x < code.Size; x++ {
			m.dark[y*code.Size+x] = code.Black(x, y)
		}
	}
	return m, nil
}

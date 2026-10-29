// Package testimage holds the picture the provider test sends (SPEC §11.6):
// "2 + 3 = 5" written in a handwriting-like style, black on white, 600x200.
//
// The picture is drawn by a small program and committed as sum.png. To change
// it, edit gen/main.go and run, from the module root:
//
//	go run ./internal/core/providers/testimage/gen
package testimage

import (
	"bytes"
	_ "embed"
)

//go:embed sum.png
var sumPNG []byte

// SumPNG returns the PNG bytes of the test image. The caller may change the
// returned slice.
func SumPNG() []byte { return bytes.Clone(sumPNG) }

package main

import (
	"bytes"
	"os"
	"testing"
)

// TestRenderMatchesCommittedImage checks that sum.png is what the program
// makes, so the picture and its source cannot drift apart.
func TestRenderMatchesCommittedImage(t *testing.T) {
	want, err := os.ReadFile("../sum.png")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // the drawing is deterministic
		got, err := render()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatal("sum.png differs from what gen makes; run go run ./internal/core/providers/testimage/gen")
		}
	}
}

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// examplesGlob matches the packed example cassettes under examples/cassettes.
// Each one is made from a folder of the same name (without the extension) with
// the pack command; the sources of its narration live next to it in
// <name>-src and are not part of the cassette.
const examplesGlob = "../../examples/cassettes/*.cassette"

// TestExamplesValidate validates every packed example cassette with the command
// (which also draws every picture with Glutter's SVG renderer): no error and no
// warning. A new example is picked up as soon as its .cassette file is added.
func TestExamplesValidate(t *testing.T) {
	packs, err := filepath.Glob(examplesGlob)
	if err != nil {
		t.Fatal(err)
	}
	if len(packs) == 0 {
		t.Skip("no example cassettes under examples/cassettes yet")
	}
	for _, pack := range packs {
		t.Run(strings.TrimSuffix(filepath.Base(pack), ".cassette"), func(t *testing.T) {
			code, stdout, stderr := runCmd(t, "validate", pack)
			if code != 0 || stdout != "" {
				t.Errorf("validate %s: exit code %d, problems %q, stderr %q", pack, code, stdout, stderr)
			}
		})
	}
}

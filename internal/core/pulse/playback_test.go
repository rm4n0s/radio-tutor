// Copied from github.com/jfreymuth/pulse v0.1.3 and changed for Radio Tutor. MIT License, Copyright (c) 2019 Johann Freymuth: see internal/core/pulse/LICENSE.

package pulse

import "testing"

func TestEnsurePlaybackBuffersGrowsForServerRequest(t *testing.T) {
	front := make([]byte, 4080)
	back := make([]byte, 4080)

	front, back = ensurePlaybackBuffers(front, back, 4112)

	if cap(front) < 4112 || cap(back) < 4112 {
		t.Fatalf("buffer capacities = (%d, %d), want both at least 4112", cap(front), cap(back))
	}
}

func TestEnsurePlaybackBuffersRetainsSufficientBuffers(t *testing.T) {
	front := make([]byte, 4096)
	back := make([]byte, 4096)

	gotFront, gotBack := ensurePlaybackBuffers(front, back, 2048)

	if &gotFront[0] != &front[0] || &gotBack[0] != &back[0] {
		t.Fatal("sufficient buffers were unexpectedly replaced")
	}
}

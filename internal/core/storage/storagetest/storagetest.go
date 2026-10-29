// Package storagetest helps tests keep away from the real Radio Tutor folders
// (SPEC §16).
package storagetest

import (
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/storage"
)

// Isolate points XDG_CONFIG_HOME, XDG_DATA_HOME, XDG_STATE_HOME and
// XDG_CACHE_HOME at fresh folders made with t.TempDir (so XDG_* never reaches the
// user's real folders; the same four variables scripts/isolated.sh sets), and
// returns the Paths that storage.Resolve now gives. The variables are restored
// and the folders removed when the test ends.
//
// Every test that touches storage, directly or through another package, calls
// Isolate first. It uses t.Setenv, so the test (and its parents) must not be
// parallel. The folders of the returned Paths are not created; storage creates
// them on first use.
func Isolate(t testing.TB) storage.Paths {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	paths, err := storage.Resolve()
	if err != nil {
		t.Fatalf("storagetest.Isolate: %v", err)
	}
	return paths
}

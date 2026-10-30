// Package storagetest helps tests keep away from the real Radio Tutor folders
// (SPEC §16).
package storagetest

import (
	"runtime"
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/storage"
)

// Isolate points the variables that locate the user's folders at fresh folders
// made with t.TempDir, so that no test reaches the real ones, and returns the
// Paths that storage.Resolve now gives. On Linux the variables are
// XDG_CONFIG_HOME, XDG_DATA_HOME, XDG_STATE_HOME and XDG_CACHE_HOME (the same
// four scripts/isolated.sh sets); on Windows they are APPDATA, LOCALAPPDATA,
// USERPROFILE and HOME, and XDG_* too, which nothing reads there. The variables
// are restored and the folders removed when the test ends.
//
// Every test that touches storage, directly or through another package, calls
// Isolate first. It uses t.Setenv, so the test (and its parents) must not be
// parallel. The folders of the returned Paths are not created; storage creates
// them on first use.
func Isolate(t testing.TB) storage.Paths {
	t.Helper()
	isolateEnv(t)
	paths, err := storage.Resolve()
	if err != nil {
		t.Fatalf("storagetest.Isolate: %v", err)
	}
	return paths
}

// UnixModes reports whether the file system keeps Unix permission bits: false on
// Windows, where os.Stat reports a mode that does not change with os.Chmod
// (beyond the read-only bit) and a folder can be written whatever its mode is.
const UnixModes = runtime.GOOS != "windows"

// SkipWithoutUnixModes skips the test on a system where permission bits do not
// protect files (Windows), saying why. Windows protects a file with an ACL, which
// the test of the code that sets one reads back.
func SkipWithoutUnixModes(t testing.TB) {
	t.Helper()
	if !UnixModes {
		t.Skip("Windows keeps no Unix permission bits: a mode neither shows nor blocks anything there")
	}
}

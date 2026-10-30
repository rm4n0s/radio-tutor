//go:build !windows

package storagetest

import "testing"

// isolateVars are the variables isolateEnv points at fresh folders.
var isolateVars = []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"}

// isolateEnv points the XDG base folders at fresh folders.
func isolateEnv(t testing.TB) {
	t.Helper()
	for _, name := range isolateVars {
		t.Setenv(name, t.TempDir())
	}
}

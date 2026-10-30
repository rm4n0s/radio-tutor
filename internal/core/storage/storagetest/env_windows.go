//go:build windows

package storagetest

import "testing"

// isolateVars are the variables isolateEnv points at fresh folders: the known
// folders of Windows that os.UserConfigDir, os.UserCacheDir and
// storage.Resolve read, the home folder, and the XDG variables for the code that
// follows them anyway.
var isolateVars = []string{"LOCALAPPDATA", "APPDATA", "USERPROFILE", "HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"}

// isolateEnv points each of the variables at a fresh folder of its own.
func isolateEnv(t testing.TB) {
	t.Helper()
	for _, name := range isolateVars {
		t.Setenv(name, t.TempDir())
	}
}

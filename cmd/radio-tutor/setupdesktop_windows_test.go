//go:build windows

package main

import (
	"path/filepath"
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/menuentry"
)

// testMenuEnv returns an Env that runs as exe and makes the shortcut in a throw-away folder, not
// in the Start menu.
func testMenuEnv(t *testing.T, exe string) menuentry.Env {
	t.Helper()
	programs := filepath.Join(t.TempDir(), "Programs")
	env := menuentry.DefaultEnv()
	env.Executable = func() (string, error) { return exe, nil }
	env.Platform.ProgramsDir = func() (string, error) { return programs, nil }
	return env
}

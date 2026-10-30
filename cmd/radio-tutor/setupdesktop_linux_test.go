//go:build !windows

package main

import (
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/menuentry"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
)

// testMenuEnv returns an Env that runs as exe and writes under the isolated XDG_DATA_HOME.
func testMenuEnv(t *testing.T, exe string) menuentry.Env {
	t.Helper()
	storagetest.Isolate(t)
	env := menuentry.DefaultEnv()
	env.Executable = func() (string, error) { return exe, nil }
	return env
}

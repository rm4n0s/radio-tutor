package storagetest_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/storage"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
)

func TestIsolate(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/real") // left alone
	paths := storagetest.Isolate(t)

	got, err := storage.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != paths {
		t.Errorf("Resolve = %+v, Isolate returned %+v", got, paths)
	}

	seen := map[string]string{}
	names := []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"}
	if runtime.GOOS == "windows" {
		names = append(names, "LOCALAPPDATA", "APPDATA", "USERPROFILE", "HOME")
	}
	for _, name := range names {
		dir := os.Getenv(name)
		if dir == "" || !strings.HasPrefix(dir, os.TempDir()) {
			t.Errorf("%s = %q, want a folder under %s", name, dir, os.TempDir())
		}
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Errorf("%s = %q is not an existing folder (%v)", name, dir, err)
		}
		if other, dup := seen[dir]; dup {
			t.Errorf("%s and %s share %q", name, other, dir)
		}
		seen[dir] = name
	}
	for _, dir := range []string{paths.Config, paths.Data, paths.State} {
		app := dir
		if runtime.GOOS == "windows" {
			app = filepath.Dir(dir) // ...\radio-tutor\config
		}
		if !strings.HasPrefix(dir, os.TempDir()) || filepath.Base(app) != "radio-tutor" {
			t.Errorf("path %q is not a radio-tutor folder under the temp folder", dir)
		}
	}
}

func TestIsolateGivesEachTestFreshFolders(t *testing.T) {
	var first storage.Paths
	t.Run("first", func(t *testing.T) { first = storagetest.Isolate(t) })
	t.Run("second", func(t *testing.T) {
		if second := storagetest.Isolate(t); second == first {
			t.Errorf("both tests got %+v", first)
		}
	})
}

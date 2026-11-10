//go:build darwin

package menuentry

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func darwinEnv(t *testing.T, home string) Env {
	t.Helper()
	exe := testProgram(t)
	return Env{
		Executable: func() (string, error) { return exe, nil },
		Platform:   PlatformEnv{Getenv: func(k string) string { return map[string]string{"HOME": home}[k] }},
	}
}

func TestInstallMakesTheBundleAndRemoveTakesItAway(t *testing.T) {
	home := t.TempDir()
	env := darwinEnv(t, home)
	e := testEntry(t)
	written, err := Install(context.Background(), e, env)
	if err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(home, "Applications", "Example App.app")
	want := []string{
		filepath.Join(app, "Contents", "Resources", "dev.example.app.icns"),
		filepath.Join(app, "Contents", "Info.plist"),
		filepath.Join(app, "Contents", "MacOS", "dev.example.app"),
	}
	if strings.Join(written, "\n") != strings.Join(want, "\n") {
		t.Fatalf("written %v, want %v", written, want)
	}
	script, err := os.ReadFile(want[2])
	if err != nil {
		t.Fatal(err)
	}
	exe, _ := env.Executable()
	resolved, _ := filepath.EvalSymlinks(exe)
	if !strings.HasPrefix(string(script), "#!/bin/sh\n") || !strings.Contains(string(script), "exec '"+resolved+"' \"$@\"\n") || !strings.Contains(string(script), "AppleLocale") {
		t.Errorf("launcher %q does not run %s", script, resolved)
	}
	if st, err := os.Stat(want[2]); err != nil || st.Mode().Perm()&0o111 == 0 {
		t.Errorf("the launcher is not executable: %v, %v", st, err)
	}
	plist, err := os.ReadFile(want[1])
	if err != nil || !strings.Contains(string(plist), "<string>dev.example.app</string>") {
		t.Errorf("Info.plist %q (%v)", plist, err)
	}
	icon, err := os.ReadFile(want[0])
	if err != nil || string(icon[:4]) != "icns" || binary.BigEndian.Uint32(icon[4:8]) != uint32(len(icon)) {
		t.Fatalf("icns %v", err)
	}
	var kinds []string
	for rest := icon[8:]; len(rest) > 0; {
		kinds = append(kinds, string(rest[:4]))
		rest = rest[binary.BigEndian.Uint32(rest[4:8]):]
	}
	if strings.Join(kinds, ",") != "icp6,ic07,ic08" {
		t.Errorf("chunks %v", kinds)
	}

	removed, err := Remove(context.Background(), e, env)
	if err != nil || strings.Join(removed, "\n") != strings.Join(want, "\n") {
		t.Fatalf("removed %v, %v", removed, err)
	}
	if _, err := os.Stat(app); !os.IsNotExist(err) {
		t.Errorf("the bundle is still there: %v", err)
	}
	removed, err = Remove(context.Background(), e, env)
	if err != nil || len(removed) != 0 {
		t.Errorf("a second Remove: %v, %v", removed, err)
	}
}

func TestApplicationsDirNeedsAnAbsoluteHome(t *testing.T) {
	env := Env{Executable: os.Executable, Platform: PlatformEnv{Getenv: func(string) string { return "relative" }}}
	_, err := Remove(context.Background(), testEntry(t), env)
	hasRoute(t, err, "Remove->remove->applicationsDir.ApplicationsDirUnknown")
}

func TestRemoveFailures(t *testing.T) {
	home := t.TempDir()
	env := darwinEnv(t, home)
	e := testEntry(t)
	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := Remove(ctx, e, env)
		hasRoute(t, err, "Remove->remove.RemoveCancelled")
	})
	t.Run("a file cannot be removed", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root ignores the permissions")
		}
		h := t.TempDir()
		macos := filepath.Join(h, "Applications", "Example App.app", "Contents", "MacOS")
		if err := os.MkdirAll(macos, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(macos, "dev.example.app"), []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(macos, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(macos, 0o700) })
		_, err := Remove(context.Background(), e, darwinEnv(t, h))
		hasRoute(t, err, "Remove->remove.EntryFileRemoveFailed")
	})
}

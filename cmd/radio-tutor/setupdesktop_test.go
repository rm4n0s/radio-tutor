package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// testMenuEnv returns an Env that runs as exe and writes to throw-away folders; it is defined per
// operating system (setupdesktop_linux_test.go, setupdesktop_windows_test.go) so that no test
// touches the real menu.

// testProgramFiles makes the running program and, next to it, the GUI program (Windows needs it).
func testProgramFiles(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "radio-tutor-cli.exe")
	for _, name := range []string{exe, filepath.Join(dir, "radio-tutor.exe")} {
		if err := os.WriteFile(name, []byte("program"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return exe
}

func TestMenuEntry(t *testing.T) {
	e := menuEntry()
	if e.ID != app.ID || e.Name != app.Title {
		t.Errorf("entry %q %q, want %q %q", e.ID, e.Name, app.ID, app.Title)
	}
	if e.ID != "dev.radiotutor.app" {
		t.Errorf("the application ID is %q", e.ID)
	}
	if len(e.IconPNG) == 0 || e.Comment == "" || e.Comments["el"] == "" || e.Names["el"] == "" {
		t.Errorf("incomplete entry: %+v", e)
	}
	if e.GUIProgram != "radio-tutor.exe" {
		t.Errorf("GUI program %q", e.GUIProgram)
	}
}

func TestSetupDesktop(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "install", args: nil, wantCode: 0, wantStdout: "Added Radio Tutor to the application menu:\n"},
		{name: "install again", args: nil, wantCode: 0, wantStdout: "Added Radio Tutor to the application menu:\n"},
		{name: "remove", args: []string{"--remove"}, wantCode: 0, wantStdout: "Removed Radio Tutor from the application menu:\n"},
		{name: "remove again", args: []string{"--remove"}, wantCode: 0, wantStdout: "nothing to remove"},
		{name: "unknown flag", args: []string{"--delete"}, wantCode: 2, wantStderr: setupDesktopUsage},
		{name: "two arguments", args: []string{"--remove", "--remove"}, wantCode: 2, wantStderr: setupDesktopUsage},
	}
	env := testMenuEnv(t, testProgramFiles(t))
	var written []string
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := setupDesktop(context.Background(), tt.args, setupDeps{
				stdout: &printer{w: &stdout}, stderr: &printer{w: &stderr}, env: env})
			if code != tt.wantCode {
				t.Errorf("exit code %d, want %d (stderr %q)", code, tt.wantCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout %q does not contain %q", stdout.String(), tt.wantStdout)
			}
			if tt.wantStderr == "" && stderr.Len() != 0 {
				t.Errorf("unexpected stderr %q", stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr %q does not contain %q", stderr.String(), tt.wantStderr)
			}
			// The files named on stdout are the files that exist (after an install) or are gone (after a remove).
			var paths []string
			for _, line := range strings.Split(stdout.String(), "\n") {
				if strings.HasPrefix(line, "  ") {
					paths = append(paths, strings.TrimPrefix(line, "  "))
				}
			}
			install := len(tt.args) == 0
			if install && len(paths) == 0 {
				t.Fatal("an install printed no path")
			}
			if install {
				written = paths
			}
			for _, p := range paths {
				if _, err := os.Stat(p); (err == nil) != install {
					t.Errorf("%s: stat error %v after an install=%v", p, err, install)
				}
			}
			if len(tt.args) == 1 && tt.args[0] == "--remove" && len(paths) != 0 && len(paths) != len(written) {
				t.Errorf("removed %v, installed %v", paths, written)
			}
		})
	}
}

func TestSetupDesktopFailures(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		executable func() (string, error)
		wantStderr string
	}{
		{
			name:       "program of go run",
			executable: nil, // filled below
			wantStderr: "go install",
		},
		{
			name:       "no running program",
			executable: func() (string, error) { return "", os.ErrNotExist },
			wantStderr: "radio-tutor setup-desktop:",
		},
	}
	goRun := filepath.Join(t.TempDir(), "go-build42", "b001", "exe")
	if err := os.MkdirAll(goRun, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"radio-tutor-cli.exe", "radio-tutor.exe"} {
		if err := os.WriteFile(filepath.Join(goRun, name), []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	tests[0].executable = func() (string, error) { return filepath.Join(goRun, "radio-tutor-cli.exe"), nil }
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := testMenuEnv(t, "")
			env.Executable = tt.executable
			var stdout, stderr bytes.Buffer
			code := setupDesktop(context.Background(), tt.args, setupDeps{
				stdout: &printer{w: &stdout}, stderr: &printer{w: &stderr}, env: env})
			if code != exitError || stdout.Len() != 0 || !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("exit %d, stdout %q, stderr %q; want exit 1, no stdout, stderr with %q", code, stdout.String(), stderr.String(), tt.wantStderr)
			}
		})
	}
}

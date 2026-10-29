package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/archive"
)

// validDir is the valid cassette of the archive package's tests.
const validDir = "../../internal/core/archive/testdata/valid"

// copyValid copies the valid cassette into a new folder and returns it.
func copyValid(t *testing.T) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "cassette")
	root := os.DirFS(validDir)
	err := fs.WalkDir(root, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(name))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(root, name)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

// edit rewrites the file name of the cassette folder dir with f.
func edit(t *testing.T, dir, name string, f func(string) string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(f(string(data))), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runCmd runs the command line and returns the exit code and the outputs.
func runCmd(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(context.Background(), args, strings.NewReader(""), &out, &errOut, nil)
	return code, out.String(), errOut.String()
}

const (
	scene1 = "lessons/what-is-a-fraction/scene.svg"
	// A nested viewBox with a negative size: the core's scene rules accept it,
	// Glutter refuses it.
	negativeViewBox = `<g id="pizza"><svg width="9" height="9" viewBox="0 0 -9 9"><circle r="4"/></svg></g>`
)

func breakGlutter(s string) string {
	return strings.Replace(s, `<g id="pizza"><circle cx="800" cy="450" r="260" fill="url(#crust)"/></g>`, negativeViewBox, 1)
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name       string
		break_     func(t *testing.T, dir string)
		args       func(dir string) []string
		wantCode   int
		wantStdout []string // each must be a line of the output; nil: empty
		wantStderr string
	}{
		{
			name:       "valid folder",
			args:       func(dir string) []string { return []string{"validate", dir} },
			wantStderr: "is valid (0 errors, 0 warnings)",
		},
		{
			name:       "valid folder with --json before the path",
			args:       func(dir string) []string { return []string{"validate", "--json", dir} },
			wantStdout: []string{"[]"},
		},
		{
			name: "warning only is still valid",
			break_: func(t *testing.T, dir string) {
				edit(t, dir, "cover.svg", func(s string) string { return strings.Replace(s, "0 0 300 400", "0 0 400 400", 1) })
			},
			args:       func(dir string) []string { return []string{"validate", dir} },
			wantStdout: []string{"warning: cover.svg"},
			wantStderr: "is valid (0 errors, 1 warning)",
		},
		{
			name: "a missing file",
			break_: func(t *testing.T, dir string) {
				if err := os.Remove(filepath.Join(dir, "lessons/what-is-a-fraction/narration.ogg")); err != nil {
					t.Fatal(err)
				}
			},
			args:       func(dir string) []string { return []string{"validate", dir} },
			wantCode:   1,
			wantStdout: []string{"lessons/what-is-a-fraction/narration.ogg"},
			wantStderr: "is not valid (1 error, 0 warnings)",
		},
		{
			name: "an extra file",
			break_: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			args:       func(dir string) []string { return []string{"validate", dir} },
			wantCode:   1,
			wantStdout: []string{"notes.txt"},
		},
		{
			name: "an svg that the core accepts and Glutter cannot draw",
			break_: func(t *testing.T, dir string) {
				edit(t, dir, scene1, breakGlutter)
			},
			args:       func(dir string) []string { return []string{"validate", dir} },
			wantCode:   1,
			wantStdout: []string{"lessons/what-is-a-fraction/scene.svg#svg/g[@id=pizza]: Radio Tutor cannot draw it: "},
		},
		{
			name: "json of an error",
			break_: func(t *testing.T, dir string) {
				edit(t, dir, scene1, breakGlutter)
			},
			args:       func(dir string) []string { return []string{"validate", "--json", dir} },
			wantCode:   1,
			wantStdout: []string{"["},
		},
		{
			name:     "no path",
			args:     func(string) []string { return []string{"validate"} },
			wantCode: 2, wantStderr: validateUsage,
		},
		{
			name:     "two paths",
			args:     func(dir string) []string { return []string{"validate", dir, dir} },
			wantCode: 2, wantStderr: validateUsage,
		},
		{
			name:     "unknown flag",
			args:     func(dir string) []string { return []string{"validate", "--bogus", dir} },
			wantCode: 2, wantStderr: "flag provided but not defined: -bogus",
		},
		{
			name:       "help",
			args:       func(string) []string { return []string{"validate", "-h"} },
			wantStdout: []string{strings.TrimSuffix(validateUsage, "\n")},
		},
		{
			name:     "a path that does not exist",
			args:     func(dir string) []string { return []string{"validate", dir + "-missing"} },
			wantCode: 1, wantStderr: "radio-tutor validate: ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := copyValid(t)
			if tt.break_ != nil {
				tt.break_(t, dir)
			}
			code, stdout, stderr := runCmd(t, tt.args(dir)...)
			if code != tt.wantCode {
				t.Errorf("exit code %d, want %d (stdout %q, stderr %q)", code, tt.wantCode, stdout, stderr)
			}
			lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
			for _, want := range tt.wantStdout {
				found := false
				for _, l := range lines {
					if strings.HasPrefix(l, want) {
						found = true
					}
				}
				if !found {
					t.Errorf("stdout has no line starting with %q:\n%s", want, stdout)
				}
			}
			if tt.wantStdout == nil && stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tt.wantStderr) {
				t.Errorf("stderr %q does not contain %q", stderr, tt.wantStderr)
			}
		})
	}
}

func TestValidateJSON(t *testing.T) {
	dir := copyValid(t)
	edit(t, dir, scene1, breakGlutter)
	edit(t, dir, "cover.svg", func(s string) string { return strings.Replace(s, "0 0 300 400", "0 0 400 400", 1) })

	code, stdout, stderr := runCmd(t, "validate", "--json", dir)
	if code != 1 {
		t.Fatalf("exit code %d, want 1 (stderr %q)", code, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty in JSON mode", stderr)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if len(got) != 2 {
		t.Fatalf("got %d problems, want 2: %s", len(got), stdout)
	}
	var sawError, sawWarning bool
	for _, p := range got {
		path, _ := p["path"].(string)
		msg, _ := p["message"].(string)
		warn, ok := p["warning"].(bool)
		if path == "" || msg == "" || !ok {
			t.Errorf("problem %v lacks path, message or warning", p)
		}
		if warn {
			sawWarning = true
			if !strings.HasPrefix(path, "cover.svg") {
				t.Errorf("warning path = %q", path)
			}
		} else {
			sawError = true
			if !strings.HasPrefix(path, scene1) || !strings.Contains(msg, "cannot draw") {
				t.Errorf("error = %v", p)
			}
		}
	}
	if !sawError || !sawWarning {
		t.Errorf("error %v, warning %v in %s", sawError, sawWarning, stdout)
	}
}

func TestValidateText(t *testing.T) {
	dir := copyValid(t)
	edit(t, dir, "cover.svg", func(s string) string { return strings.Replace(s, "0 0 300 400", "0 0 400 400", 1) })
	code, stdout, _ := runCmd(t, "validate", dir)
	if code != 0 {
		t.Fatalf("exit code %d, want 0", code)
	}
	if !strings.HasPrefix(stdout, "warning: cover.svg#") || strings.Count(stdout, "\n") != 1 {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestPackThenValidateTheZip(t *testing.T) {
	dir := copyValid(t)
	out := filepath.Join(t.TempDir(), "fractions.cassette")

	code, stdout, stderr := runCmd(t, "pack", dir, out)
	if code != 0 {
		t.Fatalf("pack: exit code %d (stderr %q)", code, stderr)
	}
	if !strings.Contains(stdout, out) {
		t.Errorf("stdout = %q, want the output name", stdout)
	}
	first, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	if code, stdout, stderr := runCmd(t, "validate", out); code != 0 || stdout != "" {
		t.Errorf("validate of the zip: exit code %d, stdout %q, stderr %q", code, stdout, stderr)
	}

	// Packing again needs --force, and gives the same bytes.
	if code, _, stderr := runCmd(t, "pack", dir, out); code != 1 || !strings.Contains(stderr, "already exists") {
		t.Errorf("pack over an existing file: exit code %d, stderr %q", code, stderr)
	}
	if code, _, stderr := runCmd(t, "pack", "--force", dir, out); code != 0 {
		t.Errorf("pack --force: exit code %d, stderr %q", code, stderr)
	}
	second, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Error("packing the same folder twice gave different bytes")
	}
}

func TestPack(t *testing.T) {
	tests := []struct {
		name       string
		break_     func(t *testing.T, dir string)
		args       func(dir, out string) []string
		wantCode   int
		wantStderr string
		wantFile   bool
	}{
		{
			name:     "force before the folder",
			args:     func(dir, out string) []string { return []string{"pack", "--force", dir, out} },
			wantFile: true,
		},
		{
			name: "warnings do not stop it and are printed",
			break_: func(t *testing.T, dir string) {
				edit(t, dir, "cover.svg", func(s string) string { return strings.Replace(s, "0 0 300 400", "0 0 400 400", 1) })
			},
			args:       func(dir, out string) []string { return []string{"pack", dir, out} },
			wantStderr: "warning: cover.svg",
			wantFile:   true,
		},
		{
			name: "an svg that Glutter cannot draw is refused",
			break_: func(t *testing.T, dir string) {
				edit(t, dir, scene1, breakGlutter)
			},
			args:       func(dir, out string) []string { return []string{"pack", dir, out} },
			wantCode:   1,
			wantStderr: "Radio Tutor cannot draw it",
		},
		{
			name: "a folder with a problem is refused",
			break_: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			args:       func(dir, out string) []string { return []string{"pack", dir, out} },
			wantCode:   1,
			wantStderr: "notes.txt",
		},
		{
			name:     "no arguments",
			args:     func(string, string) []string { return []string{"pack"} },
			wantCode: 2, wantStderr: packUsage,
		},
		{
			name:     "one argument",
			args:     func(dir, _ string) []string { return []string{"pack", dir} },
			wantCode: 2, wantStderr: packUsage,
		},
		{
			name:     "three arguments",
			args:     func(dir, out string) []string { return []string{"pack", dir, out, out} },
			wantCode: 2, wantStderr: packUsage,
		},
		{
			name:     "unknown flag",
			args:     func(dir, out string) []string { return []string{"pack", "-x", dir, out} },
			wantCode: 2, wantStderr: "flag provided but not defined: -x",
		},
		{
			name:     "help",
			args:     func(string, string) []string { return []string{"pack", "--help"} },
			wantCode: 0,
		},
		{
			name:     "a folder that does not exist",
			args:     func(dir, out string) []string { return []string{"pack", dir + "-missing", out} },
			wantCode: 1, wantStderr: "radio-tutor pack: ",
		},
		{
			name:     "a zip is not a folder",
			args:     func(dir, out string) []string { return []string{"pack", out, out + "2"} },
			wantCode: 1, wantStderr: "radio-tutor pack: ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := copyValid(t)
			if tt.break_ != nil {
				tt.break_(t, dir)
			}
			out := filepath.Join(t.TempDir(), "out.cassette")
			if tt.name == "a zip is not a folder" {
				if err := os.WriteFile(out, []byte("not a zip"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			code, stdout, stderr := runCmd(t, tt.args(dir, out)...)
			if code != tt.wantCode {
				t.Errorf("exit code %d, want %d (stdout %q, stderr %q)", code, tt.wantCode, stdout, stderr)
			}
			if !strings.Contains(stderr, tt.wantStderr) {
				t.Errorf("stderr %q does not contain %q", stderr, tt.wantStderr)
			}
			if tt.name != "a zip is not a folder" {
				_, err := os.Stat(out)
				if tt.wantFile && err != nil {
					t.Errorf("the output file was not written: %v", err)
				}
				if !tt.wantFile && err == nil {
					t.Error("the output file was written")
				}
			}
			if tt.wantFile {
				rep, err := archive.ValidatePath(context.Background(), out, nil)
				if err != nil || !rep.Valid() {
					t.Errorf("the packed file is not valid: %v %v", err, rep.Problems)
				}
			}
		})
	}
}

// TestPackRefusalWritesNothing checks that a refusal leaves no file or
// temporary file in the output folder.
func TestPackRefusalWritesNothing(t *testing.T) {
	dir := copyValid(t)
	edit(t, dir, scene1, breakGlutter)
	outDir := t.TempDir()
	if code, _, _ := runCmd(t, "pack", dir, filepath.Join(outDir, "x.cassette")); code != 1 {
		t.Fatalf("exit code %d, want 1", code)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("the output folder has %d entries", len(entries))
	}
}

// failingWriter fails every write.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestWriteJSONFailure(t *testing.T) {
	err := writeJSON(failingWriter{}, nil)
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T", err)
	}
	if !appErr.HasRoute("writeJSON.ValidateJSONFailed") {
		t.Errorf("unexpected failure path: %s", appErr.Route())
	}
}

func TestValidateStdoutFailure(t *testing.T) {
	dir := copyValid(t)
	var errOut bytes.Buffer
	code := run(context.Background(), []string{"validate", "--json", dir}, strings.NewReader(""), failingWriter{}, &errOut, nil)
	if code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "radio-tutor validate: ") {
		t.Errorf("stderr = %q", errOut.String())
	}
}

func TestValidateCancelled(t *testing.T) {
	dir := copyValid(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errOut bytes.Buffer
	for _, args := range [][]string{{"validate", dir}, {"pack", dir, filepath.Join(t.TempDir(), "x.cassette")}} {
		out.Reset()
		errOut.Reset()
		if code := run(ctx, args, strings.NewReader(""), &out, &errOut, nil); code != 1 {
			t.Errorf("%v: exit code %d, want 1 (stderr %q)", args, code, errOut.String())
		}
	}
}

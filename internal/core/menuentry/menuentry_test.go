package menuentry

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
)

// testIcon is a square PNG of the given side, with a diagonal so that scaling it changes it.
func testIcon(t testing.TB, side int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, side, side))
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 255 / side), G: uint8(y * 255 / side), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// testEntry is an entry for the tests, with a program next to the "running" one.
func testEntry(t testing.TB) Entry {
	t.Helper()
	return Entry{
		ID:         "dev.example.app",
		Name:       "Example App",
		Names:      map[string]string{"el": "Παράδειγμα"},
		Comment:    "An example",
		Comments:   map[string]string{"el": "Ένα παράδειγμα"},
		Categories: []string{"Education"},
		IconPNG:    testIcon(t, 256),
		GUIProgram: "example-gui.exe",
	}
}

// testProgram makes a file that plays the running program (and next to it the GUI program) in a
// fresh folder and returns its resolved path.
func testProgram(t testing.TB) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "example-cli.exe")
	for _, name := range []string{exe, filepath.Join(dir, "example-gui.exe")} {
		if err := os.WriteFile(name, []byte("program"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return exe
}

// envFor returns an Env whose running program is exe, on top of the platform's isolated env.
func envFor(exe string) Env {
	env := DefaultEnv()
	env.Executable = func() (string, error) { return exe, nil }
	return env
}

func hasRoute(t *testing.T, err error, route string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error with the route %s", route)
	}
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T: %v", err, err)
	}
	if !appErr.HasRoute(route) {
		t.Errorf("unexpected failure path: %s (want %s)", appErr.Route(), route)
	}
}

func TestIsTemporaryBuild(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		gocache string
		want    bool
	}{
		{"installed program", "/home/u/go/bin/radio-tutor", "", false},
		{"go run", "/tmp/go-build1234567/b001/exe/radio-tutor", "", true},
		{"go test", "/var/tmp/go-build99/b001/radio-tutor.test", "", true},
		{"build cache", "/home/u/.cache/go-build/ab/abcdef-d", "", true},
		{"custom cache", "/data/gocache/ab/abcdef-d", "/data/gocache", true},
		{"cache name as prefix only", "/data/gocache2/radio-tutor", "/data/gocache", false},
		{"relative cache variable is ignored", "/data/gocache/x", "gocache", false},
		{"a folder that only contains the word", "/home/u/my-go-build/radio-tutor", "", false},
	}
	// On Windows an absolute path starts with a drive letter.
	abs := func(p string) string {
		if p == "" || !strings.HasPrefix(p, "/") {
			return p
		}
		return filepath.VolumeName(os.TempDir()) + filepath.FromSlash(p)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTemporaryBuild(abs(tt.path), abs(tt.gocache)); got != tt.want {
				t.Errorf("isTemporaryBuild(%q, %q) = %v, want %v", tt.path, tt.gocache, got, tt.want)
			}
		})
	}
}

func TestEntryValidate(t *testing.T) {
	tests := []struct {
		name   string
		change func(e *Entry)
	}{
		{"empty id", func(e *Entry) { e.ID = "" }},
		{"id with a slash", func(e *Entry) { e.ID = "a/b" }},
		{"id with a backslash", func(e *Entry) { e.ID = `a\b` }},
		{"id is dots", func(e *Entry) { e.ID = ".." }},
		{"id with spaces around", func(e *Entry) { e.ID = " a" }},
		{"empty name", func(e *Entry) { e.Name = "" }},
		{"name with a newline", func(e *Entry) { e.Name = "a\nb" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storagetest.Isolate(t)
			e := testEntry(t)
			tt.change(&e)
			_, err := Install(context.Background(), e, envFor(testProgram(t)))
			hasRoute(t, err, "Install->Entry.validate.EntryInvalid")
			_, err = Remove(context.Background(), e, envFor(testProgram(t)))
			hasRoute(t, err, "Remove->Entry.validate.EntryInvalid")
		})
	}
}

func TestInstallRefusals(t *testing.T) {
	tests := []struct {
		name       string
		executable func(t *testing.T) func() (string, error)
		route      string
	}{
		{
			name: "no running program",
			executable: func(*testing.T) func() (string, error) {
				return func() (string, error) { return "", os.ErrNotExist }
			},
			route: "Install->runningProgram.ExecutableUnknown",
		},
		{
			name: "program that is gone",
			executable: func(t *testing.T) func() (string, error) {
				p := filepath.Join(t.TempDir(), "gone")
				return func() (string, error) { return p, nil }
			},
			route: "Install->runningProgram.ExecutableResolveFailed",
		},
		{
			name: "go run program",
			executable: func(t *testing.T) func() (string, error) {
				dir := filepath.Join(t.TempDir(), "go-build123", "b001", "exe")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				p := filepath.Join(dir, "example-cli.exe")
				if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
					t.Fatal(err)
				}
				return func() (string, error) { return p, nil }
			},
			route: "Install->runningProgram.ExecutableTemporary",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := storagetest.Isolate(t)
			env := DefaultEnv()
			env.Executable = tt.executable(t)
			_, err := Install(context.Background(), testEntry(t), env)
			hasRoute(t, err, tt.route)
			if _, statErr := os.Stat(filepath.Dir(paths.Data)); statErr == nil {
				entries, _ := os.ReadDir(filepath.Dir(paths.Data))
				if len(entries) != 0 {
					t.Errorf("a refused install wrote %v", entries)
				}
			}
		})
	}
}

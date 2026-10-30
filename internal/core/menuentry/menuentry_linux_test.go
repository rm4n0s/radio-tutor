//go:build !windows

package menuentry

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
)

func TestExecLine(t *testing.T) {
	// Each row is a path, the value of the Exec key as it is written in the file (the
	// path in double quotes, ", `, $ and \ escaped by a backslash, % doubled, and then
	// every backslash doubled again by the file format).
	tests := []struct {
		name string
		path string
		want string
	}{
		{"plain", "/home/u/go/bin/radio-tutor", `Exec="/home/u/go/bin/radio-tutor"`},
		{"space", "/home/u/My Apps/radio-tutor", `Exec="/home/u/My Apps/radio-tutor"`},
		{"quote", `/opt/a"b/radio-tutor`, `Exec="/opt/a\\"b/radio-tutor"`},
		{"backslash", `/opt/a\b/radio-tutor`, `Exec="/opt/a\\\\b/radio-tutor"`},
		{"dollar", "/opt/$HOME/radio-tutor", `Exec="/opt/\\$HOME/radio-tutor"`},
		{"backtick", "/opt/a`b/radio-tutor", "Exec=\"/opt/a\\\\`b/radio-tutor\""},
		{"percent", "/opt/100%/radio-tutor", `Exec="/opt/100%%/radio-tutor"`},
		{"single quote", "/opt/it's/radio-tutor", `Exec="/opt/it's/radio-tutor"`},
		{"tab and newline", "/opt/a\tb\nc/radio-tutor", `Exec="/opt/a\tb\nc/radio-tutor"`},
		{"unicode", "/home/μάθημα/radio-tutor", `Exec="/home/μάθημα/radio-tutor"`},
		{"everything", "/a b\"c`d$e\\f%g/x", `Exec="/a b\\"c\\` + "`" + `d\\$e\\\\f%%g/x"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := "Exec=" + escapeValue(execLine(tt.path))
			if got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestDesktopEntryText(t *testing.T) {
	e := testEntry(t)
	got := desktopEntry(e, "/home/u/go/bin/radio-tutor")
	want := `[Desktop Entry]
Type=Application
Version=1.0
Name=Example App
Name[el]=Παράδειγμα
Comment=An example
Comment[el]=Ένα παράδειγμα
Exec="/home/u/go/bin/radio-tutor"
Icon=dev.example.app
Terminal=false
Categories=Education;
StartupWMClass=dev.example.app
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestInstallAndRemove(t *testing.T) {
	paths := storagetest.Isolate(t)
	dataHome := filepath.Dir(paths.Data) // $XDG_DATA_HOME
	exe := testProgram(t)
	e := testEntry(t)
	env := envFor(exe)

	icons := []string{}
	for _, n := range []string{"48", "64", "128", "256"} {
		icons = append(icons, filepath.Join(dataHome, "icons", "hicolor", n+"x"+n, "apps", "dev.example.app.png"))
	}
	desktop := filepath.Join(dataHome, "applications", "dev.example.app.desktop")
	want := append(append([]string{}, icons...), desktop)

	for run := 1; run <= 2; run++ { // the second run replaces the files
		got, err := Install(context.Background(), e, env)
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: wrote %v, want %v", run, got, want)
		}
	}

	for i, size := range []int{48, 64, 128, 256} {
		data, err := os.ReadFile(icons[i])
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s is not a PNG: %v", icons[i], err)
		}
		if b := img.Bounds(); b.Dx() != size || b.Dy() != size {
			t.Errorf("%s is %v, want %dx%d", icons[i], b, size, size)
		}
	}
	text, err := os.ReadFile(desktop)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), `Exec="`+exe+`"`) {
		t.Errorf("the entry does not start %s:\n%s", exe, text)
	}
	for _, p := range append(append([]string{}, want...), filepath.Dir(desktop), filepath.Dir(icons[0])) {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if st.IsDir() {
			// MkdirAll's mode is reduced by the umask, never raised.
			if st.Mode().Perm()&^0o755 != 0 {
				t.Errorf("folder %s has mode %o, want at most 755", p, st.Mode().Perm())
			}
		} else if st.Mode().Perm() != 0o644 {
			t.Errorf("file %s has mode %o, want 644", p, st.Mode().Perm())
		}
	}
	// No temporary file is left next to the entry.
	entries, err := os.ReadDir(filepath.Dir(desktop))
	if err != nil || len(entries) != 1 {
		t.Errorf("applications holds %v (%v), want only the entry", entries, err)
	}

	removed, err := Remove(context.Background(), e, env)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != len(want) {
		t.Errorf("removed %v, want %v", removed, want)
	}
	for _, p := range want {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s is still there (%v)", p, err)
		}
	}
	// Removing again is fine and removes nothing.
	removed, err = Remove(context.Background(), e, env)
	if err != nil || len(removed) != 0 {
		t.Errorf("second remove: %v, %v", removed, err)
	}
}

func TestInstallFollowsSymlinks(t *testing.T) {
	paths := storagetest.Isolate(t)
	exe := testProgram(t)
	link := filepath.Join(t.TempDir(), "radio-tutor")
	if err := os.Symlink(exe, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(context.Background(), testEntry(t), envFor(link)); err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(filepath.Join(filepath.Dir(paths.Data), "applications", "dev.example.app.desktop"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), `Exec="`+exe+`"`) || strings.Contains(string(text), link) {
		t.Errorf("the entry must point at the resolved program %s, not at the link:\n%s", exe, text)
	}
}

func TestDataHome(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
		fail bool
	}{
		{"xdg", map[string]string{"XDG_DATA_HOME": "/x/data", "HOME": "/h"}, "/x/data", false},
		{"empty xdg", map[string]string{"HOME": "/h"}, "/h/.local/share", false},
		{"relative xdg is ignored", map[string]string{"XDG_DATA_HOME": "data", "HOME": "/h"}, "/h/.local/share", false},
		{"no home", map[string]string{"XDG_DATA_HOME": "data"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dataHome(PlatformEnv{Getenv: func(k string) string { return tt.env[k] }})
			if tt.fail {
				hasRoute(t, err, "dataHome.DataHomeUnknown")
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("got %q (%v), want %q", got, err, tt.want)
			}
		})
	}
}

func TestInstallErrors(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name  string
		setup func(t *testing.T, dataHome string) (Entry, context.Context)
		route string
	}{
		{
			name: "icon that is not a PNG",
			setup: func(t *testing.T, _ string) (Entry, context.Context) {
				e := testEntry(t)
				e.IconPNG = []byte("not a picture")
				return e, context.Background()
			},
			route: "Install->install->scaleIcons->scaleIconsWith.IconDecodeFailed",
		},
		{
			name: "icon that is not square",
			setup: func(t *testing.T, _ string) (Entry, context.Context) {
				e := testEntry(t)
				e.IconPNG = testRectIcon(t, 300, 256)
				return e, context.Background()
			},
			route: "Install->install->scaleIcons->scaleIconsWith.IconNotSquare",
		},
		{
			name: "icon that is small",
			setup: func(t *testing.T, _ string) (Entry, context.Context) {
				e := testEntry(t)
				e.IconPNG = testIcon(t, 64)
				return e, context.Background()
			},
			route: "Install->install->scaleIcons->scaleIconsWith.IconTooSmall",
		},
		{
			name: "data folder is a file",
			setup: func(t *testing.T, dataHome string) (Entry, context.Context) {
				if err := os.RemoveAll(dataHome); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(dataHome, []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
				return testEntry(t), context.Background()
			},
			route: "Install->install->WriteFileAtomic->writeFileAtomic.DirCreateFailed",
		},
		{
			name: "cancelled",
			setup: func(t *testing.T, _ string) (Entry, context.Context) {
				return testEntry(t), cancelled
			},
			route: "Install->install->WriteFileAtomic->writeFileAtomic.WriteCancelled",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := storagetest.Isolate(t)
			e, ctx := tt.setup(t, filepath.Dir(paths.Data))
			_, err := Install(ctx, e, envFor(testProgram(t)))
			hasRoute(t, err, tt.route)
		})
	}
}

func TestRemoveErrors(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	t.Run("no data folder", func(t *testing.T) {
		env := DefaultEnv()
		env.Platform.Getenv = func(string) string { return "" }
		_, err := Remove(context.Background(), testEntry(t), env)
		hasRoute(t, err, "Remove->remove->dataHome.DataHomeUnknown")
		_, err = Install(context.Background(), testEntry(t), Env{Executable: envFor(testProgram(t)).Executable, Platform: env.Platform})
		hasRoute(t, err, "Install->install->dataHome.DataHomeUnknown")
	})
	t.Run("cancelled", func(t *testing.T) {
		storagetest.Isolate(t)
		_, err := Remove(cancelled, testEntry(t), DefaultEnv())
		hasRoute(t, err, "Remove->remove.RemoveCancelled")
	})
	t.Run("file that cannot be deleted", func(t *testing.T) {
		storagetest.SkipIfModesDoNotBlock(t)
		paths := storagetest.Isolate(t)
		e := testEntry(t)
		if _, err := Install(context.Background(), e, envFor(testProgram(t))); err != nil {
			t.Fatal(err)
		}
		apps := filepath.Join(filepath.Dir(paths.Data), "applications")
		if err := os.Chmod(apps, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(apps, 0o755); err != nil {
				t.Error(err)
			}
		})
		_, err := Remove(context.Background(), e, DefaultEnv())
		hasRoute(t, err, "Remove->remove.EntryFileRemoveFailed")
	})
}

// testRectIcon is a PNG that is not square.
func testRectIcon(t testing.TB, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestScaleIconsEncodeFailure(t *testing.T) {
	boom := errors.New("Boom", "the encoder failed")
	_, err := scaleIconsWith(testIcon(t, 256), []int{48}, func(io.Writer, image.Image) error { return boom })
	hasRoute(t, err, "scaleIconsWith.IconEncodeFailed")
}

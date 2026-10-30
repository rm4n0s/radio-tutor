//go:build windows

package menuentry

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"github.com/rm4n0s/errors"
	"golang.org/x/sys/windows"
)

// These tests make real shortcuts through the shell (under Wine, when the tests run through
// scripts/test-windows.sh), in throw-away folders: ProgramsDir is replaced, so the Start menu of
// the machine is never touched.

// Positions of the getters in the tables of the interfaces (see menuentry_windows.go).
const (
	vtLinkGetPath             = 3
	vtLinkGetDescription      = 6
	vtLinkGetWorkingDirectory = 8
	vtLinkGetIconLocation     = 16

	vtFileLoad = 5

	vtStoreGetValue = 5
)

// readBack is what a shortcut file holds, read through the shell.
type readBack struct {
	target, folder, description, icon string
	iconIndex                         int32
	appID                             string
}

// readShortcut loads the shortcut file at path and reads its properties.
func readShortcut(t *testing.T, path string) readBack {
	t.Helper()
	uninit, err := initCOM()
	if err != nil {
		t.Fatal(err)
	}
	defer uninit()
	link, err := createShellLink()
	if err != nil {
		t.Fatal(err)
	}
	defer link.release()
	file, err := link.queryInterface(&iidPersistFile)
	if err != nil {
		t.Fatal(err)
	}
	defer file.release()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	const stgmRead = 0
	if err := file.call(vtFileLoad, uintptr(unsafe.Pointer(p)), stgmRead); err != nil {
		t.Fatalf("Load %s: %v", path, err)
	}
	text := func(name string, index int, extra ...uintptr) string {
		buf := make([]uint16, windows.MAX_PATH)
		args := append([]uintptr{uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))}, extra...)
		if err := link.call(index, args...); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return windows.UTF16ToString(buf)
	}
	var got readBack
	// GetPath(pszFile, cch, pfd, fFlags): no find data, the path as stored.
	got.target = text("GetPath", vtLinkGetPath, 0, 0)
	got.folder = text("GetWorkingDirectory", vtLinkGetWorkingDirectory)
	got.description = text("GetDescription", vtLinkGetDescription)
	buf := make([]uint16, windows.MAX_PATH)
	if err := link.call(vtLinkGetIconLocation, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&got.iconIndex))); err != nil {
		t.Fatalf("GetIconLocation: %v", err)
	}
	got.icon = windows.UTF16ToString(buf)

	store, err := link.queryInterface(&iidPropertyStore)
	if err != nil {
		t.Fatal(err)
	}
	defer store.release()
	var value propVariant
	if err := store.call(vtStoreGetValue, uintptr(unsafe.Pointer(&pkeyAppUserModel)), uintptr(unsafe.Pointer(&value))); err != nil {
		t.Fatalf("GetValue: %v", err)
	}
	if value.vt == vtLPWSTR && value.str != nil {
		got.appID = windows.UTF16PtrToString(value.str)
	}
	return got
}

// envIn returns an Env that runs as exe and puts the shortcuts in programs.
func envIn(exe, programs string) Env {
	env := envFor(exe)
	env.Platform = PlatformEnv{ProgramsDir: func() (string, error) { return programs, nil }}
	return env
}

func TestInstallAndRemoveShortcut(t *testing.T) {
	exe := testProgram(t)
	programs := filepath.Join(t.TempDir(), "Programs")
	e := testEntry(t)
	env := envIn(exe, programs)
	link := filepath.Join(programs, "Example App.lnk")
	gui := filepath.Join(filepath.Dir(exe), "example-gui.exe")

	for run := 1; run <= 2; run++ { // the second run replaces the shortcut
		got, err := Install(context.Background(), e, env)
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if len(got) != 1 || got[0] != link {
			t.Fatalf("run %d: wrote %v, want [%s]", run, got, link)
		}
	}
	entries, err := os.ReadDir(programs)
	if err != nil || len(entries) != 1 {
		t.Errorf("Programs holds %v (%v), want only the shortcut", entries, err)
	}

	got := readShortcut(t, link)
	if !strings.EqualFold(got.target, gui) {
		t.Errorf("target %q, want %q", got.target, gui)
	}
	if !strings.EqualFold(got.folder, filepath.Dir(exe)) {
		t.Errorf("working folder %q, want %q", got.folder, filepath.Dir(exe))
	}
	if got.description != e.Comment {
		t.Errorf("description %q, want %q", got.description, e.Comment)
	}
	if !strings.EqualFold(got.icon, gui) || got.iconIndex != 0 {
		t.Errorf("icon %q,%d, want %q,0", got.icon, got.iconIndex, gui)
	}
	if got.appID != e.ID {
		t.Errorf("AppUserModelID %q, want %q", got.appID, e.ID)
	}

	removed, err := Remove(context.Background(), e, env)
	if err != nil || len(removed) != 1 || removed[0] != link {
		t.Fatalf("removed %v (%v), want [%s]", removed, err, link)
	}
	if _, err := os.Stat(link); !os.IsNotExist(err) {
		t.Errorf("the shortcut is still there (%v)", err)
	}
	removed, err = Remove(context.Background(), e, env)
	if err != nil || len(removed) != 0 {
		t.Errorf("second remove: %v, %v", removed, err)
	}
}

func TestProgramsDir(t *testing.T) {
	dir, err := programsDir()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(dir) || !strings.Contains(dir, "Programs") {
		t.Errorf("programs folder %q is not the Programs folder of the Start menu", dir)
	}
}

func TestInstallShortcutErrors(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	// folderWithFile makes a folder that holds a file, so it cannot be deleted or replaced.
	blockedLink := func(t *testing.T, programs string) {
		link := filepath.Join(programs, "Example App.lnk")
		if err := os.MkdirAll(link, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(link, "x"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name  string
		setup func(t *testing.T, programs string) (Entry, context.Context)
		route string
	}{
		{
			name: "name that is not a file name",
			setup: func(t *testing.T, _ string) (Entry, context.Context) {
				e := testEntry(t)
				e.Name = "Example: App"
				return e, context.Background()
			},
			route: "Install->install->shortcutPath.EntryInvalid",
		},
		{
			name: "no program name",
			setup: func(t *testing.T, _ string) (Entry, context.Context) {
				e := testEntry(t)
				e.GUIProgram = ""
				return e, context.Background()
			},
			route: "Install->install.EntryInvalid",
		},
		{
			name: "program that is not there",
			setup: func(t *testing.T, _ string) (Entry, context.Context) {
				e := testEntry(t)
				e.GUIProgram = "missing.exe"
				return e, context.Background()
			},
			route: "Install->install.GUIProgramMissing",
		},
		{
			name: "cancelled",
			setup: func(t *testing.T, _ string) (Entry, context.Context) {
				return testEntry(t), cancelled
			},
			route: "Install->install.InstallCancelled",
		},
		{
			name: "programs folder inside a file",
			setup: func(t *testing.T, programs string) (Entry, context.Context) {
				if err := os.WriteFile(programs, []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
				return testEntry(t), context.Background()
			},
			route: "Install->install.ProgramsDirCreateFailed",
		},
		{
			name: "shortcut that cannot be replaced",
			setup: func(t *testing.T, programs string) (Entry, context.Context) {
				blockedLink(t, programs)
				return testEntry(t), context.Background()
			},
			route: "Install->install.ShortcutReplaceFailed",
		},
		{
			name: "shortcut that cannot be saved",
			setup: func(t *testing.T, programs string) (Entry, context.Context) {
				// The temporary file's name is a folder.
				if err := os.MkdirAll(filepath.Join(programs, "Example App.lnk.tmp", "x"), 0o755); err != nil {
					t.Fatal(err)
				}
				return testEntry(t), context.Background()
			},
			route: "Install->install->saveShortcut.ShortcutFailed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			programs := filepath.Join(t.TempDir(), "Programs")
			e, ctx := tt.setup(t, programs)
			if _, err := os.Stat(programs); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			_, err := Install(ctx, e, envIn(testProgram(t), programs))
			hasRoute(t, err, tt.route)
			if appErr, _ := errors.FromError(err); appErr != nil && appErr.Tag == "ShortcutFailed" {
				if step, _ := appErr.Metadata["step"].(string); step != "Save" {
					t.Errorf("failed step %q, want Save", step)
				}
			}
		})
	}

	t.Run("shortcut that cannot be deleted", func(t *testing.T) {
		programs := filepath.Join(t.TempDir(), "Programs")
		blockedLink(t, programs)
		_, err := Remove(context.Background(), testEntry(t), envIn(testProgram(t), programs))
		hasRoute(t, err, "Remove->remove.EntryFileRemoveFailed")
	})
	t.Run("name that is not a file name", func(t *testing.T) {
		e := testEntry(t)
		e.Name = "a|b"
		_, err := Remove(context.Background(), e, envIn(testProgram(t), t.TempDir()))
		hasRoute(t, err, "Remove->remove->shortcutPath.EntryInvalid")
	})
	t.Run("programs folder unknown", func(t *testing.T) {
		_, err := programsDirFrom(func() (string, error) { return "", os.ErrNotExist })
		hasRoute(t, err, "programsDirFrom.ProgramsDirUnknown")
	})
}

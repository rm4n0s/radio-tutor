package storage

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
)

type doc struct {
	Format int    `json:"format"`
	Name   string `json:"name"`
}

func routeOf(t *testing.T, err error) *errors.Error {
	t.Helper()
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T: %v", err, err)
	}
	return appErr
}

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// unixModes is true where permission bits mean something (not on Windows).
// storagetest.UnixModes is the same, which this package's own tests cannot
// import (storagetest imports storage).
const unixModes = runtime.GOOS != "windows"

func TestWriteJSONAtomicRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "deeper", "doc.json")
	want := doc{Format: 1, Name: "Ada"}
	if err := WriteJSONAtomic(context.Background(), path, want); err != nil {
		t.Fatalf("WriteJSONAtomic: %v", err)
	}
	var got doc
	if err := ReadJSON(path, &got); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if unixModes && info.Mode().Perm() != FileMode {
		t.Errorf("file mode %v, want %v", info.Mode().Perm(), os.FileMode(FileMode))
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if unixModes && dirInfo.Mode().Perm() != DirMode {
		t.Errorf("folder mode %v, want %v", dirInfo.Mode().Perm(), os.FileMode(DirMode))
	}
	if names := listDir(t, filepath.Dir(path)); !reflect.DeepEqual(names, []string{"doc.json"}) {
		t.Errorf("folder holds %v, want only doc.json", names)
	}

	// Overwrite with the same path.
	want.Name = "Grace"
	if err := WriteJSONAtomic(context.Background(), path, want); err != nil {
		t.Fatalf("second WriteJSONAtomic: %v", err)
	}
	if err := ReadJSON(path, &got); err != nil || got != want {
		t.Errorf("after overwrite got %+v (%v), want %+v", got, err, want)
	}
}

func TestWriteJSONAtomicFailures(t *testing.T) {
	boom := errors.New("Boom", "boom")
	fail := func(ops *atomicOps, step string) {
		switch step {
		case "write":
			ops.write = func(f *os.File, data []byte) error {
				// A partial write, then the failure.
				if _, err := f.Write(data[:len(data)/2]); err != nil {
					return err
				}
				return boom
			}
		case "protect":
			ops.protect = func(string) error { return boom }
		case "sync":
			ops.sync = func(*os.File) error { return boom }
		case "close":
			ops.close = func(f *os.File) error {
				_ = f.Close() // really close it, then report a failure
				return boom
			}
		case "rename":
			ops.rename = func(string, string) error { return boom }
		}
	}
	tests := []struct {
		name      string
		step      string
		route     string
		oldIsKept bool
		newIsKept bool // the rename happened, the failure is after it
	}{
		{"protect", "protect", "writeJSONAtomic->writeFileAtomic.FilePrivateFailed", true, false},
		{"write", "write", "writeJSONAtomic->writeFileAtomic.TempFileWriteFailed", true, false},
		{"sync", "sync", "writeJSONAtomic->writeFileAtomic.TempFileSyncFailed", true, false},
		{"close", "close", "writeJSONAtomic->writeFileAtomic.TempFileCloseFailed", true, false},
		{"rename", "rename", "writeJSONAtomic->writeFileAtomic.RenameFailed", true, false},
		{"folder sync", "syncDir", "writeJSONAtomic->writeFileAtomic.DirSyncFailed", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "doc.json")
			old := doc{Format: 1, Name: "old"}
			if err := WriteJSONAtomic(context.Background(), path, old); err != nil {
				t.Fatal(err)
			}
			ops := realAtomicOps()
			if tt.step == "syncDir" {
				ops.syncDir = func(string) error { return boom }
			} else {
				fail(&ops, tt.step)
			}

			err := writeJSONAtomic(context.Background(), path, doc{Format: 1, Name: "new"}, ops)
			appErr := routeOf(t, err)
			if !appErr.HasRoute(tt.route) {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
			if !errors.Is(err, boom) {
				t.Errorf("the cause is lost: %v", err)
			}

			var got doc
			if err := ReadJSON(path, &got); err != nil {
				t.Fatalf("the file is unreadable after the failure: %v", err)
			}
			if tt.oldIsKept && got != old {
				t.Errorf("file = %+v, want the old %+v", got, old)
			}
			if tt.newIsKept && got.Name != "new" {
				t.Errorf("file = %+v, want the new content", got)
			}
			if names := listDir(t, dir); !reflect.DeepEqual(names, []string{"doc.json"}) {
				t.Errorf("folder holds %v, want only doc.json (no temporary file)", names)
			}
		})
	}
}

func TestWriteJSONAtomicFailureLeavesNoFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.json")
	ops := realAtomicOps()
	ops.rename = func(string, string) error { return errors.New("Boom", "boom") }
	if err := writeJSONAtomic(context.Background(), path, doc{Format: 1}, ops); err == nil {
		t.Fatal("expected an error")
	}
	if names := listDir(t, dir); len(names) != 0 {
		t.Errorf("folder holds %v, want nothing", names)
	}
}

func TestWriteJSONAtomicErrors(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name  string
		ctx   context.Context
		path  func(t *testing.T) string
		value any
		route string
	}{
		{
			name:  "cancelled context",
			ctx:   cancelled,
			path:  func(t *testing.T) string { return filepath.Join(t.TempDir(), "doc.json") },
			value: doc{Format: 1},
			route: "WriteJSONAtomic->writeJSONAtomic->writeFileAtomic.WriteCancelled",
		},
		{
			name:  "value that is not JSON",
			ctx:   context.Background(),
			path:  func(t *testing.T) string { return filepath.Join(t.TempDir(), "doc.json") },
			value: make(chan int),
			route: "WriteJSONAtomic->writeJSONAtomic.JSONMarshalFailed",
		},
		{
			name: "parent is a file",
			ctx:  context.Background(),
			path: func(t *testing.T) string {
				file := filepath.Join(t.TempDir(), "file")
				if err := os.WriteFile(file, nil, FileMode); err != nil {
					t.Fatal(err)
				}
				return filepath.Join(file, "doc.json")
			},
			value: doc{Format: 1},
			route: "WriteJSONAtomic->writeJSONAtomic->writeFileAtomic.DirCreateFailed",
		},
		{
			name: "folder is read-only",
			ctx:  context.Background(),
			path: func(t *testing.T) string {
				if !unixModes {
					t.Skip("Windows keeps no Unix permission bits: a read-only folder still takes new files")
				}
				if os.Geteuid() == 0 {
					t.Skip("root ignores folder modes")
				}
				dir := filepath.Join(t.TempDir(), "ro")
				if err := os.Mkdir(dir, 0o500); err != nil {
					t.Fatal(err)
				}
				return filepath.Join(dir, "doc.json")
			},
			value: doc{Format: 1},
			route: "WriteJSONAtomic->writeJSONAtomic->writeFileAtomic.TempFileCreateFailed",
		},
		{
			name: "target is a folder",
			ctx:  context.Background(),
			path: func(t *testing.T) string {
				p := filepath.Join(t.TempDir(), "doc.json")
				if err := os.Mkdir(p, DirMode); err != nil {
					t.Fatal(err)
				}
				return p
			},
			value: doc{Format: 1},
			route: "WriteJSONAtomic->writeJSONAtomic->writeFileAtomic.RenameFailed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := WriteJSONAtomic(tt.ctx, tt.path(t), tt.value)
			if err == nil {
				t.Fatal("expected an error")
			}
			if appErr := routeOf(t, err); !appErr.HasRoute(tt.route) {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
		})
	}
}

func TestReadJSON(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), FileMode); err != nil {
			t.Fatal(err)
		}
		return p
	}
	tests := []struct {
		name    string
		path    string
		want    doc
		route   string // empty: success
		notExit bool
	}{
		{name: "valid", path: write("ok.json", `{"format":1,"name":"Ada"}`), want: doc{Format: 1, Name: "Ada"}},
		{name: "missing", path: filepath.Join(dir, "missing.json"), route: "ReadJSON.JSONNotFound", notExit: true},
		{name: "truncated", path: write("cut.json", `{"format":1,"na`), route: "ReadJSON.JSONInvalid"},
		{name: "wrong type", path: write("type.json", `{"format":"one"}`), route: "ReadJSON.JSONInvalid"},
		{name: "a folder", path: dir, route: "ReadJSON.JSONReadFailed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got doc
			err := ReadJSON(tt.path, &got)
			if tt.route == "" {
				if err != nil || got != tt.want {
					t.Fatalf("got %+v, %v; want %+v", got, err, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatal("expected an error")
			}
			if appErr := routeOf(t, err); !appErr.HasRoute(tt.route) {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
			if errors.Is(err, fs.ErrNotExist) != tt.notExit {
				t.Errorf("errors.Is(err, fs.ErrNotExist) = %v, want %v", !tt.notExit, tt.notExit)
			}
		})
	}
}

func TestQuarantineBroken(t *testing.T) {
	now := time.Date(2026, 10, 30, 14, 5, 9, 0, time.FixedZone("x", 3600))
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.json")
	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), FileMode); err != nil {
			t.Fatal(err)
		}
	}

	var v doc
	write("{not json")
	if err := ReadJSON(path, &v); err == nil {
		t.Fatal("expected the broken file to fail ReadJSON")
	}
	first, err := QuarantineBroken(path, now)
	if err != nil {
		t.Fatalf("QuarantineBroken: %v", err)
	}
	if want := path + ".broken-20261030T130509Z"; first != want {
		t.Errorf("quarantined as %q, want %q", first, want)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the broken file is still at its path (%v)", err)
	}
	if b, err := os.ReadFile(first); err != nil || string(b) != "{not json" {
		t.Errorf("quarantined content = %q (%v)", b, err)
	}

	// The same second again: nothing is overwritten.
	write("{still bad")
	second, err := QuarantineBroken(path, now)
	if err != nil {
		t.Fatalf("second QuarantineBroken: %v", err)
	}
	write("{worse")
	third, err := QuarantineBroken(path, now)
	if err != nil {
		t.Fatalf("third QuarantineBroken: %v", err)
	}
	if second != first+"-2" || third != first+"-3" {
		t.Errorf("names %q and %q, want %q and %q", second, third, first+"-2", first+"-3")
	}
	for p, want := range map[string]string{first: "{not json", second: "{still bad", third: "{worse"} {
		if b, err := os.ReadFile(p); err != nil || string(b) != want {
			t.Errorf("%s = %q (%v), want %q", p, b, err, want)
		}
	}
}

func TestQuarantineBrokenErrors(t *testing.T) {
	now := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.json")
	tooLong := filepath.Join(dir, strings.Repeat("a", 300))
	tests := []struct {
		name string
		path string
	}{
		{"missing file", missing},
		{"name too long to stat", tooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := QuarantineBroken(tt.path, now)
			if err == nil {
				t.Fatal("expected an error")
			}
			if appErr := routeOf(t, err); !appErr.HasRoute("QuarantineBroken.QuarantineFailed") {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
		})
	}
}

func TestEnsureDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "a", "b")
	if err := EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	if err := EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir again: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() || (unixModes && info.Mode().Perm() != DirMode) {
		t.Errorf("folder: %v, %v", info, err)
	}
}

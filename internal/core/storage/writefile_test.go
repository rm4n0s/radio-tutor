package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

)

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "file.txt")
	for _, content := range []string{"first", "second, replacing the first"} {
		if err := WriteFileAtomic(context.Background(), path, []byte(content), 0o644, 0o755); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != content {
			t.Fatalf("read %q (%v), want %q", got, err, content)
		}
	}
	if unixModes {
		for p, want := range map[string]os.FileMode{path: 0o644, filepath.Dir(path): 0o755} {
			st, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if got := st.Mode().Perm(); got != want {
				t.Errorf("%s has mode %o, want %o", p, got, want)
			}
		}
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Errorf("the folder holds %v (%v), want only the file", entries, err)
	}
}

func TestWriteFileAtomicErrors(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name  string
		ctx   context.Context
		path  func(t *testing.T) string
		route string
	}{
		{
			name:  "cancelled context",
			ctx:   cancelled,
			path:  func(t *testing.T) string { return filepath.Join(t.TempDir(), "f") },
			route: "WriteFileAtomic->writeFileAtomic.WriteCancelled",
		},
		{
			name: "parent is a file",
			ctx:  context.Background(),
			path: func(t *testing.T) string {
				file := filepath.Join(t.TempDir(), "file")
				if err := os.WriteFile(file, nil, FileMode); err != nil {
					t.Fatal(err)
				}
				return filepath.Join(file, "f")
			},
			route: "WriteFileAtomic->writeFileAtomic.DirCreateFailed",
		},
		{
			name: "target is a folder",
			ctx:  context.Background(),
			path: func(t *testing.T) string {
				p := filepath.Join(t.TempDir(), "f")
				if err := os.Mkdir(p, DirMode); err != nil {
					t.Fatal(err)
				}
				return p
			},
			route: "WriteFileAtomic->writeFileAtomic.RenameFailed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := WriteFileAtomic(tt.ctx, tt.path(t), []byte("x"), 0o644, 0o755)
			if err == nil {
				t.Fatal("expected an error")
			}
			if appErr := routeOf(t, err); !appErr.HasRoute(tt.route) {
				t.Errorf("route %s, want %s", appErr.Route(), tt.route)
			}
		})
	}
}

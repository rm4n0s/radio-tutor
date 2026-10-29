package storage

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/rm4n0s/errors"
)

func TestLockRefusesSecondHolder(t *testing.T) {
	paths := Paths{State: filepath.Join(t.TempDir(), "state", "radio-tutor")}

	unlock, err := Lock(paths)
	if err != nil {
		t.Fatalf("first Lock: %v", err)
	}
	info, err := os.Stat(paths.LockFile())
	if err != nil || info.Mode().Perm() != FileMode {
		t.Errorf("lock file: %v, %v", info, err)
	}

	_, err = Lock(paths)
	if err == nil {
		t.Fatal("second Lock succeeded while the first is held")
	}
	appErr := routeOf(t, err)
	if appErr.Tag != "AlreadyRunning" || !appErr.HasRoute("Lock->lock.AlreadyRunning") {
		t.Errorf("unexpected failure path: %s", appErr.Route())
	}
	if appErr.Metadata["path"] != paths.LockFile() {
		t.Errorf("metadata = %v", appErr.Metadata)
	}

	unlock()
	unlock() // calling it twice is fine

	again, err := Lock(paths)
	if err != nil {
		t.Fatalf("Lock after unlock: %v", err)
	}
	again()
}

func TestLockErrors(t *testing.T) {
	boom := syscall.EIO
	tests := []struct {
		name  string
		paths func(t *testing.T) Paths
		flock func(fd, how int) error
		route string
	}{
		{
			name: "state folder cannot be made",
			paths: func(t *testing.T) Paths {
				file := filepath.Join(t.TempDir(), "file")
				if err := os.WriteFile(file, nil, FileMode); err != nil {
					t.Fatal(err)
				}
				return Paths{State: filepath.Join(file, "state")}
			},
			flock: syscall.Flock,
			route: "lock->EnsureDir.DirCreateFailed",
		},
		{
			name: "lock path is a folder",
			paths: func(t *testing.T) Paths {
				p := Paths{State: t.TempDir()}
				if err := os.Mkdir(p.LockFile(), DirMode); err != nil {
					t.Fatal(err)
				}
				return p
			},
			flock: syscall.Flock,
			route: "lock.LockOpenFailed",
		},
		{
			name:  "flock fails",
			paths: func(t *testing.T) Paths { return Paths{State: t.TempDir()} },
			flock: func(int, int) error { return boom },
			route: "lock.LockFailed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unlock, err := lock(tt.paths(t), tt.flock)
			if err == nil {
				unlock()
				t.Fatal("expected an error")
			}
			if unlock != nil {
				t.Error("unlock should be nil on failure")
			}
			if appErr := routeOf(t, err); !appErr.HasRoute(tt.route) {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
			if tt.name == "flock fails" && !errors.Is(err, boom) {
				t.Errorf("the cause is lost: %v", err)
			}
		})
	}
}

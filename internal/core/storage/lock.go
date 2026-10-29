package storage

import (
	"os"
	"sync"
	"syscall"

	"github.com/rm4n0s/errors"
)

// Lock takes the exclusive single-instance lock (SPEC §4): a non-blocking
// flock on the lock file of paths, which is created (with the state folder)
// when missing. It returns the function that releases the lock; the function
// may be called more than once. A lock held by another open file description,
// in this process or in another one, fails with the tag AlreadyRunning. The
// operating system also drops the lock when the process ends.
func Lock(paths Paths) (unlock func(), err error) {
	return lock(paths, syscall.Flock)
}

func lock(paths Paths, flock func(fd, how int) error) (func(), error) {
	if err := EnsureDir(paths.State); err != nil {
		return nil, err
	}
	path := paths.LockFile()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, FileMode)
	if err != nil {
		return nil, errors.NewErr("LockOpenFailed", err).SetMetadata("path", path)
	}
	if err := flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close() // the lock error is the one to report
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errors.NewErr("AlreadyRunning", err).SetMetadata("path", path)
		}
		return nil, errors.NewErr("LockFailed", err).SetMetadata("path", path)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			// Closing the descriptor releases the flock; a close error leaves
			// nothing for the caller to do.
			_ = f.Close()
		})
	}, nil
}

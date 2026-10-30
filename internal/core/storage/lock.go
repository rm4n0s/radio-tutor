package storage

import (
	"os"
	"sync"

	"github.com/rm4n0s/errors"
)

// Lock takes the exclusive single-instance lock (SPEC §4): a non-blocking
// lock on the lock file of paths, which is created (with the state folder)
// when missing (flock on Linux, LockFileEx on Windows). It returns the function
// that releases the lock; the function may be called more than once. A lock
// held through another open file, in this process or in another one, fails with
// the tag AlreadyRunning. The operating system also drops the lock when the
// process ends.
func Lock(paths Paths) (unlock func(), err error) {
	return lock(paths, tryLockFile)
}

// lock is Lock with the operating system's locking call given, so that tests
// can make it fail. lockFile returns an error for which isLockHeld is true when
// another holder has the lock.
func lock(paths Paths, lockFile func(f *os.File) error) (func(), error) {
	if err := EnsureDir(paths.State); err != nil {
		return nil, err
	}
	path := paths.LockFile()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, FileMode)
	if err != nil {
		return nil, errors.NewErr("LockOpenFailed", err).SetMetadata("path", path)
	}
	if err := lockFile(f); err != nil {
		_ = f.Close() // the lock error is the one to report
		if isLockHeld(err) {
			return nil, errors.NewErr("AlreadyRunning", err).SetMetadata("path", path)
		}
		return nil, errors.NewErr("LockFailed", err).SetMetadata("path", path)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			// Closing the file releases the lock; a close error leaves
			// nothing for the caller to do.
			_ = f.Close()
		})
	}, nil
}

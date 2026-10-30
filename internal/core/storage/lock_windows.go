//go:build windows

package storage

import (
	"os"

	"github.com/rm4n0s/errors"
	"golang.org/x/sys/windows"
)

// tryLockFile takes an exclusive LockFileEx lock on the first byte of f without
// waiting. The lock belongs to the handle, so a second open of the same file,
// even in this process, is refused like a second instance.
func tryLockFile(f *os.File) error {
	var overlapped windows.Overlapped
	return windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
}

// isLockHeld reports whether err from tryLockFile says that another holder has
// the lock (ERROR_LOCK_VIOLATION).
func isLockHeld(err error) bool {
	return errors.Is(err, windows.ERROR_LOCK_VIOLATION)
}

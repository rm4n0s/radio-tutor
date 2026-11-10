//go:build !windows

package storage

import (
	"os"
	"syscall"

	"github.com/rm4n0s/errors"
)

// tryLockFile takes an exclusive flock on f without waiting.
func tryLockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// isLockHeld reports whether err from tryLockFile says that another holder has
// the lock.
func isLockHeld(err error) bool {
	return errors.Is(err, syscall.EWOULDBLOCK)
}

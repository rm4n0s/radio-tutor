//go:build linux

package main

import (
	"strconv"

	"golang.org/x/sys/unix"
)

// ptySlavePath unlocks the slave of the pty master fd and returns its path.
func ptySlavePath(fd int) (string, error) {
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		return "", err
	}
	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		return "", err
	}
	return "/dev/pts/" + strconv.Itoa(n), nil
}

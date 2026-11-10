//go:build darwin

package main

import (
	"bytes"
	"syscall"
	"unsafe"
)

// The requests of macOS's /dev/ptmx (<sys/ttycom.h>) that x/sys/unix does not name.
const (
	tiocPtyGrant = 0x20007454
	tiocPtyGname = 0x40807453
	tiocPtyUnlk  = 0x20007452
)

// ptySlavePath grants and unlocks the slave of the pty master fd and returns its path.
func ptySlavePath(fd int) (string, error) {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), tiocPtyGrant, 0); e != 0 {
		return "", e
	}
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), tiocPtyUnlk, 0); e != 0 {
		return "", e
	}
	var name [128]byte
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), tiocPtyGname, uintptr(unsafe.Pointer(&name[0]))); e != 0 {
		return "", e
	}
	n := bytes.IndexByte(name[:], 0)
	if n < 0 {
		n = len(name)
	}
	return string(name[:n]), nil
}

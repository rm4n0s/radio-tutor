//go:build !windows

package main

import (
	"io"
	"os"

	"github.com/rm4n0s/errors"
	"golang.org/x/sys/unix"
)

// terminalEcho returns the echo control of r when r is a terminal (a TTY, with the
// termios ioctls), otherwise nil.
func terminalEcho(r io.Reader) echoControl {
	f, ok := r.(*os.File)
	if !ok {
		return nil
	}
	e := termiosEcho{fd: int(f.Fd()), ops: realTermios()}
	if _, err := e.ops.get(e.fd); err != nil {
		return nil
	}
	return e
}

// termiosOps are the two ioctls termiosEcho uses, so that tests can make them fail.
type termiosOps struct {
	get func(fd int) (*unix.Termios, error)
	set func(fd int, t *unix.Termios) error
}

func realTermios() termiosOps {
	return termiosOps{
		get: func(fd int) (*unix.Termios, error) { return unix.IoctlGetTermios(fd, unix.TCGETS) },
		set: func(fd int, t *unix.Termios) error { return unix.IoctlSetTermios(fd, unix.TCSETS, t) },
	}
}

// termiosEcho switches the ECHO flag of the terminal on a file descriptor with
// the termios ioctls.
type termiosEcho struct {
	fd  int
	ops termiosOps
}

// Off clears the ECHO flag and returns the function that sets the terminal back
// to the settings it had before. The restore function is meant for one
// goroutine; calling it again repeats the first result.
func (e termiosEcho) Off() (func() error, error) {
	saved, err := e.ops.get(e.fd)
	if err != nil {
		return nil, errors.NewErr("TerminalReadFailed", err)
	}
	quiet := *saved
	quiet.Lflag &^= unix.ECHO
	if err := e.ops.set(e.fd, &quiet); err != nil {
		return nil, errors.NewErr("TerminalWriteFailed", err)
	}
	var restored bool
	var restoreErr error
	return func() error {
		if restored {
			return restoreErr
		}
		restored = true
		if err := e.ops.set(e.fd, saved); err != nil {
			restoreErr = errors.NewErr("TerminalRestoreFailed", err)
		}
		return restoreErr
	}, nil
}

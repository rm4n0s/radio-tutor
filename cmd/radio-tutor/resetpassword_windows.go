//go:build windows

package main

import (
	"io"
	"os"

	"github.com/rm4n0s/errors"
	"golang.org/x/sys/windows"
)

// terminalEcho returns the echo control of r when r is a console (a Windows
// console input handle), otherwise nil: a pipe, a file or NUL is not.
func terminalEcho(r io.Reader) echoControl {
	f, ok := r.(*os.File)
	if !ok {
		return nil
	}
	e := consoleEcho{handle: windows.Handle(f.Fd()), ops: realConsole()}
	if _, err := e.ops.get(e.handle); err != nil {
		return nil
	}
	return e
}

// consoleOps are the two console calls consoleEcho uses, so that tests can make
// them fail.
type consoleOps struct {
	get func(h windows.Handle) (uint32, error)
	set func(h windows.Handle, mode uint32) error
}

func realConsole() consoleOps {
	return consoleOps{
		get: func(h windows.Handle) (uint32, error) {
			var mode uint32
			err := windows.GetConsoleMode(h, &mode)
			return mode, err
		},
		set: windows.SetConsoleMode,
	}
}

// consoleEcho switches ENABLE_ECHO_INPUT of the console input handle with
// GetConsoleMode and SetConsoleMode. The other flags stay as they are, so line
// editing and Ctrl+C keep working.
type consoleEcho struct {
	handle windows.Handle
	ops    consoleOps
}

// Off clears ENABLE_ECHO_INPUT and returns the function that sets the console
// back to the mode it had before. The restore function is meant for one
// goroutine; calling it again repeats the first result.
func (e consoleEcho) Off() (func() error, error) {
	saved, err := e.ops.get(e.handle)
	if err != nil {
		return nil, errors.NewErr("TerminalReadFailed", err)
	}
	if err := e.ops.set(e.handle, saved&^windows.ENABLE_ECHO_INPUT); err != nil {
		return nil, errors.NewErr("TerminalWriteFailed", err)
	}
	var restored bool
	var restoreErr error
	return func() error {
		if restored {
			return restoreErr
		}
		restored = true
		if err := e.ops.set(e.handle, saved); err != nil {
			restoreErr = errors.NewErr("TerminalRestoreFailed", err)
		}
		return restoreErr
	}, nil
}

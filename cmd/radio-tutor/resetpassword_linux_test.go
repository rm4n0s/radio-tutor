//go:build !windows

package main

import (
	"os"
	"testing"

	"github.com/rm4n0s/errors"
	"golang.org/x/sys/unix"
)

// openPty opens a pseudo-terminal pair, or skips the test when the system has none.
func openPty(t *testing.T) (master, slave *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("no /dev/ptmx: %v", err)
	}
	t.Cleanup(func() { _ = master.Close() })
	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Skipf("cannot unlock the pty: %v", err)
	}
	n, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Skipf("cannot name the pty: %v", err)
	}
	slave, err = os.OpenFile("/dev/pts/"+itoa(n), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("cannot open the pty: %v", err)
	}
	t.Cleanup(func() { _ = slave.Close() })
	return master, slave
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

func TestTerminalEchoRealTerminal(t *testing.T) {
	_, slave := openPty(t)
	fd := int(slave.Fd())
	before, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		t.Fatal(err)
	}
	if before.Lflag&unix.ECHO == 0 {
		t.Fatal("the pty starts with echo off")
	}
	echo := terminalEcho(slave)
	if echo == nil {
		t.Fatal("a pty slave is a terminal")
	}
	restore, err := echo.Off()
	if err != nil {
		t.Fatal(err)
	}
	during, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		t.Fatal(err)
	}
	if during.Lflag&unix.ECHO != 0 {
		t.Error("echo is still on after Off")
	}
	if during.Lflag&unix.ICANON == 0 {
		t.Error("Off must keep line editing")
	}
	for range 2 { // restoring twice is fine
		if err := restore(); err != nil {
			t.Fatal(err)
		}
	}
	after, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		t.Fatal(err)
	}
	if *after != *before {
		t.Errorf("terminal settings changed:\nbefore %+v\nafter  %+v", *before, *after)
	}
}

func TestTerminalEchoFailures(t *testing.T) {
	broken := errors.New("Fake", "ioctl failed")
	saved := &unix.Termios{Lflag: unix.ECHO | unix.ICANON}
	tests := []struct {
		name    string
		ops     termiosOps
		restore bool // the failure is in the restore function
		wantTag string
		route   string
	}{
		{
			name:    "reading the settings fails",
			ops:     termiosOps{get: func(int) (*unix.Termios, error) { return nil, broken }},
			wantTag: "TerminalReadFailed", route: "termiosEcho.Off.TerminalReadFailed",
		},
		{
			name: "switching the echo off fails",
			ops: termiosOps{
				get: func(int) (*unix.Termios, error) { c := *saved; return &c, nil },
				set: func(int, *unix.Termios) error { return broken },
			},
			wantTag: "TerminalWriteFailed", route: "termiosEcho.Off.TerminalWriteFailed",
		},
		{
			name: "restoring the settings fails",
			ops: func() termiosOps {
				calls := 0
				return termiosOps{
					get: func(int) (*unix.Termios, error) { c := *saved; return &c, nil },
					set: func(int, *unix.Termios) error {
						calls++
						if calls == 2 {
							return broken
						}
						return nil
					},
				}
			}(),
			restore: true,
			wantTag: "TerminalRestoreFailed", route: "termiosEcho.Off.func1.TerminalRestoreFailed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore, err := termiosEcho{fd: 7, ops: tt.ops}.Off()
			if tt.restore {
				if err != nil {
					t.Fatal(err)
				}
				err = restore()
				if again := restore(); again != err {
					t.Errorf("a second restore gave %v, want %v", again, err)
				}
			}
			appErr, ok := errors.FromError(err)
			if !ok {
				t.Fatalf("expected *errors.Error, got %T: %v", err, err)
			}
			if appErr.Tag != tt.wantTag || !appErr.HasRoute(tt.route) {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
		})
	}
}

func TestTerminalEchoRestoresTheSettingsItSaw(t *testing.T) {
	var written []unix.Termios
	original := unix.Termios{Iflag: 5, Lflag: unix.ECHO | unix.ICANON | unix.ISIG}
	e := termiosEcho{fd: 3, ops: termiosOps{
		get: func(int) (*unix.Termios, error) { c := original; return &c, nil },
		set: func(_ int, t *unix.Termios) error { written = append(written, *t); return nil },
	}}
	restore, err := e.Off()
	if err != nil {
		t.Fatal(err)
	}
	if err := restore(); err != nil {
		t.Fatal(err)
	}
	if len(written) != 2 || written[0].Lflag != unix.ICANON|unix.ISIG || written[1] != original {
		t.Errorf("written settings: %+v", written)
	}
}

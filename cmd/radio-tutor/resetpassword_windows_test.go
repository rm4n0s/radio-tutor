//go:build windows

package main

import (
	"testing"

	"github.com/rm4n0s/errors"
	"golang.org/x/sys/windows"
)

// A real console is not available to a test run under Wine without a display,
// so the console calls are replaced (docs/MANUAL-CHECKS.md has the real check).

func TestConsoleEchoFailures(t *testing.T) {
	broken := errors.New("Fake", "console call failed")
	const saved = windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT
	tests := []struct {
		name    string
		ops     consoleOps
		restore bool // the failure is in the restore function
		wantTag string
		route   string
	}{
		{
			name:    "reading the mode fails",
			ops:     consoleOps{get: func(windows.Handle) (uint32, error) { return 0, broken }},
			wantTag: "TerminalReadFailed", route: "consoleEcho.Off.TerminalReadFailed",
		},
		{
			name: "switching the echo off fails",
			ops: consoleOps{
				get: func(windows.Handle) (uint32, error) { return saved, nil },
				set: func(windows.Handle, uint32) error { return broken },
			},
			wantTag: "TerminalWriteFailed", route: "consoleEcho.Off.TerminalWriteFailed",
		},
		{
			name: "restoring the mode fails",
			ops: func() consoleOps {
				calls := 0
				return consoleOps{
					get: func(windows.Handle) (uint32, error) { return saved, nil },
					set: func(windows.Handle, uint32) error {
						calls++
						if calls == 2 {
							return broken
						}
						return nil
					},
				}
			}(),
			restore: true,
			wantTag: "TerminalRestoreFailed", route: "consoleEcho.Off.func1.TerminalRestoreFailed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore, err := consoleEcho{handle: 7, ops: tt.ops}.Off()
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

func TestConsoleEchoRestoresTheModeItSaw(t *testing.T) {
	var written []uint32
	const original = windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT | windows.ENABLE_PROCESSED_INPUT
	e := consoleEcho{handle: 3, ops: consoleOps{
		get: func(windows.Handle) (uint32, error) { return original, nil },
		set: func(_ windows.Handle, mode uint32) error { written = append(written, mode); return nil },
	}}
	restore, err := e.Off()
	if err != nil {
		t.Fatal(err)
	}
	if err := restore(); err != nil {
		t.Fatal(err)
	}
	want := []uint32{windows.ENABLE_LINE_INPUT | windows.ENABLE_PROCESSED_INPUT, original}
	if len(written) != 2 || written[0] != want[0] || written[1] != want[1] {
		t.Errorf("written modes: %v, want %v", written, want)
	}
}

package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/rm4n0s/errors"
)

// noHandleWriter is a writer whose writes fail, like the standard error of a GUI
// program started from a terminal.
type noHandleWriter struct{}

func (noHandleWriter) Write([]byte) (int, error) {
	return 0, errors.New("FakeWriteFailed", "no handle")
}

// recordCloser is a console output that records what is written and whether it was closed.
type recordCloser struct {
	bytes.Buffer
	closed bool
}

func (r *recordCloser) Close() error { r.closed = true; return nil }

func TestRefuseGUICommand(t *testing.T) {
	console := &recordCloser{}
	var boxed string
	tests := []struct {
		name        string
		deps        guiRefusal
		wantStderr  bool
		wantConsole bool
		wantBox     bool
	}{
		{
			name:       "standard error works",
			deps:       guiRefusal{stderr: &bytes.Buffer{}, console: func() (io.WriteCloser, error) { return console, nil }, box: func(s string) { boxed = s }},
			wantStderr: true,
		},
		{
			name:        "no standard error, a parent console",
			deps:        guiRefusal{stderr: noHandleWriter{}, console: func() (io.WriteCloser, error) { return console, nil }, box: func(s string) { boxed = s }},
			wantConsole: true,
		},
		{
			name:    "no standard error, no console: a message box",
			deps:    guiRefusal{stderr: noHandleWriter{}, console: func() (io.WriteCloser, error) { return nil, errors.New("FakeNoConsole", "none") }, box: func(s string) { boxed = s }},
			wantBox: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			console.Reset()
			console.closed = false
			boxed = ""
			if got := refuseGUICommand(tt.deps); got != exitUsage {
				t.Errorf("exit code = %d, want %d", got, exitUsage)
			}
			if b, ok := tt.deps.stderr.(*bytes.Buffer); ok {
				if (b.String() == guiCommandMessage) != tt.wantStderr {
					t.Errorf("stderr = %q", b.String())
				}
			}
			if (console.String() == guiCommandMessage) != tt.wantConsole || console.closed != tt.wantConsole {
				t.Errorf("console = %q, closed %v, want used %v", console.String(), console.closed, tt.wantConsole)
			}
			if (boxed == guiCommandMessage) != tt.wantBox {
				t.Errorf("box = %q, want shown %v", boxed, tt.wantBox)
			}
		})
	}
}

func TestGUICommandMessageNamesTheCLI(t *testing.T) {
	if !strings.Contains(guiCommandMessage, "radio-tutor-cli.exe") {
		t.Fatal("the message does not name radio-tutor-cli.exe")
	}
}

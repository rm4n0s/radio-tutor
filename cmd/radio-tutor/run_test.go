package main

import (
	"bytes"
	"context"
	"io"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/rm4n0s/errors"
)

func TestRun(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	guiErr := errors.New("FakeGUIFailure", "no display")
	alreadyRunning := errors.New("AlreadyRunning", "another instance holds the lock")

	tests := []struct {
		name       string
		ctx        context.Context
		args       []string
		gui        func(context.Context) error
		wantCode   int
		wantGUI    bool
		wantStdout string
		wantStderr string
	}{
		{name: "no arguments starts the gui", args: nil, gui: func(context.Context) error { return nil }, wantCode: 0, wantGUI: true},
		{name: "gui failure", args: nil, gui: func(context.Context) error { return guiErr }, wantCode: 1, wantGUI: true, wantStderr: "no display"},
		{name: "another instance is running", args: nil, gui: func(context.Context) error { return alreadyRunning }, wantCode: 2, wantGUI: true, wantStderr: "Radio Tutor is already running"},
		{name: "cancelled context does not start the gui", ctx: cancelled, args: nil, wantCode: 1, wantStderr: "context canceled"},
		{name: "validate needs a path", args: []string{"validate"}, wantCode: 2, wantStderr: "usage: radio-tutor validate"},
		{name: "pack needs two paths", args: []string{"pack", "a"}, wantCode: 2, wantStderr: "usage: radio-tutor pack"},
		{name: "reset-password needs a username", args: []string{"reset-password"}, wantCode: 2, wantStderr: "usage: radio-tutor reset-password <username>"},
		{name: "setup-desktop takes only --remove", args: []string{"setup-desktop", "now"}, wantCode: 2, wantStderr: "usage: radio-tutor setup-desktop [--remove]"},
		{name: "version", args: []string{"version"}, wantCode: 0, wantStdout: "\n"},
		{name: "version with arguments", args: []string{"version", "x"}, wantCode: 2, wantStderr: "takes no arguments"},
		{name: "help", args: []string{"help"}, wantCode: 0, wantStdout: "usage: radio-tutor"},
		{name: "unknown command", args: []string{"bogus"}, wantCode: 2, wantStderr: `unknown command "bogus"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			started := false
			gui := func(ctx context.Context) error {
				started = true
				if tt.gui == nil {
					t.Error("the gui must not start")
					return nil
				}
				return tt.gui(ctx)
			}
			var stdout, stderr bytes.Buffer
			code := run(ctx, tt.args, strings.NewReader(""), &stdout, &stderr, gui)
			if code != tt.wantCode {
				t.Errorf("exit code %d, want %d (stderr %q)", code, tt.wantCode, stderr.String())
			}
			if started != tt.wantGUI {
				t.Errorf("gui started %v, want %v", started, tt.wantGUI)
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout %q does not contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr %q does not contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestRunUsageGoesToStderr(t *testing.T) {
	var stdout bytes.Buffer
	if code := run(context.Background(), []string{"bogus"}, strings.NewReader(""), &stdout, io.Discard, nil); code != 2 {
		t.Fatalf("exit code %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout %q, want empty", stdout.String())
	}
}

func TestVersionOf(t *testing.T) {
	tests := []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{"no build info", nil, false, "unknown"},
		{"nil info", nil, true, "unknown"},
		{"empty version", &debug.BuildInfo{}, true, "unknown"},
		{"devel", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true, "(devel)"},
		{"tagged", &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, true, "v1.2.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := versionOf(tt.info, tt.ok); got != tt.want {
				t.Errorf("versionOf = %q, want %q", got, tt.want)
			}
		})
	}
}

// failWriter fails every write.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestRunWriteFailure(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"version to a broken stdout", []string{"version"}, 1},
		{"help to a broken stdout", []string{"help"}, 1},
		{"usage error stays a usage error", []string{"bogus"}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code := run(context.Background(), tt.args, strings.NewReader(""), failWriter{}, failWriter{}, nil); code != tt.want {
				t.Errorf("exit code %d, want %d", code, tt.want)
			}
		})
	}
}

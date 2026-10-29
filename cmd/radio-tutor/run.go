package main

import (
	"context"
	"fmt"
	"io"
	"runtime/debug"
)

// Exit codes of the command.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

const usage = `usage: radio-tutor [command]

With no command the application window opens.

Commands:
  validate [--json] <file.cassette|folder>   validate a cassette
  pack [--force] <folder> <out.cassette>     validate a folder and pack it
  reset-password <username>                  set a new password for a user
  version                                    print the version
`

// run executes the command line args (without the program name) and returns the
// exit code. With no args it calls startGUI, which opens the application window
// and blocks until it is closed. Output goes to stdout and stderr; stdin is read
// by reset-password.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, startGUI func() error) int {
	out, errOut := &printer{w: stdout}, &printer{w: stderr}
	code := dispatch(ctx, args, stdin, out, errOut, startGUI)
	if code == exitOK && (out.err != nil || errOut.err != nil) {
		return exitError
	}
	return code
}

// printer writes formatted text and remembers the first write error, so that
// run can turn it into a failing exit code.
type printer struct {
	w   io.Writer
	err error
}

// printf writes the formatted text unless an earlier write failed.
func (p *printer) printf(format string, a ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, format, a...)
}

// dispatch runs the command and returns its exit code.
func dispatch(ctx context.Context, args []string, stdin io.Reader, stdout, stderr *printer, startGUI func() error) int {
	if len(args) == 0 {
		if err := ctx.Err(); err != nil {
			stderr.printf("radio-tutor: %v\n", err)
			return exitError
		}
		if err := startGUI(); err != nil {
			stderr.printf("radio-tutor: %v\n", err)
			return exitError
		}
		return exitOK
	}
	switch args[0] {
	case "validate", "pack":
		stderr.printf("radio-tutor %s: not implemented yet\n", args[0])
		return exitUsage
	case "reset-password":
		return resetPassword(ctx, args[1:], resetDeps{stdin: stdin, stdout: stdout, stderr: stderr, echo: terminalEcho})
	case "version":
		if len(args) > 1 {
			stderr.printf("radio-tutor version: takes no arguments\n")
			return exitUsage
		}
		info, ok := debug.ReadBuildInfo()
		stdout.printf("%s\n", versionOf(info, ok))
		return exitOK
	case "help", "-h", "--help":
		stdout.printf("%s", usage)
		return exitOK
	default:
		stderr.printf("radio-tutor: unknown command %q\n\n%s", args[0], usage)
		return exitUsage
	}
}

// versionOf returns the module version recorded in the build information, or
// "unknown" when the binary has none.
func versionOf(info *debug.BuildInfo, ok bool) string {
	if !ok || info == nil || info.Main.Version == "" {
		return "unknown"
	}
	return info.Main.Version
}

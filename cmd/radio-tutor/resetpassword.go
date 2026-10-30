package main

import (
	"bufio"
	"context"
	"io"
	"strings"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
)

// echoControl turns the echo of a terminal off while a password is typed.
type echoControl interface {
	// Off turns the echo off and returns the function that restores it. The
	// function may be called more than once.
	Off() (restore func() error, err error)
}

// resetDeps is what resetPassword needs from its surroundings, so that tests can
// replace the terminal and the cost of the password hash.
type resetDeps struct {
	stdin          io.Reader
	stdout, stderr *printer
	// echo returns the echo control of stdin, or nil when stdin is not a terminal.
	echo func(stdin io.Reader) echoControl
	// profileOpts are passed to profile.Open (tests use few hash iterations).
	profileOpts []profile.Option
}

const resetUsage = "usage: radio-tutor reset-password <username>\n"

// resetPassword runs `radio-tutor reset-password <username>` (SPEC §14): it
// takes the single-instance lock, reads the new password twice and saves it. It
// returns the exit code.
func resetPassword(ctx context.Context, args []string, d resetDeps) int {
	if len(args) != 1 {
		d.stderr.printf("%s", resetUsage)
		return exitUsage
	}
	username := args[0]

	paths, err := storage.Resolve()
	if err != nil {
		d.stderr.printf("radio-tutor reset-password: %v\n", err)
		return exitError
	}
	unlock, err := storage.Lock(paths)
	if err != nil {
		if appErr, ok := errors.FromError(err); ok && appErr.Tag == "AlreadyRunning" {
			d.stderr.printf("Radio Tutor is already running\n")
			return exitUsage
		}
		d.stderr.printf("radio-tutor reset-password: %v\n", err)
		return exitError
	}
	defer unlock()

	store, err := profile.Open(ctx, paths.ProfilesFile(), d.profileOpts...)
	if err != nil {
		d.stderr.printf("radio-tutor reset-password: %v\n", err)
		return exitError
	}
	p, err := store.Get(username)
	if err != nil {
		d.stderr.printf("radio-tutor reset-password: no user named %q\n", username)
		return exitError
	}

	var echo echoControl
	if d.echo != nil {
		echo = d.echo(d.stdin)
	}
	password, err := readNewPassword(ctx, d.stdin, echo, d.stderr)
	if err != nil {
		d.stderr.printf("radio-tutor reset-password: %v\n", err)
		return exitError
	}
	if err := store.SetPassword(ctx, p.Username, password); err != nil {
		d.stderr.printf("radio-tutor reset-password: %v\n", err)
		return exitError
	}
	d.stdout.printf("The password of %s was changed.\n", p.Username)
	return exitOK
}

// readNewPassword reads the new password and its repetition from stdin. On a
// terminal (echo is not nil) it asks for them with prompts on stderr and keeps
// the echo off while they are typed, restoring it when it returns, also when ctx
// is cancelled (an interrupt). Otherwise it reads one line per password. Tags:
// EchoOffFailed, EchoRestoreFailed, PasswordsDiffer, and those of readLine.
func readNewPassword(ctx context.Context, stdin io.Reader, echo echoControl, stderr *printer) (password string, err error) {
	if echo != nil {
		restore, offErr := echo.Off()
		if offErr != nil {
			return "", errors.NewErr("EchoOffFailed", offErr)
		}
		defer func() {
			if restoreErr := restore(); restoreErr != nil && err == nil {
				password, err = "", errors.NewErr("EchoRestoreFailed", restoreErr)
			}
		}()
	}
	lines := bufio.NewReader(stdin)
	var entered [2]string
	for i, prompt := range []string{"New password: ", "Repeat the password: "} {
		if echo != nil {
			stderr.printf("%s", prompt)
		}
		line, readErr := readLine(ctx, lines)
		if echo != nil {
			stderr.printf("\n") // the terminal did not echo the Enter key
		}
		if readErr != nil {
			return "", readErr
		}
		entered[i] = line
	}
	if entered[0] != entered[1] {
		return "", errors.New("PasswordsDiffer", "the two passwords are not the same")
	}
	return entered[0], nil
}

// readLine reads one line without its line ending. It gives up when ctx is
// cancelled; the read itself cannot be interrupted, so it ends with the process.
// Tags: PasswordReadCancelled, PasswordInputEnded (no line to read),
// PasswordReadFailed.
func readLine(ctx context.Context, r *bufio.Reader) (string, error) {
	type result struct {
		line string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		line, err := r.ReadString('\n')
		done <- result{line, err}
	}()
	select {
	case <-ctx.Done():
		return "", errors.NewErr("PasswordReadCancelled", ctx.Err())
	case res := <-done:
		switch {
		case res.err == nil, errors.Is(res.err, io.EOF) && res.line != "":
			return strings.TrimRight(res.line, "\r\n"), nil
		case errors.Is(res.err, io.EOF):
			return "", errors.NewErr("PasswordInputEnded", res.err)
		default:
			return "", errors.NewErr("PasswordReadFailed", res.err)
		}
	}
}

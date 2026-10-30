package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
)

// fewIterations keeps the tests fast; verification reads the count from the hash.
var fewIterations = []profile.Option{profile.WithIterations(2)}

// fakeEcho records how the terminal is used.
type fakeEcho struct {
	offCalls, restoreCalls int
	offErr, restoreErr     error
	onRestore              func()
}

func (f *fakeEcho) Off() (func() error, error) {
	f.offCalls++
	if f.offErr != nil {
		return nil, f.offErr
	}
	return func() error {
		f.restoreCalls++
		if f.onRestore != nil {
			f.onRestore()
		}
		return f.restoreErr
	}, nil
}

// resetEnv is an isolated storage with two profiles.
type resetEnv struct {
	paths storage.Paths
	store *profile.Store
}

func newResetEnv(t *testing.T) resetEnv {
	t.Helper()
	paths := storagetest.Isolate(t)
	store, err := profile.Open(context.Background(), paths.ProfilesFile(), fewIterations...)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := store.Create(ctx, "mama", "Mama", profile.RoleAdmin, "old-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(ctx, "kid", "Kid", profile.RoleStudent, "oldp"); err != nil {
		t.Fatal(err)
	}
	return resetEnv{paths: paths, store: store}
}

// login opens the file again (as the next run of the app would) and logs in.
func (e resetEnv) login(t *testing.T, username, password string) error {
	t.Helper()
	s, err := profile.Open(context.Background(), e.paths.ProfilesFile())
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Login(username, password)
	return err
}

// runReset runs resetPassword with the given stdin and echo control.
func runReset(ctx context.Context, args []string, stdin io.Reader, echo echoControl) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = resetPassword(ctx, args, resetDeps{
		stdin:       stdin,
		stdout:      &printer{w: &out},
		stderr:      &printer{w: &errOut},
		echo:        func(io.Reader) echoControl { return echo },
		profileOpts: fewIterations,
	})
	return code, out.String(), errOut.String()
}

func TestResetPasswordNotATerminal(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantCode   int
		wantStderr string
		loginUser  string // checked when set
		loginPass  string
		loginOK    bool
	}{
		{name: "admin password", args: []string{"mama"}, stdin: "new-password\nnew-password\n", wantCode: 0, loginUser: "mama", loginPass: "new-password", loginOK: true},
		{name: "old password stops working", args: []string{"mama"}, stdin: "new-password\nnew-password\n", wantCode: 0, loginUser: "mama", loginPass: "old-password", loginOK: false},
		{name: "windows line endings", args: []string{"mama"}, stdin: "new-password\r\nnew-password\r\n", wantCode: 0, loginUser: "mama", loginPass: "new-password", loginOK: true},
		{name: "no newline after the last line", args: []string{"mama"}, stdin: "new-password\nnew-password", wantCode: 0, loginUser: "mama", loginPass: "new-password", loginOK: true},
		{name: "username in upper case", args: []string{"MAMA"}, stdin: "new-password\nnew-password\n", wantCode: 0, loginUser: "mama", loginPass: "new-password", loginOK: true},
		{name: "spaces are part of the password", args: []string{"mama"}, stdin: " pass word \n pass word \n", wantCode: 0, loginUser: "mama", loginPass: " pass word ", loginOK: true},
		{name: "student minimum is 4", args: []string{"kid"}, stdin: "abcd\nabcd\n", wantCode: 0, loginUser: "kid", loginPass: "abcd", loginOK: true},
		{name: "admin minimum is 8", args: []string{"mama"}, stdin: "1234567\n1234567\n", wantCode: 1, wantStderr: "at least 8 characters", loginUser: "mama", loginPass: "old-password", loginOK: true},
		{name: "student under the minimum", args: []string{"kid"}, stdin: "abc\nabc\n", wantCode: 1, wantStderr: "at least 4 characters", loginUser: "kid", loginPass: "oldp", loginOK: true},
		{name: "empty password", args: []string{"kid"}, stdin: "\n\n", wantCode: 1, wantStderr: "at least 4 characters"},
		{name: "passwords differ", args: []string{"mama"}, stdin: "new-password\nother-password\n", wantCode: 1, wantStderr: "not the same", loginUser: "mama", loginPass: "old-password", loginOK: true},
		{name: "no input", args: []string{"mama"}, stdin: "", wantCode: 1, wantStderr: "EOF"},
		{name: "one line only", args: []string{"mama"}, stdin: "new-password\n", wantCode: 1, wantStderr: "EOF"},
		{name: "unknown user", args: []string{"nobody"}, stdin: "new-password\nnew-password\n", wantCode: 1, wantStderr: `no user named "nobody"`},
		{name: "no username", args: nil, wantCode: 2, wantStderr: "usage: radio-tutor reset-password <username>"},
		{name: "two usernames", args: []string{"mama", "kid"}, wantCode: 2, wantStderr: "usage: radio-tutor reset-password <username>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newResetEnv(t)
			code, stdout, stderr := runReset(context.Background(), tt.args, strings.NewReader(tt.stdin), nil)
			if code != tt.wantCode {
				t.Errorf("exit code %d, want %d (stderr %q)", code, tt.wantCode, stderr)
			}
			if !strings.Contains(stderr, tt.wantStderr) {
				t.Errorf("stderr %q does not contain %q", stderr, tt.wantStderr)
			}
			if tt.wantCode == 0 && !strings.Contains(stdout, "was changed") {
				t.Errorf("stdout %q", stdout)
			}
			if tt.wantCode != 0 && stdout != "" {
				t.Errorf("stdout %q on failure", stdout)
			}
			if strings.Contains(stdout+stderr, "new-password") {
				t.Errorf("a password was printed: %q %q", stdout, stderr)
			}
			if strings.Contains(stderr, "New password:") {
				t.Errorf("prompts are for terminals only, stderr %q", stderr)
			}
			if tt.loginUser != "" {
				if err := env.login(t, tt.loginUser, tt.loginPass); (err == nil) != tt.loginOK {
					t.Errorf("login %s: %v, want ok=%v", tt.loginUser, err, tt.loginOK)
				}
			}
		})
	}
}

func TestResetPasswordRefusesWhileRunning(t *testing.T) {
	env := newResetEnv(t)
	unlock, err := storage.Lock(env.paths)
	if err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runReset(context.Background(), []string{"mama"}, strings.NewReader("new-password\nnew-password\n"), nil)
	if code != 2 || !strings.Contains(stderr, "Radio Tutor is already running") {
		t.Errorf("exit code %d, stderr %q", code, stderr)
	}
	if err := env.login(t, "mama", "old-password"); err != nil {
		t.Errorf("the password changed while the app was running: %v", err)
	}
	unlock()
	if code, _, stderr := runReset(context.Background(), []string{"mama"}, strings.NewReader("new-password\nnew-password\n"), nil); code != 0 {
		t.Errorf("after the lock was released: exit code %d, stderr %q", code, stderr)
	}
}

func TestResetPasswordReleasesTheLock(t *testing.T) {
	env := newResetEnv(t)
	runReset(context.Background(), []string{"nobody"}, strings.NewReader(""), nil)
	unlock, err := storage.Lock(env.paths)
	if err != nil {
		t.Fatalf("the lock stayed held: %v", err)
	}
	unlock()
}

func TestResetPasswordStorageFailures(t *testing.T) {
	t.Run("profiles file with a newer format", func(t *testing.T) {
		paths := storagetest.Isolate(t)
		if err := os.MkdirAll(paths.Data, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(paths.ProfilesFile(), []byte(`{"format":2,"profiles":[]}`), 0o600); err != nil {
			t.Fatal(err)
		}
		code, _, stderr := runReset(context.Background(), []string{"mama"}, strings.NewReader(""), nil)
		if code != 1 || !strings.Contains(stderr, "unsupported format") {
			t.Errorf("exit code %d, stderr %q", code, stderr)
		}
	})
	t.Run("no profiles at all", func(t *testing.T) {
		storagetest.Isolate(t)
		code, _, stderr := runReset(context.Background(), []string{"mama"}, strings.NewReader(""), nil)
		if code != 1 || !strings.Contains(stderr, `no user named "mama"`) {
			t.Errorf("exit code %d, stderr %q", code, stderr)
		}
	})
	t.Run("no home for the folders", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("XDG_STATE_HOME", "")
		t.Setenv("HOME", "")
		code, _, stderr := runReset(context.Background(), []string{"mama"}, strings.NewReader(""), nil)
		if code != 1 || stderr == "" {
			t.Errorf("exit code %d, stderr %q", code, stderr)
		}
	})
	t.Run("state folder that cannot be made", func(t *testing.T) {
		storagetest.SkipOnWindows(t, "a path through a file reports not found, not not-a-folder")
		paths := storagetest.Isolate(t)
		if err := os.WriteFile(paths.State, nil, 0o600); err != nil { // a file where the folder goes
			t.Fatal(err)
		}
		code, _, stderr := runReset(context.Background(), []string{"mama"}, strings.NewReader(""), nil)
		if code != 1 || stderr == "" || strings.Contains(stderr, "already running") {
			t.Errorf("exit code %d, stderr %q", code, stderr)
		}
	})
}

func TestResetPasswordTerminal(t *testing.T) {
	env := newResetEnv(t)
	echo := &fakeEcho{}
	var stdin io.Reader = strings.NewReader("new-password\nnew-password\n")
	// The echo must be off exactly while the passwords are read.
	echo.onRestore = func() {
		if echo.offCalls != 1 {
			t.Errorf("echo restored after %d Off calls", echo.offCalls)
		}
	}
	code, stdout, stderr := runReset(context.Background(), []string{"mama"}, stdin, echo)
	if code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr)
	}
	if echo.offCalls != 1 || echo.restoreCalls != 1 {
		t.Errorf("echo off %d times, restored %d times", echo.offCalls, echo.restoreCalls)
	}
	if want := "New password: \nRepeat the password: \n"; stderr != want {
		t.Errorf("prompts %q, want %q", stderr, want)
	}
	if !strings.Contains(stdout, "was changed") {
		t.Errorf("stdout %q", stdout)
	}
	if err := env.login(t, "mama", "new-password"); err != nil {
		t.Errorf("login: %v", err)
	}
}

func TestResetPasswordTerminalRestoresOnFailure(t *testing.T) {
	tests := []struct {
		name  string
		stdin string
	}{
		{"passwords differ", "new-password\nother-password\n"},
		{"input ends", "new-password\n"},
		{"too short", "abc\nabc\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newResetEnv(t)
			echo := &fakeEcho{}
			code, _, _ := runReset(context.Background(), []string{"mama"}, strings.NewReader(tt.stdin), echo)
			if code != 1 {
				t.Errorf("exit code %d, want 1", code)
			}
			if echo.offCalls != 1 || echo.restoreCalls != 1 {
				t.Errorf("echo off %d times, restored %d times", echo.offCalls, echo.restoreCalls)
			}
		})
	}
}

func TestResetPasswordUnknownUserDoesNotTouchTheTerminal(t *testing.T) {
	newResetEnv(t)
	echo := &fakeEcho{}
	if code, _, _ := runReset(context.Background(), []string{"nobody"}, strings.NewReader(""), echo); code != 1 {
		t.Errorf("exit code %d", code)
	}
	if echo.offCalls != 0 {
		t.Errorf("echo was switched for an unknown user")
	}
}

func TestResetPasswordInterrupt(t *testing.T) {
	env := newResetEnv(t)
	pr, pw := io.Pipe() // a terminal where nobody types
	t.Cleanup(func() { _ = pw.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	echo := &fakeEcho{}
	echo.onRestore = func() {} // restore must run even though the read never returned

	finished := make(chan int, 1)
	var stderr string
	go func() {
		code, _, errOut := runReset(ctx, []string{"mama"}, pr, echo)
		stderr = errOut
		finished <- code
	}()
	time.Sleep(50 * time.Millisecond)
	cancel() // what SIGINT does through signal.NotifyContext
	select {
	case code := <-finished:
		if code != 1 || !strings.Contains(stderr, "context canceled") {
			t.Errorf("exit code %d, stderr %q", code, stderr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reset-password did not return after the interrupt")
	}
	if echo.offCalls != 1 || echo.restoreCalls != 1 {
		t.Errorf("echo off %d times, restored %d times", echo.offCalls, echo.restoreCalls)
	}
	if err := env.login(t, "mama", "old-password"); err != nil {
		t.Errorf("the password changed after an interrupt: %v", err)
	}
}

// errReader fails every read.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestReadNewPasswordRoutes(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name      string
		ctx       context.Context
		stdin     io.Reader
		echo      echoControl
		wantTag   string
		wantRoute string
	}{
		{"input ends", context.Background(), strings.NewReader(""), nil, "PasswordInputEnded", "readNewPassword->readLine.PasswordInputEnded"},
		{"second line missing", context.Background(), strings.NewReader("abc\n"), nil, "PasswordInputEnded", "readNewPassword->readLine.PasswordInputEnded"},
		{"read fails", context.Background(), errReader{}, nil, "PasswordReadFailed", "readNewPassword->readLine.PasswordReadFailed"},
		{"cancelled", cancelled, blockingReader{}, nil, "PasswordReadCancelled", "readNewPassword->readLine.PasswordReadCancelled"},
		{"passwords differ", context.Background(), strings.NewReader("abc\nabd\n"), nil, "PasswordsDiffer", "readNewPassword.PasswordsDiffer"},
		{"echo cannot be turned off", context.Background(), strings.NewReader("a\na\n"), &fakeEcho{offErr: io.ErrClosedPipe}, "EchoOffFailed", "readNewPassword.EchoOffFailed"},
		{"echo cannot be restored", context.Background(), strings.NewReader("a\na\n"), &fakeEcho{restoreErr: io.ErrClosedPipe}, "EchoRestoreFailed", "readNewPassword.func1.EchoRestoreFailed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			password, err := readNewPassword(tt.ctx, tt.stdin, tt.echo, &printer{w: io.Discard})
			if err == nil {
				t.Fatalf("expected %s, got password %q", tt.wantTag, password)
			}
			appErr, ok := errors.FromError(err)
			if !ok {
				t.Fatalf("expected *errors.Error, got %T", err)
			}
			if appErr.Tag != tt.wantTag || !appErr.HasRoute(tt.wantRoute) {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
			if password != "" {
				t.Errorf("a password %q came back with the error", password)
			}
		})
	}
}

// blockingReader blocks forever.
type blockingReader struct{}

func (blockingReader) Read([]byte) (int, error) { select {} }

func TestReadNewPasswordSucceeds(t *testing.T) {
	got, err := readNewPassword(context.Background(), strings.NewReader("abc\nabc\nrest"), nil, &printer{w: io.Discard})
	if err != nil || got != "abc" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestReadLineKeepsBufferedInput(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("one\ntwo\n"))
	for _, want := range []string{"one", "two"} {
		got, err := readLine(context.Background(), r)
		if err != nil || got != want {
			t.Errorf("got %q, %v; want %q", got, err, want)
		}
	}
}

func TestTerminalEchoNotATerminal(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pr.Close(); _ = pw.Close() })
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = devNull.Close() })
	for name, r := range map[string]io.Reader{
		"string reader": strings.NewReader("x"),
		"pipe":          pr,
		"/dev/null":     devNull,
		"nil":           nil,
	} {
		if got := terminalEcho(r); got != nil {
			t.Errorf("%s: terminalEcho = %v, want nil", name, got)
		}
	}
}

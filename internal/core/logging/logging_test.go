package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
)

func routeOf(t *testing.T, err error) *errors.Error {
	t.Helper()
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T: %v", err, err)
	}
	return appErr
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(b)
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func TestNewLevels(t *testing.T) {
	tests := []struct {
		name      string
		debug     bool
		wantDebug bool
	}{
		{"info level drops debug", false, false},
		{"debug level keeps debug", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := storagetest.Isolate(t)
			logger, closer, err := New(paths, tt.debug)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			logger.Debug("debug line")
			logger.Info("info line", "key", "value")
			if err := closer.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}

			lines := strings.Split(strings.TrimSpace(readFile(t, paths.LogFile())), "\n")
			var entries []map[string]any
			for _, l := range lines {
				var e map[string]any
				if err := json.Unmarshal([]byte(l), &e); err != nil {
					t.Fatalf("line %q is not JSON: %v", l, err)
				}
				entries = append(entries, e)
			}
			wantLines := 1
			if tt.wantDebug {
				wantLines = 2
			}
			if len(entries) != wantLines {
				t.Fatalf("%d log lines, want %d: %v", len(entries), wantLines, lines)
			}
			last := entries[len(entries)-1]
			if last["msg"] != "info line" || last["key"] != "value" || last["level"] != "INFO" {
				t.Errorf("last entry = %v", last)
			}
			info, err := os.Stat(paths.LogFile())
			if err != nil || info.Mode().Perm() != storage.FileMode {
				t.Errorf("log file: %v, %v", info, err)
			}
			dirInfo, err := os.Stat(paths.State)
			if err != nil || dirInfo.Mode().Perm() != storage.DirMode {
				t.Errorf("state folder: %v, %v", dirInfo, err)
			}
		})
	}
}

func TestNewLogsErrorsWithTagAndRoute(t *testing.T) {
	paths := storagetest.Isolate(t)
	logger, closer, err := New(paths, false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	logger.Error("failed", "err", errors.New("SomethingBroke", "it broke", "path", "/x"))
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
	content := readFile(t, paths.LogFile())
	for _, want := range []string{`"Tag":"SomethingBroke"`, `"level":"ERROR"`, `"path":"/x"`} {
		if !strings.Contains(content, want) {
			t.Errorf("log lacks %s: %s", want, content)
		}
	}
}

func TestNewAppendsToExistingLog(t *testing.T) {
	paths := storagetest.Isolate(t)
	for _, msg := range []string{"first run", "second run"} {
		logger, closer, err := New(paths, false)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		logger.Info(msg)
		if err := closer.Close(); err != nil {
			t.Fatal(err)
		}
	}
	content := readFile(t, paths.LogFile())
	if !strings.Contains(content, "first run") || !strings.Contains(content, "second run") {
		t.Errorf("log = %s", content)
	}
}

func TestNewFailsWhenStateFolderCannotBeMade(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, storage.FileMode); err != nil {
		t.Fatal(err)
	}
	logger, closer, err := New(storage.Paths{State: filepath.Join(file, "state")}, false)
	if err == nil || logger != nil || closer != nil {
		t.Fatalf("got %v, %v, %v; want only an error", logger, closer, err)
	}
	if appErr := routeOf(t, err); !appErr.HasRoute("New->NewRotatingWriter->EnsureDir.DirCreateFailed") {
		t.Errorf("unexpected failure path: %s", appErr.Route())
	}
}

func TestRotation(t *testing.T) {
	tests := []struct {
		name   string
		files  int
		writes int // writes of 4 bytes, "aaa\n", "bbb\n", ... max size 10
		want   map[string]string
	}{
		{
			name: "under the limit nothing rotates", files: 3, writes: 2,
			want: map[string]string{"log": "aaa\nbbb\n"},
		},
		{
			name: "first rotation", files: 3, writes: 3,
			want: map[string]string{"log": "ccc\n", "log.1": "aaa\nbbb\n"},
		},
		{
			name: "three files are kept", files: 3, writes: 7,
			want: map[string]string{"log": "ggg\n", "log.1": "eee\nfff\n", "log.2": "ccc\nddd\n"},
		},
		{
			name: "one file only", files: 1, writes: 5,
			want: map[string]string{"log": "eee\n"},
		},
		{
			name: "two files", files: 2, writes: 5,
			want: map[string]string{"log": "eee\n", "log.1": "ccc\nddd\n"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "log")
			w, err := NewRotatingWriter(path, 10, tt.files)
			if err != nil {
				t.Fatalf("NewRotatingWriter: %v", err)
			}
			for i := 0; i < tt.writes; i++ {
				line := strings.Repeat(string(rune('a'+i)), 3) + "\n"
				if n, err := w.Write([]byte(line)); err != nil || n != len(line) {
					t.Fatalf("Write %d: %d, %v", i, n, err)
				}
			}
			if err := w.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
			got := map[string]string{}
			for _, n := range names(t, dir) {
				got[n] = readFile(t, filepath.Join(dir, n))
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("files = %q, want %q", got, tt.want)
			}
			for n := range got {
				info, err := os.Stat(filepath.Join(dir, n))
				if err != nil || info.Mode().Perm() != storage.FileMode {
					t.Errorf("%s: %v, %v", n, info, err)
				}
			}
		})
	}
}

func TestRotationContinuesAfterReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log")
	if err := os.WriteFile(path, []byte("12345678"), storage.FileMode); err != nil {
		t.Fatal(err)
	}
	w, err := NewRotatingWriter(path, 10, 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("abc")); err != nil { // 8 + 3 > 10
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path+".1"); got != "12345678" {
		t.Errorf("log.1 = %q", got)
	}
	if got := readFile(t, path); got != "abc" {
		t.Errorf("log = %q", got)
	}
}

func TestOversizedWriteIsNotSplit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log")
	w, err := NewRotatingWriter(path, 5, 3)
	if err != nil {
		t.Fatal(err)
	}
	big := strings.Repeat("x", 20)
	for i := 0; i < 2; i++ {
		if n, err := w.Write([]byte(big)); err != nil || n != len(big) {
			t.Fatalf("Write: %d, %v", n, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != big {
		t.Errorf("log = %q", got)
	}
	if got := readFile(t, path+".1"); got != big {
		t.Errorf("log.1 = %q", got)
	}
}

func TestConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log")
	w, err := NewRotatingWriter(path, 1000, 3)
	if err != nil {
		t.Fatal(err)
	}
	const goroutines, perGoroutine = 16, 50
	line := []byte(strings.Repeat("z", 39) + "\n") // 40 bytes
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				if _, err := w.Write(line); err != nil {
					t.Errorf("Write: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// 800 lines of 40 bytes cannot fit in 3 files of at most 1000 bytes, so
	// the oldest were dropped, but every file holds only whole lines and no
	// file is larger than the limit.
	if got := names(t, dir); !reflect.DeepEqual(got, []string{"log", "log.1", "log.2"}) {
		t.Fatalf("files = %v", got)
	}
	for _, n := range []string{"log", "log.1", "log.2"} {
		content := readFile(t, filepath.Join(dir, n))
		if len(content) > 1000 {
			t.Errorf("%s has %d bytes, over the limit", n, len(content))
		}
		if len(content)%40 != 0 || bytes.Contains([]byte(content), []byte("\n\n")) {
			t.Errorf("%s holds a torn line", n)
		}
	}
}

func TestConcurrentLogger(t *testing.T) {
	paths := storagetest.Isolate(t)
	logger, closer, err := New(paths, false)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				logger.Info("line", slog.Int("g", g), slog.Int("i", i))
			}
		}()
	}
	wg.Wait()
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(readFile(t, paths.LogFile())), "\n")
	if len(lines) != 800 {
		t.Fatalf("%d lines, want 800", len(lines))
	}
	for _, l := range lines {
		if !json.Valid([]byte(l)) {
			t.Fatalf("torn line %q", l)
		}
	}
}

func TestWriteAfterClose(t *testing.T) {
	w, err := NewRotatingWriter(filepath.Join(t.TempDir(), "log"), 10, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	_, err = w.Write([]byte("x"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if appErr := routeOf(t, err); !appErr.HasRoute("RotatingWriter.Write.LogClosed") {
		t.Errorf("unexpected failure path: %s", appErr.Route())
	}
}

func TestOpenFailures(t *testing.T) {
	t.Run("log path is a folder", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "log")
		if err := os.Mkdir(path, storage.DirMode); err != nil {
			t.Fatal(err)
		}
		_, err := NewRotatingWriter(path, 10, 3)
		if err == nil {
			t.Fatal("expected an error")
		}
		if appErr := routeOf(t, err); !appErr.HasRoute("NewRotatingWriter->openLog.LogOpenFailed") {
			t.Errorf("unexpected failure path: %s", appErr.Route())
		}
	})
	t.Run("reopen after a failed rotation fails, then recovers", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "log")
		w, err := NewRotatingWriter(path, 10, 2)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("aaaaaaaa")); err != nil {
			t.Fatal(err)
		}
		// A non-empty folder as the oldest file makes the rotation fail and
		// leaves the writer without an open file.
		oldest := path + ".1"
		if err := os.MkdirAll(filepath.Join(oldest, "inner"), storage.DirMode); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("bbbbbbbb")); err == nil {
			t.Fatal("expected the rotation to fail")
		}
		// Now the log path itself cannot be opened.
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, storage.DirMode); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(oldest); err != nil {
			t.Fatal(err)
		}
		w.size = 0 // the planted folder has no content to rotate
		_, err = w.Write([]byte("bbbbbbbb"))
		if err == nil {
			t.Fatal("expected an error")
		}
		if appErr := routeOf(t, err); !appErr.HasRoute("RotatingWriter.Write->openLog.LogOpenFailed") {
			t.Errorf("unexpected failure path: %s", appErr.Route())
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("cc")); err != nil {
			t.Fatalf("the writer did not recover: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, path); got != "cc" {
			t.Errorf("log = %q", got)
		}
	})
}

func TestRotateFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log")
	w, err := NewRotatingWriter(path, 10, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("aaaaaaaa")); err != nil {
		t.Fatal(err)
	}
	// The oldest file ("log.1" with two files kept) is a non-empty folder, so
	// removing it fails.
	oldest := path + ".1"
	if err := os.MkdirAll(filepath.Join(oldest, "inner"), storage.DirMode); err != nil {
		t.Fatal(err)
	}
	_, err = w.Write([]byte("bbbbbbbb"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if appErr := routeOf(t, err); !appErr.HasRoute("RotatingWriter.Write->RotatingWriter.rotate.LogRotateFailed") {
		t.Errorf("unexpected failure path: %s", appErr.Route())
	}
	// Once the obstacle is gone the next write rotates and goes through.
	if err := os.RemoveAll(oldest); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("bbbbbbbb")); err != nil {
		t.Fatalf("the writer did not recover: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != "bbbbbbbb" {
		t.Errorf("log = %q", got)
	}
	if got := readFile(t, path+".1"); got != "aaaaaaaa" {
		t.Errorf("log.1 = %q", got)
	}
}

func TestCloseFailure(t *testing.T) {
	w, err := NewRotatingWriter(filepath.Join(t.TempDir(), "log"), 10, 3)
	if err != nil {
		t.Fatal(err)
	}
	// Close the file behind the writer's back so that its own Close fails.
	if err := w.f.Close(); err != nil {
		t.Fatal(err)
	}
	err = w.Close()
	if err == nil {
		t.Fatal("expected an error")
	}
	if appErr := routeOf(t, err); !appErr.HasRoute("RotatingWriter.Close.LogCloseFailed") {
		t.Errorf("unexpected failure path: %s", appErr.Route())
	}
}

func TestWriteFailure(t *testing.T) {
	w, err := NewRotatingWriter(filepath.Join(t.TempDir(), "log"), 10, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.f.Close(); err != nil { // writes now fail with "file already closed"
		t.Fatal(err)
	}
	_, err = w.Write([]byte("x"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if appErr := routeOf(t, err); !appErr.HasRoute("RotatingWriter.Write.LogWriteFailed") {
		t.Errorf("unexpected failure path: %s", appErr.Route())
	}
}

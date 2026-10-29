package logging

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
)

// RotatingWriter is an io.WriteCloser that appends to a file and rotates it when
// it would grow beyond a size limit. The current file is "path"; the older ones
// are "path.1" (the newest) up to "path.<files-1>". It is safe for concurrent use.
type RotatingWriter struct {
	path    string
	maxSize int64
	files   int

	mu     sync.Mutex
	f      *os.File // nil when the file is not open (closed, or a rotation failed)
	size   int64
	closed bool
}

// NewRotatingWriter opens (creating it and its folder when missing, with the
// storage modes) the log file at path for appending. When a write would take
// the file beyond maxSize bytes, the files are rotated first, and at most files
// files (the current one included, files >= 1) are kept. A single write larger
// than maxSize is never split. It fails with the tags DirCreateFailed and
// LogOpenFailed.
func NewRotatingWriter(path string, maxSize int64, files int) (*RotatingWriter, error) {
	if err := storage.EnsureDir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	w := &RotatingWriter{path: path, maxSize: maxSize, files: max(files, 1)}
	f, size, err := openLog(path)
	if err != nil {
		return nil, err
	}
	w.f, w.size = f, size
	return w, nil
}

func openLog(path string) (*os.File, int64, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, storage.FileMode)
	if err != nil {
		return nil, 0, errors.NewErr("LogOpenFailed", err).SetMetadata("path", path)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close() // the stat error is the one to report
		return nil, 0, errors.NewErr("LogOpenFailed", err).SetMetadata("path", path)
	}
	return f, info.Size(), nil
}

// Write appends p to the log file, rotating first when p does not fit. It fails
// with the tags LogClosed, LogRotateFailed, LogOpenFailed and LogWriteFailed. A
// failed rotation or reopen is tried again by the next write.
func (w *RotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, errors.New("LogClosed", "the log file is closed", "path", w.path)
	}
	if w.size > 0 && w.size+int64(len(p)) > w.maxSize {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	if w.f == nil {
		f, size, err := openLog(w.path)
		if err != nil {
			return 0, err
		}
		w.f, w.size = f, size
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	if err != nil {
		return n, errors.NewErr("LogWriteFailed", err).SetMetadata("path", w.path)
	}
	return n, nil
}

// rotate closes the current file and shifts path.1 to path.2 and so on, dropping
// the oldest file, then path to path.1. The caller holds w.mu. w.f is nil
// afterwards, so that Write opens a fresh file; when a step fails, w.size is
// left as it was, so that the next Write tries the rotation again.
func (w *RotatingWriter) rotate() error {
	if w.f != nil {
		err := w.f.Close()
		w.f = nil
		if err != nil {
			return errors.NewErr("LogRotateFailed", err).SetMetadata("path", w.path)
		}
	}
	if w.files == 1 {
		if err := os.Remove(w.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return errors.NewErr("LogRotateFailed", err).SetMetadata("path", w.path)
		}
		w.size = 0
		return nil
	}
	oldest := w.path + "." + strconv.Itoa(w.files-1)
	if err := os.Remove(oldest); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errors.NewErr("LogRotateFailed", err).SetMetadata("path", oldest)
	}
	for i := w.files - 2; i >= 0; i-- {
		from := w.path
		if i > 0 {
			from = w.path + "." + strconv.Itoa(i)
		}
		to := w.path + "." + strconv.Itoa(i+1)
		if err := os.Rename(from, to); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return errors.NewErr("LogRotateFailed", err).SetMetadata("path", from, "target", to)
		}
	}
	w.size = 0
	return nil
}

// Close closes the log file. Later writes fail with the tag LogClosed; closing
// twice is fine. It fails with the tag LogCloseFailed.
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	if err != nil {
		return errors.NewErr("LogCloseFailed", err).SetMetadata("path", w.path)
	}
	return nil
}

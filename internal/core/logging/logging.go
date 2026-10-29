// Package logging builds Radio Tutor's logger (SPEC §15): slog JSON lines in a
// rotating file in the state folder.
package logging

import (
	"io"
	"log/slog"

	"github.com/rm4n0s/radio-tutor/internal/core/storage"
)

const (
	// MaxFileSize is the size at which the log file is rotated: 10 MiB.
	MaxFileSize = 10 << 20
	// MaxFiles is the number of log files kept, the current one included.
	MaxFiles = 3
)

// New opens the log file of paths (creating its folder) and returns a logger
// that writes JSON lines to it at level Info, or Debug when debug is true. The
// file is rotated at MaxFileSize and MaxFiles files are kept. The io.Closer
// closes the file; the caller closes it when the app ends. It fails with the
// tags of NewRotatingWriter.
func New(paths storage.Paths, debug bool) (*slog.Logger, io.Closer, error) {
	w, err := NewRotatingWriter(paths.LogFile(), MaxFileSize, MaxFiles)
	if err != nil {
		return nil, nil, err
	}
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})), w, nil
}

// Package archive reads, checks, writes and stores cassettes (CASSETTE-FORMAT
// §1, §9 and §10, SPEC §6.2 and §6.3).
//
// # Reading
//
// OpenZip opens a .cassette file with every limit and name rule of
// CASSETTE-FORMAT §9 and gives back an fs.FS over the entries that passed, so
// that everything else (cassette.Load, the scene and audio checks, the
// extraction) reads a zip, a folder and an installed cassette the same way.
// Open does the same for a folder. Both never trust the archive: an entry name
// is checked before the entry is opened, the sizes the archive declares are
// checked, and then every entry is unpacked once into nothing, counting the
// bytes that really come out, before the view is handed to the caller. An
// entry that fails is left out of the view and reported as a problem, so the
// rest of the cassette can still be checked and the author sees every problem
// at once.
//
// # Checking
//
// Validate runs the checks of CASSETTE-FORMAT §10 on an fs.FS and returns a
// Report with every problem (paths are "file#field" for the documents, see the
// problem package). ValidatePath does the same for a path that is a zip or a
// folder, adding the problems of opening it. Rendering an SVG is done by a
// RenderChecker that the caller provides, because the core does not know
// Glutter.
//
// # Writing
//
// Pack writes a folder as a cassette zip: sorted entries, deflate, a fixed
// modification time, so packing the same folder twice gives the same bytes.
//
// # The store
//
// Store keeps the installed cassettes in the cassettes folder of the data
// folder: List, Get, Import (validate, extract into a temporary folder, rename;
// replace an older version after a confirmation), Remove and Recover.
package archive

import (
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/problem"
)

// The limits of CASSETTE-FORMAT §9.
const (
	// MaxZipBytes is the largest .cassette file: 512 MiB.
	MaxZipBytes = 512 << 20
	// MaxEntries is the most entries (files and folders) of an archive.
	MaxEntries = 2000
	// MaxTotalBytes is the most bytes all the entries have together once
	// unpacked: 1 GiB.
	MaxTotalBytes = 1 << 30
	// MaxEntryBytes is the most bytes one entry has once unpacked: 200 MiB.
	MaxEntryBytes = 200 << 20
	// MaxRatio is how many times its compressed size an entry may be once
	// unpacked.
	MaxRatio = 100
	// MaxProblems is the most problems the opening of one archive reports
	// before it says that there are more.
	MaxProblems = 100
)

// The names of the files of a cassette (CASSETTE-FORMAT §1).
const (
	manifestFile   = "cassette.json"
	coverFile      = "cover.svg"
	lessonsDir     = "lessons"
	exercisesDir   = "exercises"
	lessonFile     = "lesson.json"
	sceneFile      = "scene.svg"
	narrationFile  = "narration.ogg"
	exerciseFile   = "exercise.json"
	promptSVGFile  = "prompt.svg"
	promptAudioOgg = "prompt.ogg"
)

// cancelCheck returns an error tagged ArchiveCanceled when ctx is done. what
// says what was going on, for the log.
func cancelCheck(ctx context.Context, what string) error {
	if err := ctx.Err(); err != nil {
		return errors.NewErr("ArchiveCanceled", err).SetMetadata("while", what)
	}
	return nil
}

// ctxReader fails a Read once ctx is done, so that a long copy can be
// cancelled between two chunks.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, errors.NewErr("ArchiveCanceled", err)
	}
	return c.r.Read(p)
}

// display shortens and quotes an entry name for a problem path: a name made of
// the allowed characters is shown as it is (up to 120 characters), anything
// else, such as a name with control characters or a very long one, is quoted.
func display(name string) string {
	const max = 120
	if len(name) <= max && checkChars(name) == "" {
		return name
	}
	if len(name) > max {
		name = name[:max]
	}
	return strconv.QuoteToASCII(name) + "..."
}

// checkChars returns a message when name has a byte outside a-z 0-9 . - _ /.
func checkChars(name string) string {
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '.', c == '-', c == '_', c == '/':
		default:
			return "the name has characters other than a-z 0-9 . - _ /"
		}
	}
	return ""
}

// checkName applies the name rules of CASSETTE-FORMAT §9 to an entry name
// without its trailing slash. It returns "" when the name is fine.
func checkName(name string) string {
	switch {
	case name == "":
		return "the name is empty"
	case strings.HasPrefix(name, "/"):
		return "the name must be relative, it starts with /"
	case strings.Contains(name, "\\"):
		return "the name has a backslash; names use / between parts"
	}
	for _, part := range strings.Split(name, "/") {
		switch part {
		case "..":
			return "the name has a .. part, which would leave the cassette folder"
		case "":
			return "the name has an empty part (//)"
		case ".":
			return "the name has a . part"
		}
	}
	return checkChars(name)
}

// allowedFile reports whether a file with this (valid) name belongs to the
// layout of CASSETTE-FORMAT §1.
func allowedFile(name string) bool {
	parts := strings.Split(name, "/")
	switch len(parts) {
	case 1:
		return name == manifestFile || name == coverFile
	case 3:
		switch parts[0] {
		case lessonsDir:
			return parts[2] == lessonFile || parts[2] == sceneFile || parts[2] == narrationFile
		case exercisesDir:
			return parts[2] == exerciseFile || parts[2] == promptSVGFile || parts[2] == promptAudioOgg
		}
	}
	return false
}

// allowedDir reports whether a folder with this (valid) name belongs to the
// layout of CASSETTE-FORMAT §1: lessons, exercises, and one folder inside each.
func allowedDir(name string) bool {
	parts := strings.Split(name, "/")
	switch len(parts) {
	case 1:
		return name == lessonsDir || name == exercisesDir
	case 2:
		return parts[0] == lessonsDir || parts[0] == exercisesDir
	}
	return false
}

// entryProblem applies the name and layout rules to an entry (folders without
// their trailing slash) and returns the message of the first rule it breaks, or
// "".
func entryProblem(name string, isDir bool) string {
	if msg := checkName(name); msg != "" {
		return msg
	}
	if isDir {
		if !allowedDir(name) {
			return "this folder is not part of a cassette (CASSETTE-FORMAT §1)"
		}
		return ""
	}
	if !allowedFile(name) {
		return "this file is not part of a cassette (CASSETTE-FORMAT §1)"
	}
	return ""
}

// collector collects the problems of opening an archive and stops listing at
// MaxProblems, counting the rest.
type collector struct {
	ps   problem.List
	more int
}

func (c *collector) errorf(path, format string, args ...any) {
	if len(c.ps) >= MaxProblems {
		c.more++
		return
	}
	c.ps.Errorf(path, format, args...)
}

// problems returns the collected problems, with a closing one when some were
// left out.
func (c *collector) problems() []problem.Problem {
	out := append(problem.List(nil), c.ps...)
	if c.more > 0 {
		out.Errorf("", "%d more problems were not listed", c.more)
	}
	return out
}

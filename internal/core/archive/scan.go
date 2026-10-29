package archive

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/problem"
)

// scan applies the rules of CASSETTE-FORMAT §9 that do not depend on where the
// entries come from (a zip or a folder) and builds the view of the entries
// that passed.
type scan struct {
	c     collector
	tree  *treeFS
	seen  map[string]bool
	count int
	total int64
	// fatal is set when the whole archive is refused (too many entries, too
	// many bytes); the view is then empty and fatalMsg is its only problem.
	fatal    bool
	fatalMsg string
}

func newScan() *scan {
	return &scan{tree: newTree(), seen: map[string]bool{}}
}

// refuse refuses the whole archive; the first reason is the one kept.
func (s *scan) refuse(format string, args ...any) {
	if !s.fatal {
		s.fatalMsg = fmt.Sprintf(format, args...)
	}
	s.fatal = true
}

// next counts one more entry. It returns false, and the archive is refused as
// a whole, when there are more than MaxEntries.
func (s *scan) next() bool {
	s.count++
	if s.count > MaxEntries {
		s.refuse("the archive has more than %d entries, which is the limit", MaxEntries)
		return false
	}
	return true
}

// admit checks one entry (already counted with next) whose name has no trailing slash and whose size is
// what it unpacks to. It returns false when the entry is left out of the
// view; the problem has been recorded.
func (s *scan) admit(name string, isDir bool, size int64) bool {
	shown := display(name)
	if msg := entryProblem(name, isDir); msg != "" {
		s.c.errorf(shown, "%s", msg)
		return false
	}
	if s.seen[name] {
		s.c.errorf(shown, "is listed twice")
		return false
	}
	s.seen[name] = true
	if isDir {
		return true
	}
	if size > MaxEntryBytes {
		s.c.errorf(shown, "is %d bytes unpacked; the limit for one file is %d bytes (200 MiB)", size, MaxEntryBytes)
		return false
	}
	s.total += size
	if s.total > MaxTotalBytes {
		s.refuse("the files are more than %d bytes (1 GiB) unpacked together, which is the limit", MaxTotalBytes)
		return false
	}
	return true
}

// result returns the view (empty when the archive was refused as a whole) and
// the problems.
func (s *scan) result() (*treeFS, []problem.Problem, bool) {
	if s.fatal {
		return newTree(), problem.List{problem.Errorf("", "%s", s.fatalMsg)}, true
	}
	return s.tree, s.c.problems(), false
}

// scanFS checks every entry of fsys with the rules of CASSETTE-FORMAT §9 (a
// folder has no compression, so there are no ratios) and returns a view of the
// entries that passed. A symbolic link or any file that is not a plain file or
// a folder is a problem, as is a file or folder that is not part of the layout
// of §1. The error is for a failure to read fsys or a cancelled context.
func scanFS(ctx context.Context, fsys fs.FS) (*treeFS, []problem.Problem, bool, error) {
	s := newScan()
	err := fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return errors.NewErr("ArchiveFolderReadFailed", err).SetMetadata("path", name)
		}
		if err := cancelCheck(ctx, "scanning"); err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		if !s.next() {
			return fs.SkipAll
		}
		if d.Type()&^fs.ModeDir != 0 {
			s.c.errorf(display(name), "is a symbolic link or a special file; only plain files and folders are allowed")
			return nil
		}
		if d.IsDir() {
			if !s.admit(name, true, 0) {
				return fs.SkipDir
			}
			s.tree.addDir(name)
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return errors.NewErr("ArchiveFolderReadFailed", err).SetMetadata("path", name)
		}
		if !s.admit(name, false, info.Size()) {
			return nil
		}
		s.tree.addFile(name, &treeEntry{size: info.Size(), mod: info.ModTime(), open: func() (io.ReadCloser, error) { return fsys.Open(name) }})
		return nil
	})
	if err != nil {
		return nil, nil, false, err
	}
	t, ps, fatal := s.result()
	return t, ps, fatal, nil
}

// trimDir removes the one trailing slash of a folder entry's name.
func trimDir(name string) (string, bool) {
	if strings.HasSuffix(name, "/") {
		return name[:len(name)-1], true
	}
	return name, false
}

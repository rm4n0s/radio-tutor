package archive

import (
	"io"
	"io/fs"
	"path"
	"slices"
	"time"

	"github.com/rm4n0s/errors"
)

// treeFS is the fs.FS that OpenZip and Open return: a read-only tree of the
// entries that passed the checks. Reading a file is capped to the size that
// was checked, so a file that grows after the check, or a zip that lies, is an
// error and not a way to read more.
type treeFS struct {
	files map[string]*treeEntry
	// dirs maps a folder to its children: name -> is a folder. The root is ".".
	dirs map[string]map[string]bool
}

// treeEntry is a file of a treeFS.
type treeEntry struct {
	size int64
	mod  time.Time
	open func() (io.ReadCloser, error)
}

func newTree() *treeFS {
	return &treeFS{files: map[string]*treeEntry{}, dirs: map[string]map[string]bool{".": {}}}
}

// addDir adds the folder name and its parents.
func (t *treeFS) addDir(name string) {
	for cur := name; cur != "."; cur = path.Dir(cur) {
		parent := path.Dir(cur)
		if t.dirs[cur] == nil {
			t.dirs[cur] = map[string]bool{}
		}
		if t.dirs[parent] == nil {
			t.dirs[parent] = map[string]bool{}
		}
		t.dirs[parent][path.Base(cur)] = true
	}
}

// addFile adds the file name and its parent folders.
func (t *treeFS) addFile(name string, e *treeEntry) {
	t.addDir(path.Dir(name))
	t.files[name] = e
	t.dirs[path.Dir(name)][path.Base(name)] = false
}

// Open implements fs.FS.
func (t *treeFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if e, ok := t.files[name]; ok {
		rc, err := e.open()
		if err != nil {
			return nil, errors.NewErr("ArchiveEntryOpenFailed", err).SetMetadata("path", name)
		}
		return &treeFile{rc: rc, info: entryInfo{name: path.Base(name), size: e.size, mode: 0o444, mod: e.mod},
			lim: capReader{r: rc, name: name, want: e.size}}, nil
	}
	if _, ok := t.dirs[name]; ok {
		return &treeDir{t: t, name: name}, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

// entryInfo is the fs.FileInfo of an entry.
type entryInfo struct {
	name string
	size int64
	mode fs.FileMode
	mod  time.Time
}

func (i entryInfo) Name() string       { return i.name }
func (i entryInfo) Size() int64        { return i.size }
func (i entryInfo) Mode() fs.FileMode  { return i.mode }
func (i entryInfo) ModTime() time.Time { return i.mod }
func (i entryInfo) IsDir() bool        { return i.mode.IsDir() }
func (i entryInfo) Sys() any           { return nil }

// treeFile is an open file of a treeFS.
type treeFile struct {
	rc   io.ReadCloser
	info entryInfo
	lim  capReader
}

func (f *treeFile) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *treeFile) Read(p []byte) (int, error) { return f.lim.Read(p) }
func (f *treeFile) Close() error               { return f.rc.Close() }

// treeDir is an open folder of a treeFS.
type treeDir struct {
	t       *treeFS
	name    string
	entries []fs.DirEntry
	loaded  bool
	pos     int
}

func (d *treeDir) Stat() (fs.FileInfo, error) {
	return entryInfo{name: path.Base(d.name), mode: fs.ModeDir | 0o555}, nil
}

func (d *treeDir) Read([]byte) (int, error) {
	return 0, errors.New("ArchiveReadDirectory", "cannot read a folder as a file", "path", d.name)
}

func (d *treeDir) Close() error { return nil }

// ReadDir implements fs.ReadDirFile.
func (d *treeDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if !d.loaded {
		names := make([]string, 0, len(d.t.dirs[d.name]))
		for child := range d.t.dirs[d.name] {
			names = append(names, child)
		}
		slices.Sort(names)
		for _, child := range names {
			full := path.Join(d.name, child)
			if d.t.dirs[d.name][child] {
				d.entries = append(d.entries, fs.FileInfoToDirEntry(entryInfo{name: child, mode: fs.ModeDir | 0o555}))
				continue
			}
			e := d.t.files[full]
			d.entries = append(d.entries, fs.FileInfoToDirEntry(entryInfo{name: child, size: e.size, mode: 0o444, mod: e.mod}))
		}
		d.loaded = true
	}
	rest := d.entries[d.pos:]
	if n <= 0 {
		d.pos = len(d.entries)
		return slices.Clone(rest), nil
	}
	if len(rest) == 0 {
		return nil, io.EOF
	}
	if n > len(rest) {
		n = len(rest)
	}
	d.pos += n
	return slices.Clone(rest[:n]), nil
}

// capReader reads r and fails when it holds more or fewer bytes than want: the
// size that was checked. It counts the bytes that really come out, whatever the
// archive says.
type capReader struct {
	r    io.Reader
	name string
	want int64
	n    int64
}

func (c *capReader) Read(p []byte) (int, error) {
	if c.n > c.want {
		return 0, errors.New("ArchiveEntryTooLarge", "the entry has more bytes than the "+sizeSource+" says", "path", c.name, "size", c.want)
	}
	if left := c.want - c.n + 1; int64(len(p)) > left {
		p = p[:left]
	}
	n, err := c.r.Read(p)
	c.n += int64(n)
	if c.n > c.want {
		return n - int(c.n-c.want), errors.New("ArchiveEntryTooLarge", "the entry has more bytes than the "+sizeSource+" says", "path", c.name, "size", c.want)
	}
	if err == io.EOF && c.n != c.want {
		return n, errors.New("ArchiveEntryTruncated", "the entry has fewer bytes than the "+sizeSource+" says", "path", c.name, "size", c.want, "read", c.n)
	}
	return n, err
}

const sizeSource = "archive or folder"

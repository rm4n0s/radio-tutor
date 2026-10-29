package archive

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"testing/iotest"
	"time"

	"github.com/rm4n0s/errors"
)

// treeOf builds a treeFS with the files of m.
func treeOf(m map[string]string) *treeFS {
	t := newTree()
	for name, data := range m {
		d := data
		t.addFile(name, &treeEntry{size: int64(len(d)), mod: time.Unix(1e9, 0), open: func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(d)), nil
		}})
	}
	return t
}

func TestTreeFSConformance(t *testing.T) {
	files := map[string]string{
		"cassette.json":              "{}",
		"lessons/a/lesson.json":      "{}",
		"lessons/a/scene.svg":        "<svg/>",
		"lessons/b/lesson.json":      "{}",
		"exercises/e/exercise.json":  "{}",
		"exercises/e/prompt.svg":     "<svg/>",
		"exercises/e2/exercise.json": "{}",
	}
	tree := treeOf(files)
	tree.addDir("lessons/empty")
	if err := fstest.TestFS(tree, "cassette.json", "lessons/a/scene.svg", "lessons/empty", "exercises/e2/exercise.json"); err != nil {
		t.Error(err)
	}
	data, err := fs.ReadFile(tree, "lessons/a/scene.svg")
	if err != nil || string(data) != "<svg/>" {
		t.Errorf("ReadFile = %q, %v", data, err)
	}
	entries, err := fs.ReadDir(tree, "lessons")
	if err != nil || len(entries) != 3 || entries[0].Name() != "a" || entries[1].Name() != "b" || entries[2].Name() != "empty" {
		t.Errorf("ReadDir(lessons) = %v, %v", entries, err)
	}
}

func TestTreeFSErrors(t *testing.T) {
	tree := treeOf(map[string]string{"cassette.json": "{}"})
	tests := []struct {
		name string
		want error
	}{
		{"missing.json", fs.ErrNotExist},
		{"lessons", fs.ErrNotExist},
		{"../cassette.json", fs.ErrInvalid},
		{"/cassette.json", fs.ErrInvalid},
		{"cassette.json/", fs.ErrInvalid},
	}
	for _, tt := range tests {
		if _, err := tree.Open(tt.name); !errors.Is(err, tt.want) {
			t.Errorf("Open(%q) error = %v, want %v", tt.name, err, tt.want)
		}
	}

	f, err := tree.Open(".")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Read(make([]byte, 1)); err == nil {
		t.Error("reading a folder succeeded")
	} else {
		hasRoute(t, err, "treeDir.Read.ArchiveReadDirectory")
	}
}

func TestTreeDirReadDirPaging(t *testing.T) {
	tree := treeOf(map[string]string{"a.json": "", "b.json": "", "c.json": ""})
	f, err := tree.Open(".")
	if err != nil {
		t.Fatal(err)
	}
	dir := f.(fs.ReadDirFile)
	first, err := dir.ReadDir(2)
	if err != nil || len(first) != 2 {
		t.Fatalf("first = %v, %v", first, err)
	}
	second, err := dir.ReadDir(5)
	if err != nil || len(second) != 1 || second[0].Name() != "c.json" {
		t.Fatalf("second = %v, %v", second, err)
	}
	if _, err := dir.ReadDir(1); err != io.EOF {
		t.Errorf("after the end: %v", err)
	}
	rest, err := dir.ReadDir(-1)
	if err != nil || len(rest) != 0 {
		t.Errorf("ReadDir(-1) at the end = %v, %v", rest, err)
	}
}

func TestTreeOpenFailure(t *testing.T) {
	tree := newTree()
	tree.addFile("cassette.json", &treeEntry{open: func() (io.ReadCloser, error) { return nil, io.ErrClosedPipe }})
	_, err := tree.Open("cassette.json")
	hasRoute(t, err, "treeFS.Open.ArchiveEntryOpenFailed")
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("cause lost: %v", err)
	}
}

func TestCapReader(t *testing.T) {
	tests := []struct {
		name  string
		data  string
		want  int64
		route string // "" for success
	}{
		{"exact", "hello", 5, ""},
		{"empty", "", 0, ""},
		{"too many bytes", "hello world", 5, "capReader.Read.ArchiveEntryTooLarge"},
		{"one byte too many", "hello!", 5, "capReader.Read.ArchiveEntryTooLarge"},
		{"too few bytes", "hel", 5, "capReader.Read.ArchiveEntryTruncated"},
		{"nothing for a size", "", 3, "capReader.Read.ArchiveEntryTruncated"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// OneByteReader and a plain reader both must be caught.
			for _, r := range []io.Reader{strings.NewReader(tt.data), iotest.OneByteReader(strings.NewReader(tt.data))} {
				cr := &capReader{r: r, name: "x", want: tt.want}
				got, err := io.ReadAll(cr)
				if tt.route == "" {
					if err != nil || string(got) != tt.data {
						t.Errorf("got %q, %v", got, err)
					}
					continue
				}
				hasRoute(t, err, tt.route)
				if int64(len(got)) > tt.want {
					t.Errorf("returned %d bytes, more than the %d checked", len(got), tt.want)
				}
				// The error is sticky for too many bytes.
				if tt.route == "capReader.Read.ArchiveEntryTooLarge" {
					if _, err := cr.Read(make([]byte, 4)); err == nil {
						t.Error("second read succeeded")
					}
				}
			}
		})
	}
}

func TestCtxReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := ctxReader{ctx: ctx, r: bytes.NewReader([]byte("abc"))}
	buf := make([]byte, 2)
	if n, err := r.Read(buf); n != 2 || err != nil {
		t.Fatalf("first read = %d, %v", n, err)
	}
	cancel()
	_, err := r.Read(buf)
	hasRoute(t, err, "ctxReader.Read.ArchiveCanceled")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error does not wrap context.Canceled: %v", err)
	}
}

func TestCancelCheck(t *testing.T) {
	if err := cancelCheck(context.Background(), "x"); err != nil {
		t.Errorf("live context: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := cancelCheck(ctx, "x")
	hasRoute(t, err, "cancelCheck.ArchiveCanceled")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("not canceled: %v", err)
	}
}

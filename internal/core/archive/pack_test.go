package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rm4n0s/errors"
)

func TestPackDeterministic(t *testing.T) {
	dir := writeTree(t, validFS(t))
	out1 := filepath.Join(t.TempDir(), "a.cassette")
	out2 := filepath.Join(t.TempDir(), "b.cassette")
	for _, out := range []string{out1, out2} {
		ps, err := Pack(context.Background(), dir, out, false)
		if err != nil || len(ps) != 0 {
			t.Fatalf("%v %v", err, ps)
		}
		// Change the times of the files between the two packs.
		later := time.Now().Add(48 * time.Hour)
		if err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			return os.Chtimes(p, later, later)
		}); err != nil {
			t.Fatal(err)
		}
	}
	a, err := os.ReadFile(out1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Error("packing the same folder twice gave different bytes")
	}
}

func TestPackFormat(t *testing.T) {
	out := filepath.Join(t.TempDir(), "a.cassette")
	if ps, err := Pack(context.Background(), writeTree(t, validFS(t)), out, false); err != nil || len(ps) != 0 {
		t.Fatalf("%v %v", err, ps)
	}
	info, err := os.Stat(out)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, %v", info.Mode(), err)
	}
	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
		if f.Method != zip.Deflate {
			t.Errorf("%s: method %d", f.Name, f.Method)
		}
		if len(f.Extra) != 0 {
			t.Errorf("%s: extra fields %x", f.Name, f.Extra)
		}
		if want := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC); !f.Modified.Equal(want) {
			t.Errorf("%s: modified %v", f.Name, f.Modified)
		}
		if f.Mode() != 0o644 {
			t.Errorf("%s: mode %v", f.Name, f.Mode())
		}
		if strings.HasSuffix(f.Name, "/") {
			t.Errorf("folder entry %s", f.Name)
		}
	}
	if !slices.IsSorted(names) || len(names) != len(validFS(t)) {
		t.Errorf("names = %v", names)
	}
	if names[0] != "cassette.json" {
		t.Errorf("first entry = %s", names[0])
	}
}

func TestPackRoundTrip(t *testing.T) {
	original := validFS(t)
	out := packFS(t, original)
	view, closer, ps, err := OpenZip(context.Background(), out)
	if err != nil || len(ps) != 0 {
		t.Fatalf("%v %v", err, ps)
	}
	defer func() { _ = closer.Close() }()
	for name, f := range original {
		got, err := fs.ReadFile(view, name)
		if err != nil || !bytes.Equal(got, f.Data) {
			t.Errorf("%s differs: %v", name, err)
		}
	}
	rep, err := ValidatePath(context.Background(), out, &fakeRender{})
	if err != nil || len(rep.Problems) != 0 {
		t.Errorf("%v\n%s", err, listProblems(rep.Problems))
	}
}

func TestPackOverwrite(t *testing.T) {
	dir := writeTree(t, validFS(t))
	out := filepath.Join(t.TempDir(), "a.cassette")
	if err := os.WriteFile(out, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Pack(context.Background(), dir, out, false)
	hasRoute(t, err, "Pack.ArchiveOutputExists")
	if data, _ := os.ReadFile(out); string(data) != "old" {
		t.Error("the existing file was changed")
	}
	assertNoTemp(t, filepath.Dir(out), "a.cassette")

	if ps, err := Pack(context.Background(), dir, out, true); err != nil || len(ps) != 0 {
		t.Fatalf("%v %v", err, ps)
	}
	if data, _ := os.ReadFile(out); !bytes.HasPrefix(data, []byte("PK")) {
		t.Error("the file was not replaced")
	}
	assertNoTemp(t, filepath.Dir(out), "a.cassette")
}

// assertNoTemp checks that the folder holds nothing but the named files.
func assertNoTemp(t *testing.T, dir string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	slices.Sort(got)
	slices.Sort(names)
	if !slices.Equal(got, names) {
		t.Errorf("files in %s = %v, want %v", dir, got, names)
	}
}

func TestPackRefusals(t *testing.T) {
	t.Run("a folder with files that are not part of a cassette", func(t *testing.T) {
		fsys := validFS(t)
		fsys["notes.txt"] = &fstest.MapFile{Data: []byte("x")}
		fsys["lessons/what-is-a-fraction/scene.svg.bak"] = &fstest.MapFile{Data: []byte("x")}
		out := filepath.Join(t.TempDir(), "a.cassette")
		ps, err := Pack(context.Background(), writeTree(t, fsys), out, false)
		if err != nil {
			t.Fatal(err)
		}
		checkProblems(t, ps,
			want{"notes.txt", "not part of a cassette", false},
			want{"lessons/what-is-a-fraction/scene.svg.bak", "not part of a cassette", false})
		assertNoTemp(t, filepath.Dir(out))
	})
	t.Run("a symbolic link", func(t *testing.T) {
		dir := writeTree(t, validFS(t))
		if err := os.Symlink("/etc/passwd", filepath.Join(dir, "cover.svg.link")); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(t.TempDir(), "a.cassette")
		ps, err := Pack(context.Background(), dir, out, false)
		if err != nil {
			t.Fatal(err)
		}
		checkProblems(t, ps, want{"cover.svg.link", "symbolic link", false})
		assertNoTemp(t, filepath.Dir(out))
	})
	t.Run("a file that compresses more than 100 times", func(t *testing.T) {
		fsys := validFS(t)
		fsys["cover.svg"].Data = append([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 300 400">`), append(bytes.Repeat([]byte(" "), 4<<20), []byte("</svg>")...)...)
		out := filepath.Join(t.TempDir(), "a.cassette")
		ps, err := Pack(context.Background(), writeTree(t, fsys), out, false)
		if err != nil {
			t.Fatal(err)
		}
		checkProblems(t, ps, want{"cover.svg", "zip bomb", false})
		assertNoTemp(t, filepath.Dir(out))
	})
	t.Run("not a folder", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := Pack(context.Background(), file, filepath.Join(t.TempDir(), "a.cassette"), false)
		hasRoute(t, err, "Pack.ArchivePackNotFolder")
	})
	t.Run("a folder that does not exist", func(t *testing.T) {
		_, err := Pack(context.Background(), filepath.Join(t.TempDir(), "nope"), filepath.Join(t.TempDir(), "a.cassette"), false)
		hasRoute(t, err, "Pack.ArchivePackFolderStatFailed")
	})
	t.Run("an output that cannot be looked at", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := Pack(context.Background(), "testdata/valid", filepath.Join(file, "a.cassette"), false)
		hasRoute(t, err, "Pack.ArchiveOutputStatFailed")
	})
	t.Run("an output in a folder that does not exist", func(t *testing.T) {
		_, err := Pack(context.Background(), "testdata/valid", filepath.Join(t.TempDir(), "no", "a.cassette"), false)
		hasRoute(t, err, "Pack.ArchivePackCreateFailed")
	})
	t.Run("an output that cannot be replaced", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "a.cassette")
		if err := os.MkdirAll(filepath.Join(out, "inside"), 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := Pack(context.Background(), "testdata/valid", out, true)
		hasRoute(t, err, "Pack.ArchivePackRenameFailed")
		assertNoTemp(t, filepath.Dir(out), "a.cassette")
	})
	t.Run("cancelled at every step", func(t *testing.T) {
		dir := writeTree(t, validFS(t))
		out := filepath.Join(t.TempDir(), "a.cassette")
		steps := cancelAtEveryStep(t, func(ctx context.Context) error {
			_, err := Pack(ctx, dir, out, true)
			if err != nil {
				// A cancelled pack leaves no output and no temporary file.
				assertNoTemp(t, filepath.Dir(out))
			}
			return err
		})
		if steps < 5 {
			t.Errorf("only %d cancellation points", steps)
		}
	})
}

// failWriter fails every write after the first n bytes.
type failWriter struct{ n int }

func (w *failWriter) Write(p []byte) (int, error) {
	if len(p) > w.n {
		n := w.n
		w.n = 0
		return n, os.ErrClosed
	}
	w.n -= len(p)
	return len(p), nil
}

func TestWriteEntriesErrors(t *testing.T) {
	tree, _, _, err := scanFS(context.Background(), validFS(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Run("the writer fails", func(t *testing.T) {
		err := writeEntries(context.Background(), zip.NewWriter(&failWriter{n: 100}), tree)
		hasRoute(t, err, "writeEntries.ArchivePackEntryFailed")
		if !errors.Is(err, os.ErrClosed) {
			t.Errorf("cause lost: %v", err)
		}
	})
	t.Run("a file of the folder cannot be opened", func(t *testing.T) {
		broken := newTree()
		broken.addFile("cassette.json", &treeEntry{open: func() (io.ReadCloser, error) { return nil, os.ErrPermission }})
		var buf bytes.Buffer
		err := writeEntries(context.Background(), zip.NewWriter(&buf), broken)
		hasRoute(t, err, "ArchiveEntryOpenFailed")
	})
	t.Run("a file changes while it is packed", func(t *testing.T) {
		// The tree says 3 bytes, the file has 10.
		changed := newTree()
		changed.addFile("cassette.json", &treeEntry{size: 3, open: func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("0123456789")), nil
		}})
		var buf bytes.Buffer
		err := writeEntries(context.Background(), zip.NewWriter(&buf), changed)
		hasRoute(t, err, "ArchiveEntryTooLarge")
	})
}

func TestPackFinishFailure(t *testing.T) {
	// A file that is already closed cannot be written: the end of the zip fails.
	f, err := os.CreateTemp(t.TempDir(), "x")
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	if _, err := zw.Create("cassette.json"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	err = finishZip(zw, f)
	hasRoute(t, err, "finishZip.ArchivePackFinishFailed")
}

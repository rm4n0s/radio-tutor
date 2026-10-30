package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
)

// openFiles opens the zip at path and returns the files of the view, the
// problems and the error.
func openFiles(t *testing.T, path string) (map[string][]byte, []want, error) {
	t.Helper()
	view, closer, ps, err := OpenZip(context.Background(), path)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = closer.Close() }()
	files := map[string][]byte{}
	werr := fs.WalkDir(view, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(view, name)
		files[name] = data
		return err
	})
	if werr != nil {
		t.Fatal(werr)
	}
	var got []want
	for _, p := range ps {
		got = append(got, want{p.Path, p.Message, p.Warning})
	}
	return files, got, nil
}

func TestOpenZipValid(t *testing.T) {
	fsys := validFS(t)
	path := buildZip(t, zipOf(t, fsys)...)
	view, closer, ps, err := OpenZip(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closer.Close() }()
	if len(ps) != 0 {
		t.Fatalf("problems:\n%s", listProblems(ps))
	}
	names := make([]string, 0, len(fsys))
	for name, f := range fsys {
		names = append(names, name)
		got, err := fs.ReadFile(view, name)
		if err != nil || !bytes.Equal(got, f.Data) {
			t.Errorf("%s: %d bytes, %v", name, len(got), err)
		}
	}
	if err := fstest.TestFS(view, names...); err != nil {
		t.Error(err)
	}
}

func TestOpenZipWithFolderEntries(t *testing.T) {
	path := buildZip(t,
		zent{name: "lessons/"}, zent{name: "lessons/l1/"}, zent{name: "exercises/"},
		zent{name: "cassette.json", data: []byte("{}")},
		zent{name: "lessons/l1/lesson.json", data: []byte("{}")},
	)
	files, ps, err := openFiles(t, path)
	if err != nil || len(ps) != 0 || len(files) != 2 {
		t.Fatalf("files = %d, problems = %v, err = %v", len(files), ps, err)
	}
}

// bigZeros is a file of zeros, which deflates to almost nothing.
func bigZeros(n int) []byte { return make([]byte, n) }

func TestOpenZipHostile(t *testing.T) {
	good := []byte(`{"format": 1}`)
	garbage := noise(3000) // does not compress, so the ratio check does not fire
	cdata, ccrc := deflated(t, garbage)

	manyEntries := []zent{}
	for i := 0; i <= MaxEntries; i++ {
		manyEntries = append(manyEntries, zent{name: fmt.Sprintf("lessons/l1/f%d", i)})
	}
	sixHuge := []zent{}
	for i := 0; i < 6; i++ {
		sixHuge = append(sixHuge, zent{name: fmt.Sprintf("lessons/l%d/narration.ogg", i), raw: true, method: zip.Store, uncompressed: 190 << 20, compressed: 190 << 20})
	}

	tests := []struct {
		name    string
		entries []zent
		want    []want
		files   int // files left in the view
	}{
		{"zip-slip", []zent{{name: "../evil.json", data: good}, {name: "cassette.json", data: good}},
			[]want{{"../evil.json", ".. part", false}}, 1},
		{"zip-slip deep", []zent{{name: "lessons/../../evil", data: good}},
			[]want{{"lessons/../../evil", ".. part", false}}, 0},
		{"absolute name", []zent{{name: "/etc/passwd", data: good}},
			[]want{{"/etc/passwd", "relative", false}}, 0},
		{"backslash", []zent{{name: "lessons\\l1\\lesson.json", data: good}},
			[]want{{`"lessons\\l1\\lesson.json"...`, "backslash", false}}, 0},
		{"empty part", []zent{{name: "lessons//l1/lesson.json", data: good}},
			[]want{{"lessons//l1/lesson.json", "empty part", false}}, 0},
		{"dot part", []zent{{name: "./cassette.json", data: good}},
			[]want{{"./cassette.json", ". part", false}}, 0},
		{"uppercase and spaces", []zent{{name: "Cassette.JSON", data: good}, {name: "my file.svg", data: good}},
			[]want{{`"Cassette.JSON"...`, "characters", false}, {`"my file.svg"...`, "characters", false}}, 0},
		{"non-ascii and control characters", []zent{{name: "caf\u00e9.json", data: good}, {name: "a\x01b", data: good}},
			[]want{{`"caf\u00e9.json"...`, "characters", false}, {`"a\x01b"...`, "characters", false}}, 0},
		{"extra files", []zent{
			{name: ".DS_Store", data: good}, {name: "__MACOSX/._cassette.json", data: good},
			{name: "Thumbs.db", data: good}, {name: "readme.md", data: good}, {name: "lessons/l1/notes.txt", data: good},
			{name: "other/", data: nil}, {name: "cassette.json", data: good},
		}, []want{
			{`".DS_Store"...`, "characters", false}, {`"__MACOSX/._cassette.json"...`, "characters", false},
			{`"Thumbs.db"...`, "characters", false},
			{"readme.md", "not part of a cassette", false}, {"lessons/l1/notes.txt", "not part of a cassette", false},
			{"other", "not part of a cassette", false},
		}, 1},
		{"lowercase macos junk", []zent{{name: "__macosx/", data: nil}, {name: ".ds_store", data: good}},
			[]want{{"__macosx", "not part of a cassette", false}, {".ds_store", "not part of a cassette", false}}, 0},
		{"duplicate names", []zent{{name: "cassette.json", data: good}, {name: "cassette.json", data: []byte("{}")}},
			[]want{{"cassette.json", "listed twice", false}}, 1},
		{"folder listed twice", []zent{{name: "lessons/"}, {name: "lessons/"}},
			[]want{{"lessons", "listed twice", false}}, 0},
		{"symlink", []zent{{name: "cassette.json", data: []byte("/etc/passwd"), mode: fs.ModeSymlink | 0o777}},
			[]want{{"cassette.json", "symbolic link", false}}, 0},
		{"device", []zent{{name: "cover.svg", mode: fs.ModeDevice | 0o666}, {name: "lessons/l1/scene.svg", mode: fs.ModeNamedPipe | 0o666}, {name: "lessons/l1/lesson.json", mode: fs.ModeSocket | 0o666}},
			[]want{{"cover.svg", "special file", false}, {"lessons/l1/scene.svg", "special file", false}, {"lessons/l1/lesson.json", "special file", false}}, 0},
		{"setuid", []zent{{name: "cover.svg", data: good, mode: fs.ModeSetuid | 0o755}},
			[]want{{"cover.svg", "special file", false}}, 0},
		{"file marked as folder", []zent{{name: "cover.svg", data: good, mode: fs.ModeDir | 0o755}},
			[]want{{"cover.svg", "marked as a folder", false}}, 0},
		{"folder with data", []zent{{name: "lessons/", raw: true, method: zip.Store, uncompressed: 3, crc: 0, mode: fs.ModeDir | 0o755}},
			[]want{{"lessons", "folder with data", false}}, 0},
		{"bomb ratio", []zent{{name: "lessons/l1/scene.svg", data: bigZeros(20 << 20)}, {name: "cassette.json", data: good}},
			[]want{{"lessons/l1/scene.svg", "zip bomb", false}}, 1},
		{"bomb with no compressed bytes", []zent{{name: "cover.svg", raw: true, method: zip.Store, uncompressed: 10, compressed: 0}},
			[]want{{"cover.svg", "zip bomb", false}}, 0},
		{"entry too large", []zent{{name: "lessons/l1/narration.ogg", raw: true, method: zip.Store, uncompressed: 201 << 20, compressed: 201 << 20}},
			[]want{{"lessons/l1/narration.ogg", "limit for one file", false}}, 0},
		{"too many bytes", sixHuge,
			[]want{{"", "1 GiB", false}}, 0},
		{"too many entries", manyEntries,
			[]want{{"", "has 2001 entries", false}}, 0},
		{"unsupported compression", []zent{{name: "cover.svg", raw: true, method: 12, data: []byte("x"), uncompressed: 1}},
			[]want{{"cover.svg", "compression method 12", false}}, 0},
		{"encrypted", []zent{{name: "cover.svg", raw: true, method: zip.Store, flags: 1, data: []byte("x"), uncompressed: 1}},
			[]want{{"cover.svg", "encrypted", false}}, 0},
		{"header says smaller than the data", []zent{{name: "cover.svg", raw: true, method: zip.Deflate, data: cdata, uncompressed: 100, crc: ccrc}},
			[]want{{"cover.svg", "more data than its header says", false}}, 0},
		{"header says larger than the data", []zent{{name: "cover.svg", raw: true, method: zip.Deflate, data: cdata, uncompressed: uint64(len(garbage)) + 500, crc: ccrc}},
			[]want{{"cover.svg", "less data than its header says", false}}, 0},
		{"wrong checksum", []zent{{name: "cover.svg", raw: true, method: zip.Deflate, data: cdata, uncompressed: uint64(len(garbage)), crc: ccrc + 1}},
			[]want{{"cover.svg", "damaged", false}}, 0},
		{"damaged deflate data", []zent{{name: "cover.svg", raw: true, method: zip.Deflate, data: bytes.Repeat([]byte{0xff}, 20), uncompressed: 50}},
			[]want{{"cover.svg", "damaged", false}}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := buildZip(t, tt.entries...)
			files, got, err := openFiles(t, path)
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != tt.files {
				t.Errorf("view has %d files, want %d", len(files), tt.files)
			}
			var ps = make([]string, 0)
			_ = ps
			// Compare as sets with checkProblems on a Problem list.
			list := make([]want, len(got))
			copy(list, got)
			asProblems := toProblems(list)
			checkProblems(t, asProblems, tt.want...)
		})
	}
}

func TestOpenZipHostileEntriesStayOutOfTheView(t *testing.T) {
	good := []byte(`{"format": 1}`)
	path := buildZip(t,
		zent{name: "../evil", data: good},
		zent{name: "cassette.json", data: good},
		zent{name: "cover.svg", data: good, mode: fs.ModeSymlink | 0o777},
	)
	view, closer, ps, err := OpenZip(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closer.Close() }()
	if len(ps) != 2 {
		t.Errorf("problems = %v", ps)
	}
	for _, name := range []string{"cover.svg", "evil", "../evil"} {
		if _, err := fs.Stat(view, name); err == nil {
			t.Errorf("%s is in the view", name)
		}
	}
	if _, err := fs.Stat(view, "cassette.json"); err != nil {
		t.Errorf("cassette.json is not: %v", err)
	}
}

func TestOpenZipFileLevel(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	big := filepath.Join(dir, "big.cassette")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxZipBytes + 1); err != nil { // a sparse file
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	// 100 entries whose names are 60000 bytes long: a central directory of 6 MB.
	var longNames []zent
	for i := 0; i < 100; i++ {
		longNames = append(longNames, zent{name: fmt.Sprintf("%05d", i) + strings.Repeat("a", 60000)})
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{"not a zip", write("text.cassette", []byte("this is not a zip file at all, it is text")), "is not a zip file"},
		{"empty file", write("empty.cassette", nil), "is not a zip file"},
		{"truncated zip", write("cut.cassette", zipBytes(t, zent{name: "cassette.json", data: []byte("{}")})[:30]), "is not a zip file"},
		{"folder", dir, "is a folder"},
		{"too large", big, "limit is 536870912 bytes"},
		{"central directory too large", write("dir.cassette", zipBytes(t, longNames...)), "list of entries"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view, closer, ps, err := OpenZip(context.Background(), tt.path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = closer.Close() }()
			if len(ps) != 1 || ps[0].Path != "" || !strings.Contains(ps[0].Message, tt.want) || ps[0].Warning {
				t.Fatalf("problems = %v, want one containing %q", ps, tt.want)
			}
			entries, err := fs.ReadDir(view, ".")
			if err != nil || len(entries) != 0 {
				t.Errorf("view is not empty: %v, %v", entries, err)
			}
		})
	}
}

func TestOpenZipErrors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, _, _, err := OpenZip(context.Background(), filepath.Join(t.TempDir(), "nope.cassette"))
		hasRoute(t, err, "OpenZip->openZip.ArchiveStatFailed")
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("cause lost: %v", err)
		}
	})
	t.Run("a file that cannot be opened", func(t *testing.T) {
		storagetest.SkipIfModesDoNotBlock(t)
		path := buildZip(t, zent{name: "cassette.json", data: []byte("{}")})
		if err := os.Chmod(path, 0); err != nil {
			t.Fatal(err)
		}
		_, _, _, err := OpenZip(context.Background(), path)
		hasRoute(t, err, "OpenZip->openZip.ArchiveOpenFailed")
		if !errors.Is(err, fs.ErrPermission) {
			t.Errorf("cause lost: %v", err)
		}
	})
	t.Run("a damaged entry is reported, a cancelled one is an error", func(t *testing.T) {
		data := zipBytes(t, zent{name: "cover.svg", data: noise(5000)})
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		msg, err := verifyEntry(ctx, zr.File[0])
		hasRoute(t, err, "ctxReader.Read.ArchiveCanceled")
		if msg != "" {
			t.Errorf("msg = %q", msg)
		}
	})
	t.Run("cancelled before opening", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, _, _, err := OpenZip(ctx, buildZip(t, zent{name: "cassette.json", data: []byte("{}")}))
		hasRoute(t, err, "OpenZip->openZip->cancelCheck.ArchiveCanceled")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("not canceled: %v", err)
		}
	})
	t.Run("cancelled at every step", func(t *testing.T) {
		path := buildZip(t, zipOf(t, validFS(t))...)
		steps := cancelAtEveryStep(t, func(ctx context.Context) error {
			_, closer, _, err := OpenZip(ctx, path)
			if err == nil {
				return closer.Close()
			}
			return err
		})
		if steps < 4 {
			t.Errorf("only %d cancellation points", steps)
		}
	})
}

func TestDirectoryBytes(t *testing.T) {
	t.Run("plain zip", func(t *testing.T) {
		data := zipBytes(t, zent{name: "cassette.json", data: []byte("{}")}, zent{name: "cover.svg", data: []byte("<svg/>")})
		n, ok := directoryBytes(bytes.NewReader(data), int64(len(data)))
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		// Each record is 46 bytes plus the name, plus the extra field.
		want := 0
		for _, f := range zr.File {
			want += 46 + len(f.Name) + len(f.Extra) + len(f.Comment)
		}
		if !ok || n != int64(want) {
			t.Errorf("directoryBytes = %d, %v; want %d", n, ok, want)
		}
	})
	t.Run("zip64", func(t *testing.T) {
		// More than 65535 entries make the zip package write the zip64 records.
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		want := 0
		for i := 0; i < 70000; i++ {
			name := fmt.Sprintf("f%d", i)
			if _, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store}); err != nil {
				t.Fatal(err)
			}
			want += 46 + len(name)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		n, ok := directoryBytes(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if !ok || n != int64(want) {
			t.Errorf("directoryBytes = %d, %v; want %d", n, ok, want)
		}
		path := filepath.Join(t.TempDir(), "many.cassette")
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		_, closer, ps, err := OpenZip(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = closer.Close() }()
		if len(ps) != 1 || !strings.Contains(ps[0].Message, "70000 entries") {
			t.Errorf("problems = %v", ps)
		}
	})
	t.Run("not found", func(t *testing.T) {
		for name, data := range map[string][]byte{
			"text":  []byte(strings.Repeat("hello ", 20)),
			"short": []byte("PK"),
			"empty": nil,
		} {
			if n, ok := directoryBytes(bytes.NewReader(data), int64(len(data))); ok {
				t.Errorf("%s: ok with %d", name, n)
			}
		}
	})
	t.Run("offset after the end", func(t *testing.T) {
		data := zipBytes(t, zent{name: "cassette.json", data: []byte("{}")})
		pos := bytes.LastIndex(data, []byte("PK\x05\x06"))
		bad := bytes.Clone(data)
		copy(bad[pos+16:], []byte{0xf0, 0xff, 0x00, 0x00})
		if n, ok := directoryBytes(bytes.NewReader(bad), int64(len(bad))); ok {
			t.Errorf("ok with %d", n)
		}
	})
	t.Run("zip64 locator without a record", func(t *testing.T) {
		data := zipBytes(t, zent{name: "cassette.json", data: []byte("{}")})
		pos := bytes.LastIndex(data, []byte("PK\x05\x06"))
		bad := bytes.Clone(data)
		copy(bad[pos+16:], []byte{0xff, 0xff, 0xff, 0xff})
		if n, ok := directoryBytes(bytes.NewReader(bad), int64(len(bad))); ok {
			t.Errorf("ok with %d", n)
		}
		// With a locator that points outside the file.
		loc := append([]byte("PK\x06\x07"), make([]byte, 16)...)
		loc[8], loc[9] = 0xff, 0x7f
		withLoc := append(bytes.Clone(data[:pos]), append(loc, data[pos:]...)...)
		copy(withLoc[pos+len(loc)+16:], []byte{0xff, 0xff, 0xff, 0xff})
		if n, ok := directoryBytes(bytes.NewReader(withLoc), int64(len(withLoc))); ok {
			t.Errorf("locator outside the file: ok with %d", n)
		}
		// With a locator that points at something that is not a zip64 record.
		loc[8] = 0
		loc[9] = 0
		withLoc2 := append(bytes.Clone(data[:pos]), append(loc, data[pos:]...)...)
		copy(withLoc2[pos+len(loc)+16:], []byte{0xff, 0xff, 0xff, 0xff})
		if n, ok := directoryBytes(bytes.NewReader(withLoc2), int64(len(withLoc2))); ok {
			t.Errorf("locator to junk: ok with %d", n)
		}
	})
	t.Run("read error", func(t *testing.T) {
		if n, ok := directoryBytes(failingReaderAt{}, 100); ok {
			t.Errorf("ok with %d", n)
		}
	})
}

type failingReaderAt struct{}

func (failingReaderAt) ReadAt([]byte, int64) (int, error) { return 0, io.ErrClosedPipe }

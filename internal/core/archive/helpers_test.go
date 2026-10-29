package archive

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"hash/crc32"
	"io"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/problem"
)

// Paths of the files of testdata/valid used by many tests.
const (
	lesson1Dir  = "lessons/what-is-a-fraction"
	lesson1JSON = lesson1Dir + "/lesson.json"
	lesson1SVG  = lesson1Dir + "/scene.svg"
	lesson1OGG  = lesson1Dir + "/narration.ogg"
	lesson2Dir  = "lessons/adding-fractions"
	promptSVG   = "exercises/shade-the-half/prompt.svg"
	promptOGG   = "exercises/shade-the-half/prompt.ogg"
	validID     = "org.example.archive-1"
)

// validFS returns a fresh in-memory copy of testdata/valid.
func validFS(t testing.TB) fstest.MapFS {
	t.Helper()
	out := fstest.MapFS{}
	root := os.DirFS("testdata/valid")
	err := fs.WalkDir(root, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(root, name)
		if err != nil {
			return err
		}
		out[name] = &fstest.MapFile{Data: data}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// writeTree writes the files of fsys under a new temporary folder and returns
// it.
func writeTree(t testing.TB, fsys fstest.MapFS) string {
	t.Helper()
	dir := t.TempDir()
	for name, f := range fsys {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, f.Data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// packFS packs the files of fsys into a new .cassette file with Pack and
// returns its path.
func packFS(t testing.TB, fsys fstest.MapFS) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "test.cassette")
	ps, err := Pack(context.Background(), writeTree(t, fsys), out, false)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if len(ps) != 0 {
		t.Fatalf("Pack problems: %v", ps)
	}
	return out
}

// withVersion returns a copy of the valid cassette with another version.
func withVersion(t testing.TB, version string) fstest.MapFS {
	t.Helper()
	fsys := validFS(t)
	fsys["cassette.json"].Data = bytes.Replace(fsys["cassette.json"].Data, []byte(`"version": "1.0.0"`), []byte(`"version": "`+version+`"`), 1)
	if !bytes.Contains(fsys["cassette.json"].Data, []byte(version)) {
		t.Fatal("version not replaced")
	}
	return fsys
}

// zent describes an entry of a hostile zip.
type zent struct {
	name   string
	data   []byte
	method uint16      // zip.Store or zip.Deflate (default)
	mode   fs.FileMode // 0 means a plain file (or a folder if name ends in /)
	flags  uint16
	// raw entries are written as they are given: the compressed bytes in
	// data and the header values below.
	raw          bool
	uncompressed uint64
	crc          uint32
	compressed   uint64 // 0 means len(data)
}

// buildZip writes a zip with the entries to a new file and returns its path.
func buildZip(t testing.TB, entries ...zent) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hostile.cassette")
	if err := os.WriteFile(path, zipBytes(t, entries...), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// zipBytes builds the bytes of a zip.
func zipBytes(t testing.TB, entries ...zent) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: e.method, Flags: e.flags}
		if !e.raw && e.method == 0 && !strings.HasSuffix(e.name, "/") {
			h.Method = zip.Deflate
		}
		if e.mode != 0 {
			h.SetMode(e.mode)
		}
		if e.raw {
			h.Method = e.method
			h.UncompressedSize64 = e.uncompressed
			h.CompressedSize64 = e.compressed
			if h.CompressedSize64 == 0 {
				h.CompressedSize64 = uint64(len(e.data))
			}
			h.CRC32 = e.crc
			w, err := zw.CreateRaw(h)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write(e.data); err != nil {
				t.Fatal(err)
			}
			continue
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// deflated returns data compressed with deflate and its CRC.
func deflated(t testing.TB, data []byte) ([]byte, uint32) {
	t.Helper()
	var buf bytes.Buffer
	fw, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := fw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), crc32.ChecksumIEEE(data)
}

// zipOf builds the zip of the files of fsys, one entry each.
func zipOf(t testing.TB, fsys fstest.MapFS, extra ...zent) []zent {
	t.Helper()
	var out []zent
	for name, f := range fsys {
		out = append(out, zent{name: name, data: f.Data})
	}
	return append(out, extra...)
}

// want is one expected problem: the path, a part of the message, whether it is
// a warning.
type want struct {
	path    string
	msg     string
	warning bool
}

// checkProblems asserts that ps has exactly the wanted problems, in any order.
func checkProblems(t testing.TB, ps []problem.Problem, wants ...want) {
	t.Helper()
	used := make([]bool, len(ps))
	for _, w := range wants {
		found := false
		for i, p := range ps {
			if !used[i] && p.Path == w.path && strings.Contains(p.Message, w.msg) && p.Warning == w.warning {
				used[i], found = true, true
				break
			}
		}
		if !found {
			t.Errorf("missing problem %+v in:\n%s", w, listProblems(ps))
		}
	}
	for i, p := range ps {
		if !used[i] {
			t.Errorf("unexpected problem %q in:\n%s", p.String(), listProblems(ps))
		}
	}
}

func listProblems(ps []problem.Problem) string {
	var b strings.Builder
	for _, p := range ps {
		b.WriteString("  " + p.String() + "\n")
	}
	return b.String()
}

// hasRoute asserts that err is an *errors.Error with the route.
func hasRoute(t testing.TB, err error, route string) {
	t.Helper()
	if err == nil {
		t.Fatalf("no error, want route %s", route)
	}
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("error is %T, not *errors.Error: %v", err, err)
	}
	if !appErr.HasRoute(route) {
		t.Errorf("route = %s, want it to contain %s", appErr.Route(), route)
	}
}

// fakeRender is a RenderChecker that records its calls and fails for names that
// contain failOn.
type fakeRender struct {
	mu     sync.Mutex
	failOn string
	cancel context.CancelFunc // called in CheckSVG when set
	names  []string
}

func (f *fakeRender) CheckSVG(ctx context.Context, name string, data []byte) error {
	f.mu.Lock()
	f.names = append(f.names, name)
	f.mu.Unlock()
	if f.cancel != nil {
		f.cancel()
		return ctx.Err()
	}
	if f.failOn != "" && strings.Contains(name, f.failOn) {
		return errors.New("FakeRenderFailed", "the fake renderer refuses this picture")
	}
	return nil
}

// toProblems turns wants into problems, to compare lists with checkProblems.
func toProblems(ws []want) []problem.Problem {
	out := make([]problem.Problem, len(ws))
	for i, w := range ws {
		out[i] = problem.Problem{Path: w.path, Message: w.msg, Warning: w.warning}
	}
	return out
}

// noise returns n bytes that do not compress (a fixed pseudo-random sequence).
func noise(n int) []byte {
	r := rand.New(rand.NewPCG(1, 2))
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(r.Uint32())
	}
	return out
}

// stepCtx is a context that reports context.Canceled from its n+1-th call to
// Err on, so a test can cancel an operation at every point where it looks.
type stepCtx struct {
	context.Context
	left atomic.Int64
}

func newStepCtx(n int64) *stepCtx {
	c := &stepCtx{Context: context.Background()}
	c.left.Store(n)
	return c
}

func (c *stepCtx) Err() error {
	if c.left.Add(-1) < 0 {
		return context.Canceled
	}
	return nil
}

// cancelAtEveryStep runs fn with a context that is cancelled after 0, 1, 2, ...
// checks. Every run that is cancelled must return an error that wraps
// context.Canceled; the first run that is not cancelled must succeed. It
// returns the number of cancellation points.
func cancelAtEveryStep(t testing.TB, fn func(ctx context.Context) error) int {
	t.Helper()
	return cancelAtSteps(t, 1, fn)
}

// cancelAtSteps is cancelAtEveryStep trying every stride-th point only, for
// operations that are slow to repeat.
func cancelAtSteps(t testing.TB, stride int64, fn func(ctx context.Context) error) int {
	t.Helper()
	for n := int64(0); n < 1000; n += stride {
		err := fn(newStepCtx(n))
		if err == nil {
			return int(n)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled after %d checks: error does not wrap context.Canceled: %v", n, err)
		}
	}
	t.Fatal("never finished")
	return 0
}

// repeatFS is a file system with a single folder entry that lists "cover.svg"
// twice, to make a copy fail on the second.
type repeatFS struct{}

func (repeatFS) Open(name string) (fs.File, error) {
	switch name {
	case ".":
		return &repeatDir{}, nil
	case "cover.svg":
		return (fstest.MapFS{"cover.svg": {Data: []byte("x")}}).Open(name)
	}
	return nil, fs.ErrNotExist
}

type repeatDir struct{ done bool }

func (*repeatDir) Stat() (fs.FileInfo, error) {
	return (fstest.MapFS{"a": {Data: nil}}).Stat(".")
}
func (*repeatDir) Read([]byte) (int, error) { return 0, io.EOF }
func (*repeatDir) Close() error             { return nil }
func (d *repeatDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if d.done {
		return nil, io.EOF
	}
	d.done = true
	one := fs.FileInfoToDirEntry(entryInfo{name: "cover.svg", size: 1, mode: 0o444})
	return []fs.DirEntry{one, one}, nil
}

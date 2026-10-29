package cassette

import (
	"context"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rm4n0s/errors"
)

// setRaw replaces a file's bytes.
func setRaw(name, data string) func(t *testing.T, fsys fstest.MapFS) {
	return func(t *testing.T, fsys fstest.MapFS) {
		fsys[name] = &fstest.MapFile{Data: []byte(data)}
	}
}

func TestLoadFileLevelRules(t *testing.T) {
	const m = "cassette.json"
	const l1 = lesson1
	validManifest := func(t *testing.T) string {
		return string(minimalFS(t)[manifest].Data)
	}
	tests := []struct {
		name string
		mut  func(t *testing.T, fsys fstest.MapFS)
		want []want
		nil  bool // the cassette is nil
	}{
		{"empty file", setRaw(m, ""), []want{{m, "invalid JSON", false}}, true},
		{"only spaces", setRaw(m, "  \n "), []want{{m, "invalid JSON", false}}, true},
		{"syntax error names the line", setRaw(m, "{\n \"format\": 1,\n \"id\": }\n"), []want{{m, "invalid JSON on line 3", false}}, true},
		{"truncated", setRaw(m, `{"format": 1, "id": "org.x"`), []want{{m, "unexpected end of JSON input", false}}, true},
		{"trailing data", func(t *testing.T, fsys fstest.MapFS) {
			fsys[m].Data = append(fsys[m].Data, []byte(" {}")...)
		}, []want{{m, "unexpected data after the top-level value", false}}, true},
		{"trailing comma", setRaw(m, `{"format": 1,}`), []want{{m, "invalid JSON", false}}, true},
		{"comments are not JSON", setRaw(m, "// c\n{}"), []want{{m, "invalid JSON on line 1", false}}, true},
		{"top level is an array", setRaw(m, "[]"), []want{{m, "must be a JSON object, not an array", false}}, true},
		{"top level is a string", setRaw(m, `"x"`), []want{{m, "must be a JSON object, not a string", false}}, true},
		{"top level is null", setRaw(m, "null"), []want{{m, "must be a JSON object, not null", false}}, true},
		{"byte order mark", func(t *testing.T, fsys fstest.MapFS) {
			fsys[m].Data = append([]byte("\xef\xbb\xbf"), fsys[m].Data...)
		}, []want{{m, "byte order mark", false}}, true},
		{"invalid UTF-8", func(t *testing.T, fsys fstest.MapFS) {
			fsys[m].Data = []byte(strings.Replace(validManifest(t), "Fractions", "Fra\xffctions", 1))
		}, []want{{m, "not valid UTF-8 (first bad byte on line 5)", false}}, true},
		{"larger than 1 MiB", func(t *testing.T, fsys fstest.MapFS) {
			fsys[m].Data = []byte(strings.Replace(validManifest(t), `"Fractions, part 1"`, `"`+strings.Repeat("a", MaxJSONBytes)+`"`, 1))
		}, []want{{m, "is larger than 1 MiB", false}}, true},
		{"exactly 1 MiB is read", func(t *testing.T, fsys fstest.MapFS) {
			v := validManifest(t)
			pad := MaxJSONBytes - len(v)
			fsys[m].Data = []byte(strings.Replace(v, `"format": 1,`, `"format": 1,`+strings.Repeat(" ", pad), 1))
		}, nil, false},
		{"duplicate field", func(t *testing.T, fsys fstest.MapFS) {
			fsys[m].Data = []byte(strings.Replace(validManifest(t), `"format": 1,`, `"format": 1, "title": "again",`, 1))
		}, []want{{m + "#title", "duplicate field", false}}, true},
		{"duplicate field in a lesson key", setRaw(l1, `{"format":1,"title":"x","tracks":[{"actor":"a","keys":[{"t":0,"t":1,"x":1}]}],"captions":[{"start":0,"end":1,"text":"x"}]}`),
			[]want{{l1 + "#tracks[0].keys[0].t", "duplicate field", false}}, false},
		{"nested too deeply", setRaw(m, strings.Repeat("[", 100)+strings.Repeat("]", 100)),
			[]want{{m, "nested too deeply", false}}, true},
		{"a number too large for float64", setRaw(l1, `{"format":1,"title":"x","tracks":[],"captions":[{"start":0,"end":1e999,"text":"x"}]}`),
			[]want{{l1 + "#captions[0].end", "too large", false}}, false},
		{"a newer manifest format", setRaw(m, `{"format": 2, "something": "new"}`),
			[]want{{m + "#format", "This cassette needs a newer Radio Tutor", false}}, true},
		{"a newer lesson format shows only that", setRaw(l1, `{"format": 3, "scenes": []}`),
			[]want{{l1 + "#format", "This cassette needs a newer Radio Tutor", false}}, false},
		{"a newer exercise format", setRaw(ex1, `{"format": 2}`),
			[]want{{ex1 + "#format", "This cassette needs a newer Radio Tutor", false}}, false},
		{"escaped text is decoded", func(t *testing.T, fsys fstest.MapFS) {
			edit(t, fsys, lesson2, func(root map[string]any) { obj(root, "captions", 0)["text"] = "line one\nline \"two\" α" })
		}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := minimalFS(t)
			tt.mut(t, fsys)
			c, ps := load(t, fsys)
			checkProblems(t, ps, tt.want...)
			if (c == nil) != tt.nil {
				t.Errorf("cassette nil = %v, want %v", c == nil, tt.nil)
			}
		})
	}
}

// faultFS makes chosen operations fail.
type faultFS struct {
	fs.FS
	open, stat, read, readDir map[string]error
}

func (f faultFS) Open(name string) (fs.File, error) {
	if err := f.open[name]; err != nil {
		return nil, err
	}
	file, err := f.FS.Open(name)
	if err != nil {
		return nil, err
	}
	return faultFile{File: file, stat: f.stat[name], read: f.read[name]}, nil
}

func (f faultFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if err := f.readDir[name]; err != nil {
		return nil, err
	}
	return fs.ReadDir(f.FS, name)
}

type faultFile struct {
	fs.File
	stat, read error
}

func (f faultFile) Stat() (fs.FileInfo, error) {
	if f.stat != nil {
		return nil, f.stat
	}
	return f.File.Stat()
}

func (f faultFile) Read(p []byte) (int, error) {
	if f.read != nil {
		return 0, f.read
	}
	return f.File.Read(p)
}

func TestLoadErrors(t *testing.T) {
	boom := errors.New("Boom", "disk on fire")
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name  string
		ctx   context.Context
		fsys  func(t *testing.T) fs.FS
		route string
		path  string
	}{
		{"cancelled before the manifest", cancelled, func(t *testing.T) fs.FS { return minimalFS(t) },
			"Load->loader.document->loader.readFile.CassetteLoadCanceled", "cassette.json"},
		{"open fails", context.Background(), func(t *testing.T) fs.FS {
			return faultFS{FS: minimalFS(t), open: map[string]error{lesson2: boom}}
		}, "Load->loader.loadLesson->loader.document->loader.readFile.CassetteFileOpenFailed", lesson2},
		{"stat fails", context.Background(), func(t *testing.T) fs.FS {
			return faultFS{FS: minimalFS(t), stat: map[string]error{ex2: boom}}
		}, "Load->loader.loadExercise->loader.document->loader.readFile.CassetteFileStatFailed", ex2},
		{"read fails", context.Background(), func(t *testing.T) fs.FS {
			return faultFS{FS: minimalFS(t), read: map[string]error{manifest: boom}}
		}, "Load->loader.document->loader.readFile.CassetteFileReadFailed", manifest},
		{"listing lessons fails", context.Background(), func(t *testing.T) fs.FS {
			return faultFS{FS: minimalFS(t), readDir: map[string]error{"lessons": boom}}
		}, "Load->loader.folders.CassetteFolderReadFailed", "lessons"},
		{"listing exercises fails", context.Background(), func(t *testing.T) fs.FS {
			return faultFS{FS: minimalFS(t), readDir: map[string]error{"exercises": boom}}
		}, "Load->loader.folders.CassetteFolderReadFailed", "exercises"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, ps, err := Load(tt.ctx, tt.fsys(t))
			if c != nil || ps != nil {
				t.Errorf("cassette %v, problems %v: want none with an error", c, ps)
			}
			appErr, ok := errors.FromError(err)
			if !ok {
				t.Fatalf("error is %T: %v", err, err)
			}
			if !appErr.HasRoute(tt.route) {
				t.Errorf("route = %s, want it to contain %s", appErr.Route(), tt.route)
			}
			if appErr.Metadata["path"] != tt.path {
				t.Errorf("path = %v, want %v", appErr.Metadata["path"], tt.path)
			}
		})
	}
	// A cancelled context's error stays reachable.
	_, _, err := Load(cancelled, minimalFS(t))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false for %v", err)
	}
}

func TestLoadCancelledInTheMiddle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fsys := cancelOnOpen{FS: minimalFS(t), name: lesson2, cancel: cancel}
	_, _, err := Load(ctx, fsys)
	appErr, ok := errors.FromError(err)
	if !ok || !appErr.HasRoute("CassetteLoadCanceled") {
		t.Fatalf("err = %v", err)
	}
}

// cancelOnOpen cancels a context when a file is opened.
type cancelOnOpen struct {
	fs.FS
	name   string
	cancel context.CancelFunc
}

func (c cancelOnOpen) Open(name string) (fs.File, error) {
	f, err := c.FS.Open(name)
	if name == manifest || name == lesson1 { // the files before the one we cancel at
		return f, err
	}
	if name == c.name {
		c.cancel()
	}
	return f, err
}

package cassette

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rm4n0s/radio-tutor/internal/core/problem"
)

// minimalFS returns a fresh in-memory copy of testdata/minimal, the valid
// cassette every rule test breaks in one place.
func minimalFS(t *testing.T) fstest.MapFS {
	t.Helper()
	out := fstest.MapFS{}
	root := os.DirFS("testdata/minimal")
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

// edit decodes the JSON file name of fsys, lets fn change it and writes it
// back.
func edit(t *testing.T, fsys fstest.MapFS, name string, fn func(root map[string]any)) {
	t.Helper()
	f, ok := fsys[name]
	if !ok {
		t.Fatalf("no file %s", name)
	}
	var root map[string]any
	if err := json.Unmarshal(f.Data, &root); err != nil {
		t.Fatal(err)
	}
	fn(root)
	data, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	f.Data = data
}

// at follows a path of map keys (string) and slice indexes (int) from v.
func at(v any, path ...any) any {
	for _, p := range path {
		switch k := p.(type) {
		case string:
			v = v.(map[string]any)[k]
		case int:
			v = v.([]any)[k]
		}
	}
	return v
}

// obj is at() for a value that is an object.
func obj(v any, path ...any) map[string]any {
	return at(v, path...).(map[string]any)
}

// load runs Load and fails the test on an error.
func load(t *testing.T, fsys fs.FS) (*Cassette, []problem.Problem) {
	t.Helper()
	c, ps, err := Load(context.Background(), fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return c, ps
}

// want is an expected problem: the path, a part of the message, and whether it
// is a warning.
type want struct {
	path, msg string
	warning   bool
}

// checkProblems fails unless ps holds exactly the expected problems, in any
// order.
func checkProblems(t *testing.T, ps []problem.Problem, wants ...want) {
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
			t.Errorf("missing problem %+v", w)
		}
	}
	for i, p := range ps {
		if !used[i] {
			t.Errorf("unexpected problem: %v", p)
		}
	}
	if t.Failed() {
		t.Logf("all problems:")
		for _, p := range ps {
			t.Logf("  %v", p)
		}
	}
}

// removeFile deletes a file but keeps its folder, which an fstest.MapFS
// otherwise forgets together with its last file.
func removeFile(fsys fstest.MapFS, name string) {
	delete(fsys, name)
	fsys[path.Dir(name)+"/notes.txt"] = &fstest.MapFile{Data: []byte("x")}
}

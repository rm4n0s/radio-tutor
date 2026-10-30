package archtest

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rm4n0s/errors"
)

// moduleFS returns the module root (the folder with go.mod above this package)
// as a file system.
func moduleFS(t *testing.T) fs.FS {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return os.DirFS(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
}

// TestModuleBoundaries applies the rules of SPEC §5 to the real module. Packages
// that do not exist yet simply contribute no files.
func TestModuleBoundaries(t *testing.T) {
	files, err := Scan(moduleFS(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range Check(files) {
		t.Error(v)
	}
}

func TestCheck(t *testing.T) {
	const gui = ModulePath + "/internal/gui/app"
	tests := []struct {
		name  string
		files []File
		want  []string // "file: import"
	}{
		{name: "no files"},
		{
			name: "core imports glutter",
			files: []File{
				{Path: "internal/core/audio/a.go", Imports: []string{GlutterPrefix}},
				{Path: "internal/core/cassette/a.go", Imports: []string{GlutterPrefix + "/material"}},
				{Path: "internal/core/cassette/a_test.go", Imports: []string{GlutterPrefix + "/gluttertest"}},
			},
			want: []string{
				"internal/core/audio/a.go: " + GlutterPrefix,
				"internal/core/cassette/a.go: " + GlutterPrefix + "/material",
				"internal/core/cassette/a_test.go: " + GlutterPrefix + "/gluttertest",
			},
		},
		{
			name: "core imports gui",
			files: []File{
				{Path: "internal/core/progress/p.go", Imports: []string{gui}},
				{Path: "internal/core/progress/q.go", Imports: []string{ModulePath + "/internal/core/storage"}},
			},
			want: []string{"internal/core/progress/p.go: " + gui},
		},
		{
			name: "gui may import glutter and core",
			files: []File{
				{Path: "internal/gui/app/a.go", Imports: []string{GlutterPrefix + "/material", ModulePath + "/internal/core/storage"}},
				{Path: "cmd/radio-tutor/main.go", Imports: []string{GlutterPrefix + "/material", gui}},
			},
		},
		{
			name: "net/http in the core",
			files: []File{
				{Path: "internal/core/grading/openai/c.go", Imports: []string{"net/http"}},
				{Path: "internal/core/providers/c.go", Imports: []string{"net/http"}},
				{Path: "internal/core/phone/c.go", Imports: []string{"net/http"}},
				{Path: "internal/core/grading/g.go", Imports: []string{"net/http"}},
				{Path: "internal/core/phone/sub/c.go", Imports: []string{"net/http"}},
				{Path: "internal/core/submission/s.go", Imports: []string{"net/http"}},
				{Path: "internal/core/submission/s_test.go", Imports: []string{"net/http"}},
				{Path: "internal/gui/app/a.go", Imports: []string{"net/http"}},
			},
			want: []string{
				"internal/core/grading/g.go: net/http",
				"internal/core/phone/sub/c.go: net/http",
				"internal/core/submission/s.go: net/http",
			},
		},
		{
			name: "os/exec",
			files: []File{
				{Path: "internal/core/audio/helper_test.go", Imports: []string{"os/exec"}, BuildTags: "radiotutor_audio"},
				{Path: "internal/core/audio/other_test.go", Imports: []string{"os/exec"}},
				{Path: "internal/core/audio/plain.go", Imports: []string{"os/exec"}, BuildTags: "radiotutor_audio"},
				{Path: "internal/core/storage/s_test.go", Imports: []string{"os/exec"}, BuildTags: "radiotutor_audio"},
				{Path: "cmd/radio-tutor/main.go", Imports: []string{"os/exec"}},
				{Path: "internal/gui/app/a.go", Imports: []string{"os/exec"}},
				{Path: "scripts/samplegen/main.go", Imports: []string{"os/exec"}},
				{Path: "cmd/radio-tutor/w_test.go", Imports: []string{"os/exec"}, BuildTags: "radiotutor_wine && linux"},
				{Path: "cmd/radio-tutor/w2_test.go", Imports: []string{"os/exec"}},
				{Path: "cmd/radio-tutor/w.go", Imports: []string{"os/exec"}, BuildTags: "radiotutor_wine"},
				{Path: "internal/gui/app/w_test.go", Imports: []string{"os/exec"}, BuildTags: "radiotutor_wine"},
				{Path: "scriptsfoo/main.go", Imports: []string{"os/exec"}},
			},
			want: []string{
				"cmd/radio-tutor/main.go: os/exec",
				"cmd/radio-tutor/w.go: os/exec",
				"cmd/radio-tutor/w2_test.go: os/exec",
				"internal/core/audio/other_test.go: os/exec",
				"internal/core/audio/plain.go: os/exec",
				"internal/core/storage/s_test.go: os/exec",
				"internal/gui/app/a.go: os/exec",
				"internal/gui/app/w_test.go: os/exec",
				"scriptsfoo/main.go: os/exec",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, v := range Check(tt.files) {
				if v.Rule == "" {
					t.Errorf("violation %+v has no rule", v)
				}
				got = append(got, v.File+": "+v.Import)
			}
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("violations:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

func TestScan(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod":                        {Data: []byte("module x\n")},
		"a/a.go":                        {Data: []byte("package a\n\nimport (\n\t\"os/exec\"\n\t_ \"net/http\"\n)\n")},
		"a/raw.go":                      {Data: []byte("package a\n\nimport `strings`\n")},
		"internal/core/audio/x_test.go": {Data: []byte("//go:build radiotutor_audio && linux\n\npackage audio\n\nimport \"os/exec\"\n")},
		"a/testdata/bad.go":             {Data: []byte("not go at all")},
		".hidden/bad.go":                {Data: []byte("not go at all")},
		"vendor/bad.go":                 {Data: []byte("not go at all")},
		"a/readme.md":                   {Data: []byte("# a")},
	}
	files, err := Scan(fsys)
	if err != nil {
		t.Fatal(err)
	}
	want := []File{
		{Path: "a/a.go", Imports: []string{"os/exec", "net/http"}},
		{Path: "a/raw.go", Imports: []string{"strings"}},
		{Path: "internal/core/audio/x_test.go", Imports: []string{"os/exec"}, BuildTags: "radiotutor_audio && linux"},
	}
	if len(files) != len(want) {
		t.Fatalf("scanned %d files, want %d: %+v", len(files), len(want), files)
	}
	for i, w := range want {
		g := files[i]
		if g.Path != w.Path || g.BuildTags != w.BuildTags || strings.Join(g.Imports, ",") != strings.Join(w.Imports, ",") {
			t.Errorf("file %d = %+v, want %+v", i, g, w)
		}
	}
	if v := Check(files); len(v) != 1 || v[0].File != "a/a.go" {
		t.Errorf("violations %v, want only a/a.go", v)
	}
}

// failingFS lists its files but cannot open the ones named in fail.
type failingFS struct {
	fs.FS
	fail map[string]bool
}

func (f failingFS) Open(name string) (fs.File, error) {
	if f.fail[name] {
		return nil, fs.ErrPermission
	}
	return f.FS.Open(name)
}

func TestScanErrors(t *testing.T) {
	tests := []struct {
		name      string
		fsys      fs.FS
		wantRoute string
	}{
		{
			name:      "unreadable root",
			fsys:      failingFS{FS: fstest.MapFS{"a.go": {Data: []byte("package a\n")}}, fail: map[string]bool{".": true}},
			wantRoute: "Scan.func1.ScanWalkFailed",
		},
		{
			name:      "unreadable file",
			fsys:      failingFS{FS: fstest.MapFS{"a.go": {Data: []byte("package a\n")}}, fail: map[string]bool{"a.go": true}},
			wantRoute: "Scan.func1.ScanReadFailed",
		},
		{
			name:      "syntax error",
			fsys:      fstest.MapFS{"a.go": {Data: []byte("package a\nimport (\n")}},
			wantRoute: "Scan.func1.ScanParseFailed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Scan(tt.fsys)
			if err == nil {
				t.Fatal("expected an error")
			}
			appErr, ok := errors.FromError(err)
			if !ok {
				t.Fatalf("expected *errors.Error, got %T", err)
			}
			if !appErr.HasRoute(tt.wantRoute) {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
		})
	}
}

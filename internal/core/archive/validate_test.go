package archive

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/problem"
)

func validate(t *testing.T, fsys fs.FS, render RenderChecker) Report {
	t.Helper()
	rep, err := Validate(context.Background(), fsys, render)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return rep
}

func TestValidateValid(t *testing.T) {
	render := &fakeRender{}
	rep := validate(t, validFS(t), render)
	if len(rep.Problems) != 0 || !rep.Valid() {
		t.Fatalf("problems:\n%s", listProblems(rep.Problems))
	}
	if rep.Cassette == nil || rep.Cassette.Manifest.ID != validID || len(rep.Cassette.Lessons) != 2 || len(rep.Cassette.Exercises) != 3 {
		t.Errorf("cassette = %+v", rep.Cassette)
	}
	// The renderer saw every picture: the layers of both scenes, the cover and
	// the prompt picture.
	wantNames := []string{
		lesson1SVG + "#static layer 1",
		lesson1SVG + "#svg/g[@id=pizza]",
		lesson1SVG + "#svg/g[@id=slice]",
		lesson1SVG + "#svg/g[@id=label-half]",
		promptSVG,
		lesson2Dir + "/scene.svg#static layer 1",
		lesson2Dir + "/scene.svg#svg/g[@id=sum]",
		"cover.svg",
	}
	if strings.Join(render.names, "\n") != strings.Join(wantNames, "\n") {
		t.Errorf("rendered:\n%s\nwant:\n%s", strings.Join(render.names, "\n"), strings.Join(wantNames, "\n"))
	}
}

func TestValidateFolderAndOpenDirFS(t *testing.T) {
	rep := validate(t, os.DirFS("testdata/valid"), nil)
	if len(rep.Problems) != 0 {
		t.Fatalf("problems:\n%s", listProblems(rep.Problems))
	}
}

func TestValidateEveryProblemAtOnce(t *testing.T) {
	fsys := validFS(t)
	delete(fsys, "cover.svg")
	fsys[lesson1SVG].Data = bytes.Replace(fsys[lesson1SVG].Data, []byte(`<g id="slice">`), []byte(`<script>x</script><g id="slice">`), 1)
	fsys[lesson1JSON].Data = bytes.Replace(fsys[lesson1JSON].Data, []byte(`"actor": "pizza"`), []byte(`"actor": "pizzaa"`), 1)
	fsys[lesson1OGG].Data = fsys[lesson1OGG].Data[:2000]
	fsys["exercises/shade-the-half/exercise.json"].Data = []byte(`{"format": 1}`)
	fsys["notes.txt"] = &fstest.MapFile{Data: []byte("x")}

	rep := validate(t, fsys, nil)
	checkProblems(t, rep.Problems,
		want{"notes.txt", "not part of a cassette", false},
		want{"cover.svg", "is missing", false},
		want{lesson1SVG + "#svg/script[1]", "script", false},
		want{lesson1JSON + "#tracks[0].actor", `no actor "pizzaa"`, false},
		want{lesson1OGG, "cannot be decoded as Ogg Vorbis", false},
		want{"exercises/shade-the-half/exercise.json#title", "required", false},
		want{"exercises/shade-the-half/exercise.json#prompt", "required", false},
		want{"exercises/shade-the-half/exercise.json#reference_solution", "required", false},
		want{"exercises/shade-the-half/exercise.json#grading", "required", false},
	)
	if rep.Valid() {
		t.Error("report is valid")
	}
}

func TestValidateFiles(t *testing.T) {
	tone16k, err := os.ReadFile("../audio/testdata/tone-mono-16000.ogg")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		mut  func(t *testing.T, fsys fstest.MapFS)
		want []want
	}{
		{"cover missing", func(t *testing.T, fsys fstest.MapFS) { delete(fsys, "cover.svg") },
			[]want{{"cover.svg", "is missing", false}}},
		{"cover has the wrong aspect ratio", func(t *testing.T, fsys fstest.MapFS) {
			fsys["cover.svg"].Data = bytes.Replace(fsys["cover.svg"].Data, []byte("0 0 300 400"), []byte("0 0 400 300"), 1)
		}, []want{{"cover.svg#svg/@viewBox", "3:4", true}}},
		{"cover is not svg", func(t *testing.T, fsys fstest.MapFS) { fsys["cover.svg"].Data = []byte("not xml") },
			[]want{{"cover.svg", "", false}}},
		{"cover too large", func(t *testing.T, fsys fstest.MapFS) { fsys["cover.svg"].Data = make([]byte, 5<<20+1) },
			[]want{{"cover.svg", "limit is 5242880 bytes (5 MiB)", false}}},
		{"scene missing", func(t *testing.T, fsys fstest.MapFS) { delete(fsys, lesson1SVG) },
			[]want{{lesson1SVG, "is missing", false}}},
		{"scene with no actors the tracks use", func(t *testing.T, fsys fstest.MapFS) {
			fsys[lesson1SVG].Data = bytes.ReplaceAll(fsys[lesson1SVG].Data, []byte(`<g id="slice">`), []byte(`<g id="other">`))
		}, []want{{lesson1JSON + "#tracks[1].actor", `no actor "slice"`, false}}},
		{"scene with a reveal box outside", func(t *testing.T, fsys fstest.MapFS) {
			fsys[lesson1JSON].Data = bytes.Replace(fsys[lesson1JSON].Data, []byte("[650, 720, 300, 110]"), []byte("[1500, 720, 300, 110]"), 1)
		}, []want{{lesson1JSON + "#tracks[2].reveal_box", "not inside the view box", false}}},
		{"scene forbidden attribute", func(t *testing.T, fsys fstest.MapFS) {
			fsys[lesson1SVG].Data = bytes.Replace(fsys[lesson1SVG].Data, []byte(`<rect `), []byte(`<rect onclick="x()" `), 1)
		}, []want{{lesson1SVG + "#svg/rect[1]/@onclick", "onclick", false}}},
		{"scene unusable", func(t *testing.T, fsys fstest.MapFS) { fsys[lesson1SVG].Data = []byte("<svg") },
			[]want{{lesson1SVG + "#line 1", "", false}}},
		{"narration missing", func(t *testing.T, fsys fstest.MapFS) { delete(fsys, lesson1OGG) },
			[]want{{lesson1OGG, "is missing", false}}},
		{"narration corrupt", func(t *testing.T, fsys fstest.MapFS) {
			fsys[lesson1OGG].Data = bytes.Repeat([]byte{0x55}, 3000)
		}, []want{{lesson1OGG, "cannot be decoded as Ogg Vorbis", false}}},
		{"narration with a rate that is not allowed", func(t *testing.T, fsys fstest.MapFS) { fsys[lesson1OGG].Data = tone16k },
			[]want{
				{lesson1OGG, "16000 Hz", false},
				{lesson1JSON + "#tracks[2].keys[1].t", "after the end of the narration", false},
				{lesson1JSON + "#tracks[2].keys[0].t", "after the end of the narration", false},
				{lesson1JSON + "#tracks[1].keys[1].t", "after the end of the narration", false},
				{lesson1JSON + "#captions[1].end", "after the end of the narration", false},
				{lesson1JSON + "#captions[2].end", "after the end of the narration", false},
			}},
		{"narration larger than 20 MiB", func(t *testing.T, fsys fstest.MapFS) { fsys[lesson1OGG].Data = make([]byte, 20<<20+1) },
			[]want{{lesson1OGG, "limit is 20971520 bytes (20 MiB)", false}}},
		{"a key after the end of the narration", func(t *testing.T, fsys fstest.MapFS) {
			fsys[lesson1JSON].Data = bytes.Replace(fsys[lesson1JSON].Data, []byte(`{"t": 2.8, "reveal": 1}`), []byte(`{"t": 3.5, "reveal": 1}`), 1)
		}, []want{{lesson1JSON + "#tracks[2].keys[1].t", "after the end of the narration", false}}},
		{"a caption after the end of the narration", func(t *testing.T, fsys fstest.MapFS) {
			fsys[lesson1JSON].Data = bytes.Replace(fsys[lesson1JSON].Data, []byte(`"end": 2.9`), []byte(`"end": 3.2`), 1)
		}, []want{{lesson1JSON + "#captions[2].end", "after the end of the narration", false}}},
		{"within the tolerance", func(t *testing.T, fsys fstest.MapFS) {
			fsys[lesson1JSON].Data = bytes.Replace(fsys[lesson1JSON].Data, []byte(`"end": 2.9`), []byte(`"end": 3.04`), 1)
		}, nil},
		{"prompt picture not valid", func(t *testing.T, fsys fstest.MapFS) {
			fsys[promptSVG].Data = bytes.Replace(fsys[promptSVG].Data, []byte("<circle"), []byte("<foreignObject/><circle"), 1)
		}, []want{{promptSVG + "#svg/foreignObject[1]", "foreignObject", false}}},
		{"prompt audio is too long for a prompt", func(t *testing.T, fsys fstest.MapFS) {
			fsys[promptOGG].Data = fsys[lesson1OGG].Data // 3 s is fine; see the next case
		}, nil},
		{"prompt audio corrupt", func(t *testing.T, fsys fstest.MapFS) {
			fsys[promptOGG].Data = []byte("OggS but not really")
		}, []want{{promptOGG, "cannot be decoded", false}}},
		{"prompt audio with a rate that is not allowed", func(t *testing.T, fsys fstest.MapFS) { fsys[promptOGG].Data = tone16k },
			[]want{{promptOGG, "16000 Hz", false}}},
		{"optional files may be missing", func(t *testing.T, fsys fstest.MapFS) {
			delete(fsys, promptSVG)
			delete(fsys, promptOGG)
		}, nil},
		{"an extra file", func(t *testing.T, fsys fstest.MapFS) { fsys["README.md"] = &fstest.MapFile{Data: []byte("hi")} },
			[]want{{`"README.md"...`, "characters", false}}},
		{"an extra lowercase file", func(t *testing.T, fsys fstest.MapFS) {
			fsys["lessons/what-is-a-fraction/notes.txt"] = &fstest.MapFile{Data: []byte("hi")}
		}, []want{{"lessons/what-is-a-fraction/notes.txt", "not part of a cassette", false}}},
		{"a symbolic link", func(t *testing.T, fsys fstest.MapFS) {
			fsys["cover.svg"] = &fstest.MapFile{Data: []byte("/etc/passwd"), Mode: fs.ModeSymlink | 0o777}
		}, []want{{"cover.svg", "symbolic link", false}, {"cover.svg", "is missing", false}}},
		{"an unlisted lesson folder", func(t *testing.T, fsys fstest.MapFS) {
			fsys["lessons/stray/lesson.json"] = &fstest.MapFile{Data: []byte("{}")}
		}, []want{{"lessons/stray", "not listed", false}}},
		{"a lesson folder that is missing", func(t *testing.T, fsys fstest.MapFS) {
			for name := range fsys {
				if strings.HasPrefix(name, lesson2Dir+"/") {
					delete(fsys, name)
				}
			}
		}, []want{{"cassette.json#items[3].id", "has no folder", false}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := validFS(t)
			tt.mut(t, fsys)
			rep := validate(t, fsys, nil)
			// A path given without a message part matches any message.
			checkLoose(t, rep.Problems, tt.want)
		})
	}
}

// checkLoose is checkProblems for a slice; an empty message part matches any
// message.
func checkLoose(t *testing.T, ps []problem.Problem, wants []want) {
	t.Helper()
	checkProblems(t, ps, wants...)
}

func TestValidateManifestUnusable(t *testing.T) {
	fsys := validFS(t)
	fsys["cassette.json"].Data = []byte("[]")
	delete(fsys, "cover.svg")
	rep := validate(t, fsys, nil)
	if rep.Cassette != nil {
		t.Error("cassette is not nil")
	}
	checkLoose(t, rep.Problems, []want{
		{"cassette.json", "", false},
		{"cover.svg", "is missing", false},
	})
}

func TestValidateRenderChecker(t *testing.T) {
	tests := []struct {
		name   string
		failOn string
		want   []want
	}{
		{"an actor layer", "g[@id=slice]", []want{{lesson1SVG + "#svg/g[@id=slice]", "Radio Tutor cannot draw it: the fake renderer refuses this picture", false}}},
		{"a static layer", "static layer 1", []want{
			{lesson1SVG + "#static layer 1", "cannot draw it", false},
			{lesson2Dir + "/scene.svg#static layer 1", "cannot draw it", false},
		}},
		{"the cover", "cover.svg", []want{{"cover.svg", "cannot draw it", false}}},
		{"a prompt picture", "prompt.svg", []want{{promptSVG, "cannot draw it", false}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rep := validate(t, validFS(t), &fakeRender{failOn: tt.failOn})
			checkLoose(t, rep.Problems, tt.want)
		})
	}

	t.Run("the renderer is not asked about a scene that has errors", func(t *testing.T) {
		fsys := validFS(t)
		fsys[lesson1SVG].Data = bytes.Replace(fsys[lesson1SVG].Data, []byte(`<rect `), []byte(`<rect onclick="x()" `), 1)
		render := &fakeRender{}
		validate(t, fsys, render)
		for _, n := range render.names {
			if strings.HasPrefix(n, lesson1SVG) {
				t.Errorf("rendered %s", n)
			}
		}
	})

	t.Run("a cancelled renderer cancels the validation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_, err := Validate(ctx, validFS(t), &fakeRender{cancel: cancel})
		hasRoute(t, err, "validator.renderOne->cancelCheck.ArchiveCanceled")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("not canceled: %v", err)
		}
	})
}

func TestValidateCancelledAtEveryStep(t *testing.T) {
	fsys := validFS(t)
	steps := cancelAtSteps(t, 3, func(ctx context.Context) error {
		_, err := Validate(ctx, fsys, &fakeRender{})
		return err
	})
	if steps < 10 {
		t.Errorf("only %d cancellation points", steps)
	}
}

func TestValidateReadFailures(t *testing.T) {
	t.Run("a folder that cannot be read", func(t *testing.T) {
		_, err := Validate(context.Background(), failingFS{valid: validFS(t), failDir: true}, nil)
		hasRoute(t, err, "scanFS.func1.ArchiveFolderReadFailed")
	})
	t.Run("a file that cannot be opened", func(t *testing.T) {
		_, err := Validate(context.Background(), failingFS{valid: validFS(t), failOpen: lesson1SVG}, nil)
		hasRoute(t, err, "validator.readSVG")
		hasRoute(t, err, "ArchiveEntryOpenFailed")
	})
	t.Run("an audio file that cannot be opened", func(t *testing.T) {
		_, err := Validate(context.Background(), failingFS{valid: validFS(t), failOpen: lesson1OGG}, nil)
		hasRoute(t, err, "ArchiveEntryOpenFailed")
	})
	t.Run("a file that changes size after the scan", func(t *testing.T) {
		fsys := validFS(t)
		// The scan sees the size of the file in the listing; the content
		// is longer when it is read.
		shrunk := failingFS{valid: fsys, lie: map[string]int64{"cover.svg": 10}}
		_, err := Validate(context.Background(), shrunk, nil)
		hasRoute(t, err, "ArchiveEntryTooLarge")
	})
	t.Run("a file with an entry that is not a file or a folder", func(t *testing.T) {
		_, err := Validate(context.Background(), failingFS{valid: validFS(t), failInfo: true}, nil)
		hasRoute(t, err, "scanFS.func1.ArchiveFolderReadFailed")
	})
}

// failingFS wraps a MapFS and fails or lies at chosen points.
type failingFS struct {
	valid    fstest.MapFS
	failDir  bool             // ReadDir of the root fails
	failOpen string           // Open of this file fails
	lie      map[string]int64 // Info of these files says another size
	failInfo bool             // Info of an entry fails
}

func (f failingFS) Open(name string) (fs.File, error) {
	if name == f.failOpen {
		return nil, os.ErrPermission
	}
	if f.failDir && name == "." {
		return failingDir{}, nil
	}
	file, err := f.valid.Open(name)
	if err != nil {
		return nil, err
	}
	if dir, ok := file.(fs.ReadDirFile); ok && (f.lie != nil || f.failInfo) {
		return liarDir{ReadDirFile: dir, lie: f.lie, failInfo: f.failInfo}, nil
	}
	return file, nil
}

type failingDir struct{}

func (failingDir) Stat() (fs.FileInfo, error)         { return nil, os.ErrPermission }
func (failingDir) Read([]byte) (int, error)           { return 0, os.ErrPermission }
func (failingDir) Close() error                       { return nil }
func (failingDir) ReadDir(int) ([]fs.DirEntry, error) { return nil, os.ErrPermission }

// liarDir lists entries whose Info says another size, or fails.
type liarDir struct {
	fs.ReadDirFile
	lie      map[string]int64
	failInfo bool
}

func (d liarDir) ReadDir(n int) ([]fs.DirEntry, error) {
	entries, err := d.ReadDirFile.ReadDir(n)
	for i, e := range entries {
		entries[i] = liarEntry{DirEntry: e, lie: d.lie, failInfo: d.failInfo && !e.IsDir()}
	}
	return entries, err
}

type liarEntry struct {
	fs.DirEntry
	lie      map[string]int64
	failInfo bool
}

func (e liarEntry) Info() (fs.FileInfo, error) {
	if e.failInfo {
		return nil, os.ErrPermission
	}
	info, err := e.DirEntry.Info()
	if err != nil {
		return nil, err
	}
	if size, ok := e.lie[e.Name()]; ok {
		return sizedInfo{FileInfo: info, size: size}, nil
	}
	return info, nil
}

type sizedInfo struct {
	fs.FileInfo
	size int64
}

func (i sizedInfo) Size() int64 { return i.size }

func TestValidatePath(t *testing.T) {
	t.Run("a cassette file", func(t *testing.T) {
		path := packFS(t, validFS(t))
		rep, err := ValidatePath(context.Background(), path, &fakeRender{})
		if err != nil || len(rep.Problems) != 0 {
			t.Fatalf("%v\n%s", err, listProblems(rep.Problems))
		}
	})
	t.Run("a folder", func(t *testing.T) {
		rep, err := ValidatePath(context.Background(), "testdata/valid", nil)
		if err != nil || len(rep.Problems) != 0 || rep.Cassette == nil {
			t.Fatalf("%v\n%s", err, listProblems(rep.Problems))
		}
	})
	t.Run("problems of opening come first", func(t *testing.T) {
		fsys := validFS(t)
		delete(fsys, "cover.svg")
		entries := zipOf(t, fsys, zent{name: "../evil", data: []byte("x")}, zent{name: "README.md", data: []byte("x")})
		rep, err := ValidatePath(context.Background(), buildZip(t, entries...), nil)
		if err != nil {
			t.Fatal(err)
		}
		checkLoose(t, rep.Problems, []want{
			{"../evil", ".. part", false},
			{`"README.md"...`, "characters", false},
			{"cover.svg", "is missing", false},
		})
		if rep.Problems[len(rep.Problems)-1].Path != "cover.svg" {
			t.Errorf("order:\n%s", listProblems(rep.Problems))
		}
	})
	t.Run("a zip that is refused as a whole", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "x.cassette")
		if err := os.WriteFile(path, []byte("not a zip"), 0o644); err != nil {
			t.Fatal(err)
		}
		rep, err := ValidatePath(context.Background(), path, nil)
		if err != nil || len(rep.Problems) != 1 || rep.Cassette != nil {
			t.Fatalf("%v\n%s", err, listProblems(rep.Problems))
		}
	})
	t.Run("a path that does not exist", func(t *testing.T) {
		_, err := ValidatePath(context.Background(), filepath.Join(t.TempDir(), "nope"), nil)
		hasRoute(t, err, "ValidatePath->openPath.ArchivePathStatFailed")
	})
	t.Run("cancelled at every step", func(t *testing.T) {
		path := packFS(t, validFS(t))
		cancelAtSteps(t, 5, func(ctx context.Context) error {
			_, err := ValidatePath(ctx, path, &fakeRender{})
			return err
		})
	})
}

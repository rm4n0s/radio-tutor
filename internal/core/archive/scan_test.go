package archive

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestScanFolder(t *testing.T) {
	good := []byte("x")
	t.Run("a valid folder is the same as the view", func(t *testing.T) {
		tree, ps, fatal, err := scanFS(context.Background(), os.DirFS("testdata/valid"))
		if err != nil || fatal || len(ps) != 0 {
			t.Fatalf("%v %v %v", err, fatal, ps)
		}
		if _, err := fs.Stat(tree, "cassette.json"); err != nil {
			t.Error(err)
		}
	})

	t.Run("symbolic links in a real folder", func(t *testing.T) {
		dir := writeTree(t, validFS(t))
		outside := filepath.Join(t.TempDir(), "secret.txt")
		if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(dir, "cover.svg")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(dir, "cover.svg")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(t.TempDir(), filepath.Join(dir, "lessons", "linked")); err != nil {
			t.Fatal(err)
		}
		tree, ps, fatal, err := scanFS(context.Background(), os.DirFS(dir))
		if err != nil || fatal {
			t.Fatalf("%v %v", err, fatal)
		}
		checkProblems(t, ps,
			want{"cover.svg", "symbolic link", false},
			want{"lessons/linked", "symbolic link", false})
		if _, err := fs.Stat(tree, "cover.svg"); err == nil {
			t.Error("the link is in the view")
		}
		// And Validate does not follow it either.
		rep, err := ValidatePath(context.Background(), dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		checkLoose(t, rep.Problems, []want{
			{"cover.svg", "symbolic link", false},
			{"lessons/linked", "symbolic link", false},
			{"cover.svg", "is missing", false},
		})
	})

	t.Run("entries that are not allowed", func(t *testing.T) {
		fsys := validFS(t)
		fsys["README.md"] = &fstest.MapFile{Data: good}
		fsys[".DS_Store"] = &fstest.MapFile{Data: good}
		fsys["__MACOSX/._cover.svg"] = &fstest.MapFile{Data: good}
		fsys["lessons/what-is-a-fraction/deep/er/file.json"] = &fstest.MapFile{Data: good}
		fsys["lessons/what-is-a-fraction/notes.txt"] = &fstest.MapFile{Data: good}
		_, ps, fatal, err := scanFS(context.Background(), fsys)
		if err != nil || fatal {
			t.Fatalf("%v %v", err, fatal)
		}
		checkProblems(t, ps,
			want{`"README.md"...`, "characters", false},
			want{`".DS_Store"...`, "characters", false},
			want{`"__MACOSX"...`, "characters", false},
			want{"lessons/what-is-a-fraction/deep", "not part of a cassette", false},
			want{"lessons/what-is-a-fraction/notes.txt", "not part of a cassette", false},
		)
	})

	t.Run("special files in an fs", func(t *testing.T) {
		fsys := validFS(t)
		fsys["lessons/what-is-a-fraction/narration.ogg"] = &fstest.MapFile{Mode: fs.ModeDevice | 0o600}
		fsys["lessons/adding-fractions/scene.svg"] = &fstest.MapFile{Mode: fs.ModeNamedPipe | 0o600}
		_, ps, _, err := scanFS(context.Background(), fsys)
		if err != nil {
			t.Fatal(err)
		}
		checkProblems(t, ps,
			want{lesson1OGG, "special file", false},
			want{lesson2Dir + "/scene.svg", "special file", false})
	})

	t.Run("too many entries", func(t *testing.T) {
		fsys := fstest.MapFS{}
		for i := 0; i <= MaxEntries; i++ {
			fsys[fmt.Sprintf("lessons/l1/f%04d", i)] = &fstest.MapFile{Data: good}
		}
		tree, ps, fatal, err := scanFS(context.Background(), fsys)
		if err != nil || !fatal {
			t.Fatalf("%v %v", err, fatal)
		}
		checkProblems(t, ps, want{"", "more than 2000 entries", false})
		if entries, _ := fs.ReadDir(tree, "."); len(entries) != 0 {
			t.Error("the view is not empty")
		}
	})

	t.Run("too many entries counted in links too", func(t *testing.T) {
		fsys := fstest.MapFS{}
		for i := 0; i <= MaxEntries; i++ {
			fsys[fmt.Sprintf("lessons/l1/f%04d", i)] = &fstest.MapFile{Mode: fs.ModeSymlink | 0o777}
		}
		_, ps, fatal, err := scanFS(context.Background(), fsys)
		if err != nil || !fatal {
			t.Fatalf("%v %v", err, fatal)
		}
		checkProblems(t, ps, want{"", "more than 2000 entries", false})
	})

	t.Run("sizes of real sparse files", func(t *testing.T) {
		dir := t.TempDir()
		sparse := func(name string, size int64) {
			t.Helper()
			p := filepath.Join(dir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(p)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.Truncate(size); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
		sparse("lessons/a/narration.ogg", MaxEntryBytes+1)
		_, ps, fatal, err := scanFS(context.Background(), os.DirFS(dir))
		if err != nil || fatal {
			t.Fatalf("%v %v", err, fatal)
		}
		checkProblems(t, ps, want{"lessons/a/narration.ogg", "limit for one file", false})

		// Six files of 190 MiB are more than 1 GiB together.
		dir = t.TempDir()
		for i := 0; i < 2; i++ {
			sparse(fmt.Sprintf("lessons/l%d/narration.ogg", i), 190<<20)
			sparse(fmt.Sprintf("lessons/l%d/scene.svg", i), 190<<20)
			sparse(fmt.Sprintf("lessons/l%d/lesson.json", i), 190<<20)
		}
		_, ps, fatal, err = scanFS(context.Background(), os.DirFS(dir))
		if err != nil || !fatal {
			t.Fatalf("%v %v", err, fatal)
		}
		checkProblems(t, ps, want{"", "1 GiB", false})
	})

	t.Run("a view caps what it reads", func(t *testing.T) {
		// The listing says 3 bytes, the file has more: the read fails.
		fsys := failingFS{valid: fstest.MapFS{"cover.svg": {Data: []byte("0123456789")}}, lie: map[string]int64{"cover.svg": 3}}
		tree, ps, _, err := scanFS(context.Background(), fsys)
		if err != nil || len(ps) != 0 {
			t.Fatalf("%v %v", err, ps)
		}
		_, err = fs.ReadFile(tree, "cover.svg")
		hasRoute(t, err, "capReader.Read.ArchiveEntryTooLarge")
	})

	t.Run("cancelled", func(t *testing.T) {
		cancelAtEveryStep(t, func(ctx context.Context) error {
			_, _, _, err := scanFS(ctx, validFS(t))
			return err
		})
	})
}

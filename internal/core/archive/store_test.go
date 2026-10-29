package archive

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
)

func newTestStore(t *testing.T) (*Store, storage.Paths) {
	t.Helper()
	paths := storagetest.Isolate(t)
	return NewStore(paths, &fakeRender{}), paths
}

func ver(major, minor, patch int) cassette.Version {
	return cassette.Version{Major: major, Minor: minor, Patch: patch}
}

func always(old, new cassette.Version) bool { return true }

// importVersion packs the valid cassette with the version and imports it.
func importVersion(t *testing.T, s *Store, version string, confirm func(old, new cassette.Version) bool) ImportResult {
	t.Helper()
	res, err := s.Import(context.Background(), packFS(t, withVersion(t, version)), confirm)
	if err != nil {
		t.Fatalf("Import %s: %v", version, err)
	}
	return res
}

// leftovers lists the entries of the cassettes folder that are not installed
// cassettes (the temporary and set-aside folders).
func leftovers(t *testing.T, paths storage.Paths) []string {
	t.Helper()
	entries, err := os.ReadDir(paths.CassettesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if strings.Contains(e.Name(), tmpMarker) || strings.Contains(e.Name(), oldMarker) {
			out = append(out, e.Name())
		}
	}
	return out
}

func installedVersion(t *testing.T, s *Store) string {
	t.Helper()
	_, m, err := s.Get(context.Background(), validID)
	if err != nil {
		t.Fatal(err)
	}
	return m.Version.String()
}

func TestStoreImportAndGet(t *testing.T) {
	s, paths := newTestStore(t)
	if list, err := s.List(context.Background()); err != nil || len(list) != 0 {
		t.Fatalf("empty store: %v %v", list, err)
	}

	res := importVersion(t, s, "1.0.0", nil)
	if res.Status != ImportInstalled || res.ID != validID || res.Version != ver(1, 0, 0) || len(res.Problems) != 0 {
		t.Fatalf("result = %+v", res)
	}
	if left := leftovers(t, paths); len(left) != 0 {
		t.Errorf("leftovers: %v", left)
	}

	list, err := s.List(context.Background())
	if err != nil || len(list) != 1 || list[0].ID != validID || list[0].Broken || list[0].Manifest.Title != "Fractions, part 1" {
		t.Fatalf("List = %+v, %v", list, err)
	}

	view, m, err := s.Get(context.Background(), validID)
	if err != nil || m.ID != validID {
		t.Fatalf("Get: %v", err)
	}
	original := validFS(t)
	for name, f := range original {
		if name == "cassette.json" {
			continue // the version was changed by withVersion
		}
		got, err := fs.ReadFile(view, name)
		if err != nil || !bytes.Equal(got, f.Data) {
			t.Errorf("%s differs: %v", name, err)
		}
	}
	// The installed cassette is valid by itself.
	rep, err := ValidatePath(context.Background(), filepath.Join(paths.CassettesDir(), validID), nil)
	if err != nil || len(rep.Problems) != 0 {
		t.Errorf("%v\n%s", err, listProblems(rep.Problems))
	}
	// Modes: folders 0700, files 0600.
	if err := filepath.WalkDir(filepath.Join(paths.CassettesDir(), validID), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		want := fs.FileMode(0o600)
		if d.IsDir() {
			want = 0o700
		}
		if info.Mode().Perm() != want {
			t.Errorf("%s has mode %v, want %v", p, info.Mode().Perm(), want)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestStoreImportFromFolder(t *testing.T) {
	s, _ := newTestStore(t)
	res, err := s.Import(context.Background(), "testdata/valid", nil)
	if err != nil || res.Status != ImportInstalled {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestStoreVersions(t *testing.T) {
	type step struct {
		version string
		confirm func(old, new cassette.Version) bool
		want    ImportStatus
		prev    cassette.Version
		after   string // the installed version after the step
	}
	var asked [][2]cassette.Version
	record := func(answer bool) func(old, new cassette.Version) bool {
		return func(old, new cassette.Version) bool {
			asked = append(asked, [2]cassette.Version{old, new})
			return answer
		}
	}
	steps := []step{
		{"1.2.0", nil, ImportInstalled, ver(0, 0, 0), "1.2.0"},
		{"1.2.0", always, ImportAlreadyInstalled, ver(1, 2, 0), "1.2.0"},
		{"1.1.9", always, ImportNewerInstalled, ver(1, 2, 0), "1.2.0"},
		{"0.9.9", always, ImportNewerInstalled, ver(1, 2, 0), "1.2.0"},
		{"1.10.0", record(false), ImportDeclined, ver(1, 2, 0), "1.2.0"},
		{"1.10.0", nil, ImportDeclined, ver(1, 2, 0), "1.2.0"},
		{"1.10.0", record(true), ImportUpgraded, ver(1, 2, 0), "1.10.0"},
		{"2.0.0", record(true), ImportUpgraded, ver(1, 10, 0), "2.0.0"},
	}
	s, paths := newTestStore(t)
	for _, st := range steps {
		res := importVersion(t, s, st.version, st.confirm)
		if res.Status != st.want || res.Previous != st.prev || res.ID != validID {
			t.Errorf("import %s: %+v, want status %v previous %v", st.version, res, st.want, st.prev)
		}
		if got := installedVersion(t, s); got != st.after {
			t.Errorf("after import %s the installed version is %s, want %s", st.version, got, st.after)
		}
		if left := leftovers(t, paths); len(left) != 0 {
			t.Errorf("leftovers: %v", left)
		}
	}
	wantAsked := [][2]cassette.Version{{ver(1, 2, 0), ver(1, 10, 0)}, {ver(1, 2, 0), ver(1, 10, 0)}, {ver(1, 10, 0), ver(2, 0, 0)}}
	// record(false) ran once, nil never, record(true) twice.
	if len(asked) != 3 || asked[0] != wantAsked[0] || asked[1] != wantAsked[1] || asked[2] != wantAsked[2] {
		t.Errorf("asked = %v", asked)
	}
}

func TestStoreUpgradeReplacesEverything(t *testing.T) {
	s, paths := newTestStore(t)
	importVersion(t, s, "1.0.0", nil)
	// A file that only the old version has.
	old := filepath.Join(paths.CassettesDir(), validID, "old-only.txt")
	if err := os.WriteFile(old, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	fsys := withVersion(t, "1.1.0")
	fsys["cover.svg"].Data = bytes.Replace(fsys["cover.svg"].Data, []byte("#3b6ea5"), []byte("#aa0000"), 1)
	res, err := s.Import(context.Background(), packFS(t, fsys), always)
	if err != nil || res.Status != ImportUpgraded {
		t.Fatalf("%+v %v", res, err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("the old folder's file is still there: %v", err)
	}
	view, _, err := s.Get(context.Background(), validID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := fs.ReadFile(view, "cover.svg")
	if err != nil || !bytes.Contains(data, []byte("#aa0000")) {
		t.Errorf("cover.svg is the old one: %v", err)
	}
}

func TestStoreImportInvalid(t *testing.T) {
	s, paths := newTestStore(t)
	tests := []struct {
		name string
		path func(t *testing.T) string
		want []want
		id   string
	}{
		{"extra file", func(t *testing.T) string {
			fsys := validFS(t)
			return buildZip(t, zipOf(t, fsys, zent{name: "readme.md", data: []byte("x")})...)
		}, []want{{"readme.md", "not part of a cassette", false}}, validID},
		{"zip-slip", func(t *testing.T) string {
			return buildZip(t, zipOf(t, validFS(t), zent{name: "../../evil", data: []byte("x")})...)
		}, []want{{"../../evil", ".. part", false}}, validID},
		{"scene with a script", func(t *testing.T) string {
			fsys := validFS(t)
			fsys[lesson1SVG].Data = bytes.Replace(fsys[lesson1SVG].Data, []byte("<rect "), []byte("<script/><rect "), 1)
			return buildZip(t, zipOf(t, fsys)...)
		}, []want{{lesson1SVG + "#svg/script[1]", "script", false}}, validID},
		{"missing narration", func(t *testing.T) string {
			fsys := validFS(t)
			delete(fsys, lesson1OGG)
			return buildZip(t, zipOf(t, fsys)...)
		}, []want{{lesson1OGG, "is missing", false}}, validID},
		{"manifest unusable", func(t *testing.T) string {
			fsys := validFS(t)
			fsys["cassette.json"].Data = []byte("{")
			return buildZip(t, zipOf(t, fsys)...)
		}, []want{{"cassette.json", "", false}}, ""},
		{"not a zip", func(t *testing.T) string {
			p := filepath.Join(t.TempDir(), "x.cassette")
			if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
				t.Fatal(err)
			}
			return p
		}, []want{{"", "not a zip", false}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := s.Import(context.Background(), tt.path(t), always)
			if err != nil {
				t.Fatal(err)
			}
			if res.Status != ImportInvalid || res.ID != "" {
				t.Fatalf("result = %+v", res)
			}
			checkLoose(t, res.Problems, tt.want)
			if list, _ := s.List(context.Background()); len(list) != 0 {
				t.Errorf("installed: %v", list)
			}
			if _, err := os.Stat(paths.CassettesDir()); err == nil {
				if left, _ := os.ReadDir(paths.CassettesDir()); len(left) != 0 {
					t.Errorf("the cassettes folder is not empty: %v", left)
				}
			}
		})
	}

	t.Run("the renderer refuses a picture", func(t *testing.T) {
		s := NewStore(paths, &fakeRender{failOn: "cover.svg"})
		res, err := s.Import(context.Background(), "testdata/valid", nil)
		if err != nil || res.Status != ImportInvalid {
			t.Fatalf("%+v %v", res, err)
		}
		checkLoose(t, res.Problems, []want{{"cover.svg", "cannot draw it", false}})
	})
}

func TestStoreImportWarnings(t *testing.T) {
	s, _ := newTestStore(t)
	fsys := validFS(t)
	fsys["cover.svg"].Data = bytes.Replace(fsys["cover.svg"].Data, []byte("0 0 300 400"), []byte("0 0 400 300"), 1)
	res, err := s.Import(context.Background(), buildZip(t, zipOf(t, fsys)...), nil)
	if err != nil || res.Status != ImportInstalled {
		t.Fatalf("%+v %v", res, err)
	}
	checkLoose(t, res.Problems, []want{{"cover.svg#svg/@viewBox", "3:4", true}})
}

func TestStoreImportErrors(t *testing.T) {
	t.Run("a file that does not exist", func(t *testing.T) {
		s, _ := newTestStore(t)
		_, err := s.Import(context.Background(), filepath.Join(t.TempDir(), "nope.cassette"), nil)
		hasRoute(t, err, "Store.Import->openPath.ArchivePathStatFailed")
	})
	t.Run("cancelled at every step", func(t *testing.T) {
		path := packFS(t, withVersion(t, "1.1.0"))
		s, paths := newTestStore(t)
		importVersion(t, s, "1.0.0", nil)
		steps := cancelAtSteps(t, 4, func(ctx context.Context) error {
			res, err := s.Import(ctx, path, always)
			if err != nil {
				if left := leftovers(t, paths); len(left) != 0 {
					t.Errorf("leftovers after a cancelled import: %v", left)
				}
				if v := installedVersion(t, s); v != "1.0.0" {
					t.Errorf("the installed version changed to %s", v)
				}
				return err
			}
			if res.Status != ImportUpgraded {
				t.Errorf("status = %v", res.Status)
			}
			return nil
		})
		if steps < 15 {
			t.Errorf("only %d cancellation points", steps)
		}
	})
	t.Run("cancelled while the confirmation is asked", func(t *testing.T) {
		s, paths := newTestStore(t)
		importVersion(t, s, "1.0.0", nil)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_, err := s.Import(ctx, packFS(t, withVersion(t, "1.1.0")), func(old, new cassette.Version) bool {
			cancel()
			return true
		})
		hasRoute(t, err, "ArchiveCanceled")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("not canceled: %v", err)
		}
		if left := leftovers(t, paths); len(left) != 0 {
			t.Errorf("leftovers: %v", left)
		}
		if v := installedVersion(t, s); v != "1.0.0" {
			t.Errorf("the installed version is %s", v)
		}
	})
	t.Run("the installed cassette changes while the confirmation is asked", func(t *testing.T) {
		s, paths := newTestStore(t)
		importVersion(t, s, "1.0.0", nil)
		_, err := s.Import(context.Background(), packFS(t, withVersion(t, "1.5.0")), func(old, new cassette.Version) bool {
			// Another import installs 1.2.0 meanwhile.
			res, err := s.Import(context.Background(), packFS(t, withVersion(t, "1.2.0")), always)
			if err != nil || res.Status != ImportUpgraded {
				t.Errorf("inner import: %+v %v", res, err)
			}
			return true
		})
		hasRoute(t, err, "Store.Import.StoreChangedDuringImport")
		if v := installedVersion(t, s); v != "1.2.0" {
			t.Errorf("the installed version is %s", v)
		}
		if left := leftovers(t, paths); len(left) != 0 {
			t.Errorf("leftovers: %v", left)
		}
	})
	t.Run("the same version is installed meanwhile", func(t *testing.T) {
		s, _ := newTestStore(t)
		importVersion(t, s, "1.0.0", nil)
		path := packFS(t, withVersion(t, "2.0.0"))
		res, err := s.Import(context.Background(), path, func(old, new cassette.Version) bool {
			inner, err := s.Import(context.Background(), path, always)
			if err != nil || inner.Status != ImportUpgraded {
				t.Errorf("inner = %+v, %v", inner, err)
			}
			return true
		})
		if err != nil || res.Status != ImportAlreadyInstalled || res.Previous != ver(2, 0, 0) {
			t.Errorf("outer = %+v, %v", res, err)
		}
	})
	t.Run("the installed cassette is removed meanwhile", func(t *testing.T) {
		s, _ := newTestStore(t)
		importVersion(t, s, "1.0.0", nil)
		res, err := s.Import(context.Background(), packFS(t, withVersion(t, "2.0.0")), func(old, new cassette.Version) bool {
			if err := s.Remove(context.Background(), validID); err != nil {
				t.Error(err)
			}
			return true
		})
		if err != nil || res.Status != ImportInstalled {
			t.Errorf("%+v, %v", res, err)
		}
		if v := installedVersion(t, s); v != "2.0.0" {
			t.Errorf("version = %s", v)
		}
	})
	t.Run("the cassettes folder cannot be created", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can write anywhere")
		}
		paths := storagetest.Isolate(t)
		if err := os.MkdirAll(paths.Data, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(paths.Data, 0o700) })
		s := NewStore(paths, nil)
		_, err := s.Import(context.Background(), "testdata/valid", nil)
		hasRoute(t, err, "DirCreateFailed")
	})
	t.Run("the data folder is a file", func(t *testing.T) {
		paths := storagetest.Isolate(t)
		if err := os.WriteFile(paths.Data, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		s := NewStore(paths, nil)
		_, err := s.Import(context.Background(), "testdata/valid", nil)
		hasRoute(t, err, "Store.Import->Store.lockedState->Store.stateOf.StoreStatFailed")
	})
}

func TestStoreBrokenEntries(t *testing.T) {
	s, paths := newTestStore(t)
	importVersion(t, s, "1.0.0", nil)
	mk := func(id, manifest string) {
		t.Helper()
		dir := filepath.Join(paths.CassettesDir(), id)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if manifest != "" {
			if err := os.WriteFile(filepath.Join(dir, "cassette.json"), []byte(manifest), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	mk("org.broken.garbage", "{ this is not json")
	mk("org.broken.nomanifest", "")
	mk("org.broken.otherid", `{"format":1,"id":"org.example.somethingelse","version":"1.0.0","title":"T","language":"en","subject":"s","ages":[8,9],"items":[{"type":"lesson","id":"a"}]}`)
	mk("org.broken.badfields", `{"format":1,"id":"org.broken.badfields","version":"1.0.0","title":"","language":"en","subject":"s","ages":[8,9],"items":[{"type":"lesson","id":"a"}]}`)
	// Not listed: a file, a folder that is not an id, temporary folders.
	if err := os.WriteFile(filepath.Join(paths.CassettesDir(), "org.file.notafolder"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	mk("Not An Id", "{}")
	mk(validID+tmpMarker+"abc", "{}")
	mk(validID+oldMarker+"abc", "{}")

	list, err := s.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, in := range list {
		got = append(got, in.ID+"="+map[bool]string{true: "broken", false: "ok"}[in.Broken])
		if in.Broken && len(in.Problems) == 0 {
			t.Errorf("%s is broken without a problem", in.ID)
		}
		if !in.Broken && in.Manifest.ID != in.ID {
			t.Errorf("%s has manifest %q", in.ID, in.Manifest.ID)
		}
	}
	want := []string{"org.broken.badfields=broken", "org.broken.garbage=broken", "org.broken.nomanifest=broken", "org.broken.otherid=broken", validID + "=ok"}
	if !slices.Equal(got, want) {
		t.Errorf("List = %v, want %v", got, want)
	}

	t.Run("Get refuses a broken one", func(t *testing.T) {
		for _, id := range []string{"org.broken.garbage", "org.broken.otherid"} {
			_, _, err := s.Get(context.Background(), id)
			hasRoute(t, err, "Store.Get.StoreBroken")
		}
	})
	t.Run("a broken one is replaced by an import without asking", func(t *testing.T) {
		mk(validID+"x", "")
		if err := os.RemoveAll(filepath.Join(paths.CassettesDir(), validID)); err != nil {
			t.Fatal(err)
		}
		mk(validID, "{ broken")
		res := importVersion(t, s, "1.0.0", nil)
		if res.Status != ImportInstalled {
			t.Errorf("status = %v", res.Status)
		}
		if v := installedVersion(t, s); v != "1.0.0" {
			t.Errorf("version = %s", v)
		}
	})
	t.Run("a broken one can be removed", func(t *testing.T) {
		if err := s.Remove(context.Background(), "org.broken.garbage"); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(paths.CassettesDir(), "org.broken.garbage")); !os.IsNotExist(err) {
			t.Errorf("still there: %v", err)
		}
	})
}

func TestStoreRemove(t *testing.T) {
	s, paths := newTestStore(t)
	importVersion(t, s, "1.0.0", nil)
	if err := s.Remove(context.Background(), validID); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.List(context.Background()); len(list) != 0 {
		t.Errorf("List = %v", list)
	}
	if left := leftovers(t, paths); len(left) != 0 {
		t.Errorf("leftovers: %v", left)
	}
	_, _, err := s.Get(context.Background(), validID)
	hasRoute(t, err, "Store.Get.StoreNotInstalled")
	err = s.Remove(context.Background(), validID)
	hasRoute(t, err, "Store.Remove.StoreNotInstalled")
	err = s.Remove(context.Background(), "../etc")
	hasRoute(t, err, "Store.Remove.StoreIDInvalid")
	_, _, err = s.Get(context.Background(), "a/b")
	hasRoute(t, err, "Store.Get.StoreIDInvalid")

	// It can be installed again.
	if res := importVersion(t, s, "1.0.0", nil); res.Status != ImportInstalled {
		t.Errorf("status = %v", res.Status)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = s.Remove(ctx, validID)
	hasRoute(t, err, "Store.Remove->cancelCheck.ArchiveCanceled")
	if _, _, err := s.Get(context.Background(), validID); err != nil {
		t.Errorf("a cancelled remove removed it: %v", err)
	}
}

func TestStoreRecover(t *testing.T) {
	s, paths := newTestStore(t)
	importVersion(t, s, "1.0.0", nil)
	dir := paths.CassettesDir()
	mk := func(name string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, name), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name, "marker"), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// An interrupted import, an interrupted upgrade whose old folder is
	// missing its cassette (restored), and one whose cassette is back (removed).
	mk(validID + tmpMarker + "aaaa")
	mk("org.example.second" + oldMarker + "bbbb")
	mk(validID + oldMarker + "cccc")
	mk("Bad Id" + oldMarker + "dddd")
	if err := os.WriteFile(filepath.Join(dir, "a-file"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := s.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if left := leftovers(t, paths); !slices.Equal(left, []string{"Bad Id" + oldMarker + "dddd"}) {
		t.Errorf("leftovers = %v", left)
	}
	data, err := os.ReadFile(filepath.Join(dir, "org.example.second", "marker"))
	if err != nil || string(data) != "org.example.second"+oldMarker+"bbbb" {
		t.Errorf("not restored: %q %v", data, err)
	}
	if v := installedVersion(t, s); v != "1.0.0" {
		t.Errorf("version = %s", v)
	}

	t.Run("a store with no folder", func(t *testing.T) {
		s2, _ := newTestStore(t)
		if err := s2.Recover(context.Background()); err != nil {
			t.Error(err)
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		hasRoute(t, s.Recover(ctx), "Store.Recover->cancelCheck.ArchiveCanceled")
	})
}

func TestStoreFileOperationFailures(t *testing.T) {
	type setup struct {
		name  string
		ops   func(real storeOps, calls *int) storeOps
		first string // the first installed version, "" for none
		route string
		// state after: the installed version and whether leftovers remain
		wantVersion  string
		wantLeftover bool
	}
	failOnNth := func(n int) func(real storeOps, calls *int) storeOps {
		return func(real storeOps, calls *int) storeOps {
			real.rename = func(oldPath, newPath string) error {
				*calls++
				if *calls == n {
					return os.ErrPermission
				}
				return os.Rename(oldPath, newPath)
			}
			return real
		}
	}
	tests := []setup{
		{"install: the rename fails", failOnNth(1), "", "Store.Import->Store.swap.StoreInstallFailed", "", false},
		{"upgrade: setting the old folder aside fails", failOnNth(1), "1.0.0", "Store.Import->Store.swap.StoreAsideFailed", "1.0.0", false},
		{"upgrade: moving the new folder in fails and the old one comes back", failOnNth(2), "1.0.0", "Store.Import->Store.swap.StoreReplaceFailed", "1.0.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, paths := newTestStore(t)
			if tt.first != "" {
				importVersion(t, s, tt.first, nil)
			}
			calls := 0
			s.ops = tt.ops(s.ops, &calls)
			_, err := s.Import(context.Background(), packFS(t, withVersion(t, "2.0.0")), always)
			hasRoute(t, err, tt.route)
			if tt.wantVersion != "" {
				if v := installedVersion(t, s); v != tt.wantVersion {
					t.Errorf("installed version = %s", v)
				}
			} else if list, _ := s.List(context.Background()); len(list) != 0 {
				t.Errorf("installed: %v", list)
			}
			if left := leftovers(t, paths); len(left) != 0 {
				t.Errorf("leftovers: %v", left)
			}
		})
	}

	t.Run("upgrade: the old folder cannot be put back either", func(t *testing.T) {
		s, paths := newTestStore(t)
		importVersion(t, s, "1.0.0", nil)
		calls := 0
		s.ops.rename = func(oldPath, newPath string) error {
			calls++
			if calls >= 2 {
				return os.ErrPermission
			}
			return os.Rename(oldPath, newPath)
		}
		_, err := s.Import(context.Background(), packFS(t, withVersion(t, "2.0.0")), always)
		hasRoute(t, err, "Store.swap.StoreReplaceFailed")
		appErr, _ := errors.FromError(err)
		if appErr.Metadata["restore_error"] == nil || appErr.Metadata["old_folder"] == nil {
			t.Errorf("metadata = %v", appErr.Metadata)
		}
		// Recover puts it back.
		s.ops.rename = os.Rename
		if err := s.Recover(context.Background()); err != nil {
			t.Fatal(err)
		}
		if v := installedVersion(t, s); v != "1.0.0" {
			t.Errorf("after Recover: %s", v)
		}
		if left := leftovers(t, paths); len(left) != 0 {
			t.Errorf("leftovers: %v", left)
		}
	})
	t.Run("upgrade: the old folder cannot be deleted", func(t *testing.T) {
		s, paths := newTestStore(t)
		importVersion(t, s, "1.0.0", nil)
		real := s.ops.removeAll
		s.ops.removeAll = func(path string) error {
			if strings.Contains(path, oldMarker) {
				return os.ErrPermission
			}
			return real(path)
		}
		res, err := s.Import(context.Background(), packFS(t, withVersion(t, "2.0.0")), always)
		if err != nil || res.Status != ImportUpgraded {
			t.Fatalf("%+v %v", res, err)
		}
		if len(res.Problems) != 1 || !res.Problems[0].Warning || !strings.Contains(res.Problems[0].Message, "could not be deleted") {
			t.Errorf("problems = %v", res.Problems)
		}
		if v := installedVersion(t, s); v != "2.0.0" {
			t.Errorf("version = %s", v)
		}
		s.ops.removeAll = real
		if err := s.Recover(context.Background()); err != nil {
			t.Fatal(err)
		}
		if left := leftovers(t, paths); len(left) != 0 {
			t.Errorf("leftovers: %v", left)
		}
	})
	t.Run("remove: the rename fails", func(t *testing.T) {
		s, _ := newTestStore(t)
		importVersion(t, s, "1.0.0", nil)
		s.ops.rename = func(string, string) error { return os.ErrPermission }
		hasRoute(t, s.Remove(context.Background(), validID), "Store.Remove.StoreRemoveFailed")
		s.ops.rename = os.Rename
		if _, _, err := s.Get(context.Background(), validID); err != nil {
			t.Errorf("still installed? %v", err)
		}
	})
	t.Run("remove: the delete fails", func(t *testing.T) {
		s, paths := newTestStore(t)
		importVersion(t, s, "1.0.0", nil)
		real := s.ops.removeAll
		s.ops.removeAll = func(string) error { return os.ErrPermission }
		hasRoute(t, s.Remove(context.Background(), validID), "Store.Remove.StoreRemoveLeftoverFailed")
		if list, _ := s.List(context.Background()); len(list) != 0 {
			t.Errorf("it is still listed: %v", list)
		}
		s.ops.removeAll = real
		if err := s.Recover(context.Background()); err != nil {
			t.Fatal(err)
		}
		if left := leftovers(t, paths); len(left) != 0 {
			t.Errorf("leftovers: %v", left)
		}
	})
	t.Run("recover: a delete fails", func(t *testing.T) {
		s, paths := newTestStore(t)
		if err := os.MkdirAll(filepath.Join(paths.CassettesDir(), validID+tmpMarker+"x"), 0o700); err != nil {
			t.Fatal(err)
		}
		s.ops.removeAll = func(string) error { return os.ErrPermission }
		hasRoute(t, s.Recover(context.Background()), "Store.Recover.StoreRecoverFailed")
	})
}

func TestStoreReadFailures(t *testing.T) {
	t.Run("the cassettes folder is a file", func(t *testing.T) {
		paths := storagetest.Isolate(t)
		if err := os.MkdirAll(paths.Data, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(paths.CassettesDir(), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		s := NewStore(paths, nil)
		_, err := s.List(context.Background())
		hasRoute(t, err, "Store.List.StoreListFailed")
		_, _, err = s.Get(context.Background(), validID)
		hasRoute(t, err, "Store.Get->Store.stateOf.StoreStatFailed")
		hasRoute(t, s.Recover(context.Background()), "Store.Recover.StoreRecoverListFailed")
	})
	t.Run("an installed file is not a folder", func(t *testing.T) {
		s, paths := newTestStore(t)
		if err := os.MkdirAll(paths.CassettesDir(), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(paths.CassettesDir(), validID), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, _, err := s.Get(context.Background(), validID)
		hasRoute(t, err, "Store.Get.StoreNotInstalled")
		// An import cannot replace a file with a folder.
		_, err = s.Import(context.Background(), "testdata/valid", nil)
		hasRoute(t, err, "Store.swap.StoreInstallFailed")
		if left := leftovers(t, paths); len(left) != 0 {
			t.Errorf("leftovers: %v", left)
		}
	})
	t.Run("List and Get stop when the context is done", func(t *testing.T) {
		s, _ := newTestStore(t)
		importVersion(t, s, "1.0.0", nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := s.List(ctx)
		hasRoute(t, err, "Store.List->Store.manifestOf->LoadManifest->loader.document->loader.readFile.CassetteLoadCanceled")
		_, _, err = s.Get(ctx, validID)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Get: %v", err)
		}
	})
}

func TestExtractErrors(t *testing.T) {
	t.Run("the target exists", func(t *testing.T) {
		err := extract(context.Background(), validFS(t), t.TempDir())
		hasRoute(t, err, "extract.StoreExtractMkdirFailed")
	})
	t.Run("a folder cannot be made", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "x")
		tooLong := strings.Repeat("a", 300)
		err := extract(context.Background(), fstest.MapFS{"lessons/" + tooLong + "/lesson.json": {Data: []byte("{}")}}, dst)
		hasRoute(t, err, "extract.func1.StoreExtractFolderFailed")
	})
	t.Run("a file that cannot be read", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "x")
		err := extract(context.Background(), failingFS{valid: validFS(t), failOpen: "cover.svg"}, dst)
		hasRoute(t, err, "extract.func1.StoreExtractFileFailed")
	})
	t.Run("a file that exists already", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "x")
		err := extract(context.Background(), repeatFS{}, dst)
		hasRoute(t, err, "StoreExtractFileFailed")
	})
	t.Run("a listing that fails", func(t *testing.T) {
		err := extract(context.Background(), failingFS{valid: validFS(t), failDir: true}, filepath.Join(t.TempDir(), "x"))
		hasRoute(t, err, "extract.func1.StoreExtractReadFailed")
	})
	t.Run("a source that lies about its size", func(t *testing.T) {
		tree := newTree()
		tree.addFile("cassette.json", &treeEntry{size: 2, open: func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("{}{}{}")), nil
		}})
		err := extract(context.Background(), tree, filepath.Join(t.TempDir(), "x"))
		hasRoute(t, err, "ArchiveEntryTooLarge")
	})
}

func TestCheckExtracted(t *testing.T) {
	s, paths := newTestStore(t)
	importVersion(t, s, "1.0.0", nil)
	dir := filepath.Join(paths.CassettesDir(), validID)
	_, m, err := s.Get(context.Background(), validID)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkExtracted(context.Background(), dir, m); err != nil {
		t.Fatal(err)
	}
	other := m
	other.Version = ver(9, 9, 9)
	hasRoute(t, checkExtracted(context.Background(), dir, other), "checkExtracted.StoreSourceChanged")
	other = m
	other.ID = "org.other.id"
	hasRoute(t, checkExtracted(context.Background(), dir, other), "checkExtracted.StoreSourceChanged")
	hasRoute(t, checkExtracted(context.Background(), t.TempDir(), m), "checkExtracted.StoreSourceChanged")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := checkExtracted(ctx, dir, m); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: %v", err)
	}
}

func TestImportStatusString(t *testing.T) {
	seen := map[string]bool{}
	for s := ImportInvalid; s <= ImportDeclined; s++ {
		name := s.String()
		if name == "" || name == "unknown" || seen[name] {
			t.Errorf("status %d is called %q", s, name)
		}
		seen[name] = true
	}
	if ImportStatus(99).String() != "unknown" {
		t.Error("unknown status")
	}
}

func TestStoreConcurrency(t *testing.T) {
	s, _ := newTestStore(t)
	path1 := packFS(t, withVersion(t, "1.0.0"))
	path2 := packFS(t, withVersion(t, "1.1.0"))
	ctx := context.Background()
	importVersion(t, s, "1.0.0", nil)

	var wg sync.WaitGroup
	results := make([]ImportResult, 6)
	errs := make([]error, 6)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := path1
			if i%2 == 1 {
				p = path2
			}
			results[i], errs[i] = s.Import(ctx, p, always)
		}()
	}
	// Readers run meanwhile.
	stop := make(chan struct{})
	var readers sync.WaitGroup
	for i := 0; i < 3; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				list, err := s.List(ctx)
				if err != nil {
					t.Error(err)
					return
				}
				for _, in := range list {
					if in.Broken {
						t.Errorf("a half-installed cassette was listed: %+v", in)
					}
				}
			}
		}()
	}
	wg.Wait()
	close(stop)
	readers.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("import %d: %v", i, err)
		}
		// Every import ends with a final answer: the 1.1.0 ones upgrade or find
		// 1.1.0 installed, the 1.0.0 ones find 1.0.0 or a newer one.
		switch st := results[i].Status; {
		case i%2 == 1 && st != ImportUpgraded && st != ImportAlreadyInstalled:
			t.Errorf("import %d of 1.1.0: %v", i, st)
		case i%2 == 0 && st != ImportAlreadyInstalled && st != ImportNewerInstalled:
			t.Errorf("import %d of 1.0.0: %v", i, st)
		}
	}
	if v := installedVersion(t, s); v != "1.1.0" {
		t.Errorf("installed %s", v)
	}
}

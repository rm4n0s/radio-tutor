package admin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/student"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

func openCassettes(t *testing.T, a *testutil.App) {
	t.Helper()
	a.TapKey(KeyCassettes)
	waitKey(t, a, student.KeyImport)
}

// chooser answers the file dialog with the path.
func chooser(path string) app.FileChooser {
	return func(context.Context, m.BuildContext, app.FileQuestion) (string, bool, error) {
		return path, true, nil
	}
}

// breakCassette makes the installed cassette's manifest unreadable.
func breakCassette(t testing.TB, env *app.Env, id string) {
	t.Helper()
	file := filepath.Join(env.Paths.CassettesDir(), id, "cassette.json")
	if err := os.WriteFile(file, []byte("{ not json"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// bothHaveData gives mia and mum progress and photos for the cassette.
func bothHaveData(t *testing.T, env *app.Env, id string) {
	t.Helper()
	for _, user := range []string{"mia", "mum"} {
		setProgress(t, env, user, id, func(p *progress.Progress) {
			p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StatePassed, Attempts: 1}
		})
		photos(t, env, user, id)
	}
}

func progressFile(env *app.Env, user, id string) string {
	return filepath.Join(env.Paths.ProgressDir(), user, id+".json")
}

func TestCassettesList(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		twoCassettes(t, env)
		install(t, env, fixture(t, "org.example.bad-3", "1.0.0", "Bad one"))
		breakCassette(t, env, "org.example.bad-3")
	}})
	openCassettes(t, a)
	waitText(t, a, fractionsTitle)
	for _, text := range []string{"Second steps", "version 1.0.0", "version 2.1.0", fractionsID, otherID,
		"org.example.bad-3", "This cassette is damaged. It can only be removed."} {
		if !has(a, text) {
			t.Errorf("the list does not show %q", text)
		}
	}
	for _, id := range []string{fractionsID, otherID, "org.example.bad-3"} {
		if !a.HasKey(KeyRemovePrefix + id) {
			t.Errorf("no Remove button for %s", id)
		}
	}
	if has(a, "Bad one") {
		t.Error("the broken cassette shows a title")
	}
}

func TestCassettesNothingInstalled(t *testing.T) {
	a := pump(t, config{})
	openCassettes(t, a)
	waitText(t, a, "No cassettes are installed. Import one to start.")
}

func TestCassettesListFailure(t *testing.T) {
	storagetest.SkipOnWindows(t, "a path through a file reports not found, not not-a-folder")
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		if err := os.RemoveAll(env.Paths.CassettesDir()); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(env.Paths.CassettesDir(), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}})
	openCassettes(t, a)
	waitText(t, a, "The cassettes could not be listed.")
}

func TestCassettesImport(t *testing.T) {
	path := fixture(t, fractionsID, "1.0.0", fractionsTitle)
	a := pump(t, config{choose: chooser(path)})
	openCassettes(t, a)
	waitText(t, a, "No cassettes are installed. Import one to start.")
	a.TapKey(student.KeyImport)
	waitText(t, a, fractionsTitle)
	waitText(t, a, "The cassette \""+fractionsTitle+"\" was added.")
	if !a.HasKey(KeyRemovePrefix + fractionsID) {
		t.Error("the imported cassette has no row")
	}
}

func TestCassettesImportUpgrade(t *testing.T) {
	path := fixture(t, fractionsID, "1.1.0", fractionsTitle)
	a := pump(t, config{choose: chooser(path), setup: func(t *testing.T, env *app.Env) {
		install(t, env, fixture(t, fractionsID, "1.0.0", fractionsTitle))
		bothHaveData(t, env, fractionsID)
	}})
	openCassettes(t, a)
	waitText(t, a, "version 1.0.0")
	tapBusy(a, student.KeyImport)
	whileBusy(t, a, "the upgrade dialog", func() bool { return a.HasKey(student.KeyDialogYes) })
	if !has(a, "Version 1.0.0 is installed. Replace it with version 1.1.0? The progress is kept.") {
		t.Error("the dialog does not name both versions")
	}
	tapBusy(a, student.KeyDialogYes)
	whileBusy(t, a, "the import to end", func() bool { return has(a, "version 1.1.0") })
	waitText(t, a, "version 1.1.0")
	if !exists(progressFile(a.Env, "mia", fractionsID)) {
		t.Error("the progress was not kept")
	}
}

func TestCassettesImportInvalid(t *testing.T) {
	bad := t.TempDir()
	if err := os.WriteFile(filepath.Join(bad, "cassette.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := pump(t, config{choose: chooser(bad)})
	openCassettes(t, a)
	a.TapKey(student.KeyImport)
	waitText(t, a, "This file is not a good cassette")
	a.TapKey(student.KeyDialogOK)
	waitGone(t, a, student.KeyDialogOK)
	waitText(t, a, "No cassettes are installed. Import one to start.")
}

func TestCassettesRemove(t *testing.T) {
	tests := []struct {
		name       string
		deleteData bool
	}{
		{"keeps the progress and photos", false},
		{"deletes the progress and photos", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
				twoCassettes(t, env)
				bothHaveData(t, env, fractionsID)
				bothHaveData(t, env, otherID)
			}})
			openCassettes(t, a)
			waitText(t, a, fractionsTitle)
			a.TapKey(KeyRemovePrefix + fractionsID)
			waitKey(t, a, KeyDeleteData)
			if !has(a, "Remove \""+fractionsTitle+"\"?") || !has(a, "Delete the progress and photos of every student for this cassette") {
				t.Error("the dialog lacks its texts")
			}
			if tt.deleteData {
				a.TapKey(KeyDeleteData)
			}
			a.TapKey(KeyDialogOK)
			waitText(t, a, "The cassette \""+fractionsTitle+"\" was removed.")
			waitGone(t, a, KeyRemovePrefix+fractionsID)
			if exists(filepath.Join(a.Env.Paths.CassettesDir(), fractionsID)) {
				t.Error("the cassette folder is still there")
			}
			for _, user := range []string{"mia", "mum"} {
				if got := exists(progressFile(a.Env, user, fractionsID)); got == tt.deleteData {
					t.Errorf("progress of %s exists = %v", user, got)
				}
				if got := exists(filepath.Join(a.Env.Paths.PhotosDir(), user, fractionsID)); got == tt.deleteData {
					t.Errorf("photos of %s exist = %v", user, got)
				}
				// The other cassette is never touched.
				if !exists(progressFile(a.Env, user, otherID)) || !exists(filepath.Join(a.Env.Paths.PhotosDir(), user, otherID)) {
					t.Errorf("the data of the other cassette of %s was deleted", user)
				}
			}
		})
	}
}

func TestCassettesRemoveCheckboxStartsUnchecked(t *testing.T) {
	a := pump(t, config{setup: twoCassettes})
	openCassettes(t, a)
	waitText(t, a, fractionsTitle)
	a.TapKey(KeyRemovePrefix + fractionsID)
	waitKey(t, a, KeyDeleteData)
	if checked(a) {
		t.Error("the delete-data checkbox starts checked")
	}
	a.TapKey(KeyDeleteData)
	if !checked(a) {
		t.Error("tapping the checkbox did not check it")
	}
}

func TestCassettesRemoveCancel(t *testing.T) {
	a := pump(t, config{setup: twoCassettes})
	openCassettes(t, a)
	waitText(t, a, fractionsTitle)
	a.TapKey(KeyRemovePrefix + fractionsID)
	waitKey(t, a, KeyDialogCancel)
	a.TapKey(KeyDialogCancel)
	waitGone(t, a, KeyDialogOK)
	if !exists(filepath.Join(a.Env.Paths.CassettesDir(), fractionsID)) {
		t.Error("Cancel removed the cassette")
	}
}

func TestCassettesRemoveBroken(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		twoCassettes(t, env)
		breakCassette(t, env, otherID)
		bothHaveData(t, env, otherID)
	}})
	openCassettes(t, a)
	waitText(t, a, "This cassette is damaged. It can only be removed.")
	a.TapKey(KeyRemovePrefix + otherID)
	waitText(t, a, "Remove \""+otherID+"\"?")
	a.TapKey(KeyDeleteData)
	a.TapKey(KeyDialogOK)
	waitGone(t, a, KeyRemovePrefix+otherID)
	if exists(progressFile(a.Env, "mia", otherID)) {
		t.Error("the progress of the broken cassette was not deleted")
	}
}

func TestCassettesRemoveFailureIsShown(t *testing.T) {
	a := pump(t, config{setup: twoCassettes})
	openCassettes(t, a)
	waitText(t, a, fractionsTitle)
	// The folder disappears behind the screen's back.
	if err := os.RemoveAll(filepath.Join(a.Env.Paths.CassettesDir(), fractionsID)); err != nil {
		t.Fatal(err)
	}
	a.TapKey(KeyRemovePrefix + fractionsID)
	waitKey(t, a, KeyDialogOK)
	a.TapKey(KeyDialogOK)
	waitText(t, a, "Something went wrong: the cassette is not installed (StoreNotInstalled)")
}

func TestRemoveCassetteDataFailures(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		setup func(t *testing.T, env *app.Env)
		route string
	}{
		{"not a cassette id", "../x", nil, "removeCassetteData.AdminPhotosCassetteInvalid"},
		{"photos is not a folder", fractionsID, func(t *testing.T, env *app.Env) {
			if err := os.MkdirAll(env.Paths.Data, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(env.Paths.PhotosDir(), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, "removeCassetteData.AdminPhotosListFailed"},
		{"a photo folder cannot be removed", fractionsID, func(t *testing.T, env *app.Env) {
			dir := photos(t, env, "mia", fractionsID)
			if err := os.Chmod(filepath.Dir(dir), 0o500); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(filepath.Dir(dir), 0o700) })
		}, "removeCassetteData.AdminPhotosDeleteFailed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				storagetest.SkipOnWindows(t, "the setup makes a file where a folder goes or a folder read-only")
			}
			env, _ := testutil.NewEnv(t, testutil.Options{})
			if tt.setup != nil {
				tt.setup(t, env)
			}
			wantRoute(t, removeCassetteData(context.Background(), env, tt.id), tt.route)
		})
	}
}

func TestRemoveCassetteDataWithNoPhotos(t *testing.T) {
	env, _ := testutil.NewEnv(t, testutil.Options{})
	if err := removeCassetteData(context.Background(), env, fractionsID); err != nil {
		t.Errorf("a data folder without photos is an error: %v", err)
	}
}

// checked reports whether the one checkbox on screen is checked.
func checked(a *testutil.App) bool {
	found := gluttertest.ByType[*m.Checkbox]().Find(a.Tester)
	if len(found) != 1 {
		panic("expected one checkbox")
	}
	return found[0].Widget().(*m.Checkbox).Value().OrElse(false)
}

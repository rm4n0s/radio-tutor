package admin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

// dataEnv is an environment without a window.
func dataEnv(t *testing.T) *app.Env {
	t.Helper()
	env, _ := testutil.NewEnv(t, testutil.Options{})
	return env
}

// lockedFolder makes a photos folder that cannot be removed: a folder inside
// it has no write permission. It skips the test when the user can write
// anywhere (root).
func lockedFolder(t *testing.T, dir string) {
	t.Helper()
	storagetest.SkipIfModesDoNotBlock(t)
	inner := filepath.Join(dir, "inner")
	if err := os.MkdirAll(inner, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inner, "1.jpg"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(inner, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(inner, 0o700); err != nil {
			t.Error(err)
		}
	})
}

func TestPhotoWorkRefusesNamesThatAreNotFolders(t *testing.T) {
	env := dataEnv(t)
	ctx := context.Background()
	tests := []struct {
		name  string
		do    func() error
		route string
	}{
		{"submission with a bad username", func() error { return deleteSubmissionPhotos(ctx, env, "../mia", fractionsID, subOlder) }, "deleteSubmissionPhotos->photoFolder.AdminPhotosNameInvalid"},
		{"submission with a bad cassette", func() error { return deleteSubmissionPhotos(ctx, env, "mia", "..", subOlder) }, "deleteSubmissionPhotos->photoFolder.AdminPhotosNameInvalid"},
		{"submission with a bad id", func() error { return deleteSubmissionPhotos(ctx, env, "mia", fractionsID, "../x") }, "deleteSubmissionPhotos->photoFolder.AdminPhotosNameInvalid"},
		{"cassette photos", func() error { return removeCassettePhotos(env, "mia", "a/b") }, "removeCassettePhotos->photoFolder.AdminPhotosNameInvalid"},
		{"reset all with photos", func() error { return resetAll(ctx, env, "../mia", fractionsID, "mum", at(1, 0, 0), true) }, "resetAll->removeCassettePhotos->photoFolder.AdminPhotosNameInvalid"},
		{"student photos", func() error { return deleteStudentPhotos(ctx, env, "../mia") }, "deleteStudentPhotos->removePhotosOf.AdminPhotosUsernameInvalid"},
		{"list of saves", func() error { _, err := listSaveIDs(env, "../mia"); return err }, "listSaveIDs.AdminSavesUsernameInvalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { wantRoute(t, tt.do(), tt.route) })
	}
}

func TestPhotoWorkReportsFoldersThatCannotBeRemoved(t *testing.T) {
	env := dataEnv(t)
	ctx := context.Background()
	submission := filepath.Join(env.Paths.PhotosDir(), "mia", fractionsID, subOlder)
	lockedFolder(t, submission)
	wantRoute(t, deleteSubmissionPhotos(ctx, env, "mia", fractionsID, subOlder), "deleteSubmissionPhotos.AdminSubmissionPhotosDeleteFailed")
	wantRoute(t, removeCassettePhotos(env, "mia", fractionsID), "removeCassettePhotos.AdminCassettePhotosDeleteFailed")
	wantRoute(t, resetAll(ctx, env, "mia", fractionsID, "mum", at(1, 0, 0), true), "resetAll->removeCassettePhotos.AdminCassettePhotosDeleteFailed")
}

func TestDeleteSubmissionPhotosOfAnUnknownSubmission(t *testing.T) {
	env := dataEnv(t)
	setProgress(t, env, "mia", fractionsID, richProgress)
	err := deleteSubmissionPhotos(context.Background(), env, "mia", fractionsID, "20200101T000000-0000")
	wantRoute(t, err, "AdminSubmissionUnknown")
	// The failed update saved nothing.
	if p := loadSaved(t, env, "mia", fractionsID); len(p.Submissions) != 2 || len(p.Submission(subOlder).Photos) != 2 {
		t.Errorf("the save file changed: %+v", p.Submissions)
	}
}

func TestListSaveIDs(t *testing.T) {
	env := dataEnv(t)
	if ids, err := listSaveIDs(env, "mia"); err != nil || ids != nil {
		t.Fatalf("a student without a folder gave %v, %v", ids, err)
	}
	for _, id := range []string{otherID, fractionsID} {
		setProgress(t, env, "mia", id, func(*progress.Progress) {})
	}
	dir := filepath.Join(env.Paths.ProgressDir(), "mia")
	for _, name := range []string{"notes.txt", "bad id.json", fractionsID + ".json.broken-20261030T000000"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ids, err := listSaveIDs(env, "mia")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != fractionsID || ids[1] != otherID {
		t.Errorf("the save files are %v, want %s and %s", ids, fractionsID, otherID)
	}
}

func TestListSaveIDsFailsWhenTheStudentFolderIsAFile(t *testing.T) {
	storagetest.SkipOnWindows(t, "a path through a file reports not found, not not-a-folder")
	env := dataEnv(t)
	if err := os.MkdirAll(env.Paths.ProgressDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.Paths.ProgressDir(), "mia"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := listSaveIDs(env, "mia")
	wantRoute(t, err, "listSaveIDs.AdminSavesListFailed")
}

func TestOpenCassetteRefusesADamagedCassette(t *testing.T) {
	env := dataEnv(t)
	install(t, env, fixture(t, fractionsID, "1.0.0", fractionsTitle))
	lesson := filepath.Join(env.Paths.Data, "cassettes", fractionsID, "lessons", lessonWhat, "lesson.json")
	if err := os.WriteFile(lesson, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := openCassette(context.Background(), env, fractionsID)
	wantRoute(t, err, "openCassette.AdminCassetteInvalid")
	if _, err := openCassette(context.Background(), env, otherID); !hasTag(err, "StoreNotInstalled") {
		t.Errorf("a cassette that is not installed gave %v", err)
	}
}

func TestScanWaitingCountsWhatTheStatesSay(t *testing.T) {
	env := dataEnv(t)
	testutil.AddProfile(t, env, "mia", "Mia", profile.RoleStudent, studentPassword)
	testutil.AddProfile(t, env, "mum", "Mum", profile.RoleAdmin, adminPassword)
	twoCassettes(t, env)
	setProgress(t, env, "mia", fractionsID, func(p *progress.Progress) {
		// Stored needs_review, but the exercise has no attempt limit: the unlock
		// rule says it is not waiting.
		p.Exercises[exName] = progress.ExerciseProgress{State: progress.StateNeedsReview, Attempts: 9}
		p.Exercises[exShade] = progress.ExerciseProgress{Attempts: 3} // used up: waiting
		p.Exercises[exSum] = progress.ExerciseProgress{State: progress.StatePassed, Attempts: 5}
	})
	got, err := scanWaiting(context.Background(), env, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.total() != 1 || got.ofStudent("mia") != 1 || got["mia"][fractionsID] != 1 {
		t.Errorf("the counts are %v, want one for mia", got)
	}
	only, err := scanWaiting(context.Background(), env, []string{"mum"})
	if err != nil || only.total() != 0 {
		t.Errorf("counting only mum gave %v, %v", only, err)
	}
}

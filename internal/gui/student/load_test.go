package student

import (
	"context"
	"testing"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

func TestCountDone(t *testing.T) {
	items := []cassette.Item{
		{Type: cassette.ItemLesson, ID: "l1"},
		{Type: cassette.ItemExercise, ID: "e1"},
		{Type: cassette.ItemExercise, ID: "e2"},
		{Type: cassette.ItemLesson, ID: "l2"},
	}
	tests := []struct {
		name     string
		progress func(p *progress.Progress)
		done     int
	}{
		{"nothing", func(*progress.Progress) {}, 0},
		{"a completed lesson and a passed exercise", func(p *progress.Progress) {
			p.Lessons["l1"] = progress.LessonProgress{CompletedAt: completedAt}
			p.Exercises["e1"] = progress.ExerciseProgress{State: progress.StatePassed}
		}, 2},
		{"needs review is not done", func(p *progress.Progress) {
			p.Exercises["e1"] = progress.ExerciseProgress{State: progress.StateNeedsReview}
		}, 0},
		{"a lesson that was played but not completed", func(p *progress.Progress) {
			p.Lessons["l1"] = progress.LessonProgress{Plays: 3}
		}, 0},
		{"entries of ids the cassette does not have or has with another type", func(p *progress.Progress) {
			p.Lessons["gone"] = progress.LessonProgress{CompletedAt: completedAt}
			p.Lessons["e1"] = progress.LessonProgress{CompletedAt: completedAt}
			p.Exercises["l2"] = progress.ExerciseProgress{State: progress.StatePassed}
		}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := progress.New("x")
			tt.progress(p)
			done, total := countDone(items, p)
			if done != tt.done || total != 4 {
				t.Errorf("countDone = %d of %d, want %d of 4", done, total, tt.done)
			}
		})
	}
}

func TestWorksheetState(t *testing.T) {
	w := cassette.Worksheet{ExerciseIDs: []string{"a", "b"}}
	tests := []struct {
		name   string
		states map[string]progress.ItemState
		want   progress.ItemState
	}{
		{"all locked", map[string]progress.ItemState{"a": progress.StateLocked, "b": progress.StateLocked}, progress.StateLocked},
		{"all passed", map[string]progress.ItemState{"a": progress.StatePassed, "b": progress.StatePassed}, progress.StatePassed},
		{"one needs review", map[string]progress.ItemState{"a": progress.StatePassed, "b": progress.StateNeedsReview}, progress.StateNeedsReview},
		{"one passed one available", map[string]progress.ItemState{"a": progress.StatePassed, "b": progress.StateAvailable}, progress.StateAvailable},
		{"all available", map[string]progress.ItemState{"a": progress.StateAvailable, "b": progress.StateAvailable}, progress.StateAvailable},
		{"locked and needs review", map[string]progress.ItemState{"a": progress.StateLocked, "b": progress.StateNeedsReview}, progress.StateNeedsReview},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := worksheetState(w, tt.states); got != tt.want {
				t.Errorf("worksheetState = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestLoadCassetteRejectsAnInvalidCassette(t *testing.T) {
	env, _ := testutil.NewEnv(t, testutil.Options{})
	install(t, env, fixture(t, fractionsID, "1.0.0", fractionsTitle))
	removeLessons(t, env, fractionsID)
	_, err := loadCassette(context.Background(), env, "mia", fractionsID)
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T: %v", err, err)
	}
	if !appErr.HasRoute("loadCassette.StudentCassetteInvalid") {
		t.Errorf("unexpected failure path: %s", appErr.Route())
	}
}

func TestLoadCassetteErrorsFromTheStores(t *testing.T) {
	env, _ := testutil.NewEnv(t, testutil.Options{})
	install(t, env, fixture(t, fractionsID, "1.0.0", fractionsTitle))
	tests := []struct {
		name     string
		username string
		id       string
		route    string
	}{
		{"not installed", "mia", "org.example.nothing-1", "Store.Get.StoreNotInstalled"},
		{"a username that cannot be a folder name", "../x", fractionsID, "ProgressUsernameInvalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadCassette(context.Background(), env, tt.username, tt.id)
			appErr, ok := errors.FromError(err)
			if !ok {
				t.Fatalf("expected *errors.Error, got %T: %v", err, err)
			}
			if !appErr.HasRoute(tt.route) {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
		})
	}
}

func TestLoadCassetteSetsAsideABrokenSaveFile(t *testing.T) {
	env, _ := testutil.NewEnv(t, testutil.Options{})
	install(t, env, fixture(t, fractionsID, "1.0.0", fractionsTitle))
	setProgress(t, env, fractionsID, func(p *progress.Progress) { p.LastItem = lesson1 })
	writeBrokenSave(t, env, fractionsID)
	v, err := loadCassette(context.Background(), env, "mia", fractionsID)
	if err != nil {
		t.Fatalf("loadCassette: %v", err)
	}
	if v.Done != 0 || v.Total != 5 || v.Continue.ID != lesson1 {
		t.Errorf("view = done %d of %d, continue %q", v.Done, v.Total, v.Continue.ID)
	}
	if len(v.Cover) == 0 {
		t.Error("the cover was not read")
	}
	// The next load works on the fresh file.
	if _, err := loadCassette(context.Background(), env, "mia", fractionsID); err != nil {
		t.Errorf("second load: %v", err)
	}
}

func TestLoadShelfWithABrokenSaveFile(t *testing.T) {
	env, _ := testutil.NewEnv(t, testutil.Options{})
	install(t, env, fixture(t, fractionsID, "1.0.0", fractionsTitle))
	writeBrokenSave(t, env, fractionsID)
	cards, err := loadShelf(context.Background(), env, "mia")
	if err != nil || len(cards) != 1 || cards[0].Done != 0 {
		t.Fatalf("loadShelf = %+v, %v", cards, err)
	}
}

func TestLoadShelfSkipsACassetteWithoutACover(t *testing.T) {
	env, _ := testutil.NewEnv(t, testutil.Options{})
	install(t, env, fixture(t, fractionsID, "1.0.0", fractionsTitle))
	removeCover(t, env, fractionsID)
	cards, err := loadShelf(context.Background(), env, "mia")
	if err != nil || len(cards) != 1 || cards[0].Cover != nil || cards[0].Title != fractionsTitle {
		t.Fatalf("loadShelf = %+v, %v", cards, err)
	}
}

func TestLoadShelfFailsWhenCancelled(t *testing.T) {
	env, _ := testutil.NewEnv(t, testutil.Options{})
	install(t, env, fixture(t, fractionsID, "1.0.0", fractionsTitle))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := loadShelf(ctx, env, "mia"); err == nil {
		t.Error("a cancelled load succeeded")
	}
}

package progress

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
)

const cassetteID = "org.example.test-1"

func newStore(t *testing.T) (*Store, storage.Paths) {
	t.Helper()
	paths := storagetest.Isolate(t)
	return NewStore(paths, WithClock(func() time.Time { return t0 })), paths
}

func progressFile(paths storage.Paths, username string) string {
	return filepath.Join(paths.ProgressDir(), username, cassetteID+".json")
}

func writeRaw(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMissingIsFresh(t *testing.T) {
	s, paths := newStore(t)
	p, err := s.Load(context.Background(), "ann", cassetteID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p, New(cassetteID)) {
		t.Errorf("fresh progress = %+v", p)
	}
	if _, err := os.Stat(paths.ProgressDir()); err == nil {
		t.Error("Load created the progress folder")
	}
}

func TestSaveLoadRoundTripAndFile(t *testing.T) {
	s, paths := newStore(t)
	ctx := context.Background()
	c := testCassette()

	p := New(cassetteID)
	if _, err := Migrate(c, p); err != nil {
		t.Fatal(err)
	}
	p.CassetteVersion = cassette.Version{Major: 1, Minor: 2, Patch: 0}
	p.LastItem = "e1"
	p.Settings.SkipLessons = true
	complete(t, c, p, "l1")
	p.Lessons["l1"] = LessonProgress{CompletedAt: t0, Plays: 2}
	results := []Result{{ExerciseID: "e1", Verdict: VerdictCorrect, Transcription: "1/2", FeedbackForChild: "Well done", NotesForParent: "neat"}}
	if _, err := ApplyResults(c, p, results, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	p.Submissions = []Submission{{
		ID: "20261030T171955-3f9a", CreatedAt: t0, ExerciseIDs: []string{"e1"}, Photos: []string{"1.jpg"},
		Status: SubmissionGraded, Provider: "Ollama", Model: "qwen", PageNotes: "tidy", Results: results,
	}}
	if err := ApplyOverride(c, p, Override{ExerciseID: "e2", Action: ActionMarkPassed, By: "mom", At: t0}); err != nil {
		t.Fatal(err)
	}
	p.Orphaned = &Orphaned{Lessons: map[string]LessonProgress{"old": {Plays: 1}}}

	if err := s.Save(ctx, "ann", p); err != nil {
		t.Fatal(err)
	}
	path := progressFile(paths, "ann")
	if info, err := os.Stat(path); err != nil || (storagetest.UnixModes && info.Mode().Perm() != storage.FileMode) {
		t.Errorf("file: %v, %v", info, err)
	}
	if info, err := os.Stat(filepath.Dir(path)); err != nil || (storagetest.UnixModes && info.Mode().Perm() != storage.DirMode) {
		t.Errorf("folder: %v, %v", info, err)
	}

	// the field names of SPEC §7.4
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"format", "cassette_id", "cassette_version", "settings", "last_item", "lessons",
		"exercises", "submissions", "overrides", "orphaned"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("save file has no %q:\n%s", key, data)
		}
	}
	if doc["format"] != float64(1) || doc["cassette_version"] != "1.2.0" || doc["last_item"] != "e1" {
		t.Errorf("save file head: %v", doc)
	}
	settings := doc["settings"].(map[string]any)
	if _, ok := settings["skip_lessons"]; !ok {
		t.Errorf("settings = %v", settings)
	}
	if _, ok := settings["free_navigation"]; !ok {
		t.Errorf("settings = %v", settings)
	}
	lesson := doc["lessons"].(map[string]any)["l1"].(map[string]any)
	if lesson["completed_at"] != "2026-10-30T17:00:00Z" || lesson["plays"] != float64(2) {
		t.Errorf("lesson = %v", lesson)
	}
	ex := doc["exercises"].(map[string]any)["e1"].(map[string]any)
	wantEx := map[string]any{"state": "passed", "attempts": float64(1), "hints_shown": float64(0), "passed_by": "ai",
		"passed_at": "2026-10-30T17:01:00Z", "last_verdict": "correct"}
	if !reflect.DeepEqual(ex, wantEx) {
		t.Errorf("exercise = %v\nwant       %v", ex, wantEx)
	}
	sub := doc["submissions"].([]any)[0].(map[string]any)
	for _, key := range []string{"id", "created_at", "exercise_ids", "photos", "status", "provider", "model", "page_notes", "results", "error"} {
		if _, ok := sub[key]; !ok {
			t.Errorf("submission has no %q: %v", key, sub)
		}
	}
	result := sub["results"].([]any)[0].(map[string]any)
	for _, key := range []string{"exercise_id", "verdict", "transcription", "feedback_for_child", "notes_for_parent", "counted_attempt"} {
		if _, ok := result[key]; !ok {
			t.Errorf("result has no %q: %v", key, result)
		}
	}
	ov := doc["overrides"].([]any)[0].(map[string]any)
	for _, key := range []string{"exercise_id", "action", "by", "at", "note"} {
		if _, ok := ov[key]; !ok {
			t.Errorf("override has no %q: %v", key, ov)
		}
	}

	got, err := s.Load(ctx, "ann", cassetteID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, p) {
		t.Errorf("round trip\n got %+v\nwant %+v", got, p)
	}
	other, err := s.Load(ctx, "bob", cassetteID)
	if err != nil || len(other.Exercises) != 0 {
		t.Errorf("another student's progress: %+v, %v", other, err)
	}
}

func TestLoadAcceptsTheSpecExample(t *testing.T) {
	s, paths := newStore(t)
	writeRaw(t, progressFile(paths, "ann"), `{
  "format": 1,
  "cassette_id": "org.example.test-1",
  "cassette_version": "1.2.0",
  "settings": {"skip_lessons": false, "free_navigation": false},
  "last_item": "adding-fractions",
  "lessons": {"what-is-a-fraction": {"completed_at": "2026-10-30T17:02:11Z", "plays": 2}},
  "exercises": {"shade-the-half": {"state": "passed", "attempts": 2, "hints_shown": 1, "passed_by": "ai",
      "passed_at": "2026-10-30T17:20:40Z", "last_verdict": "correct"}},
  "submissions": [{"id": "20261030T171955-3f9a", "created_at": "2026-10-30T17:19:55Z",
      "exercise_ids": ["shade-the-half", "write-the-sum"], "photos": ["1.jpg", "2.jpg"], "status": "graded",
      "provider": "Ollama on this computer", "model": "qwen2.5vl:7b", "page_notes": "...",
      "results": [{"exercise_id": "shade-the-half", "verdict": "correct", "transcription": "...",
          "feedback_for_child": "...", "notes_for_parent": "...", "counted_attempt": true}], "error": ""}],
  "overrides": [{"exercise_id": "write-the-sum", "action": "mark_passed", "by": "mom", "at": "2026-10-30T18:00:00Z", "note": ""}]
}`)
	p, err := s.Load(context.Background(), "ann", cassetteID)
	if err != nil {
		t.Fatal(err)
	}
	if p.CassetteVersion != (cassette.Version{Major: 1, Minor: 2}) || p.LastItem != "adding-fractions" ||
		!p.Lessons["what-is-a-fraction"].Completed() || p.Exercises["shade-the-half"].PassedBy != PassedByAI ||
		len(p.Submissions) != 1 || !p.Submissions[0].Results[0].CountedAttempt || p.Overrides[0].Action != ActionMarkPassed {
		t.Errorf("loaded %+v", p)
	}
}

func TestLoadNullsBecomeEmpty(t *testing.T) {
	s, paths := newStore(t)
	writeRaw(t, progressFile(paths, "ann"), `{"format":1,"cassette_id":"org.example.test-1","cassette_version":"1.0.0",
	  "lessons":null,"exercises":null,"submissions":null,"overrides":null}`)
	p, err := s.Load(context.Background(), "ann", cassetteID)
	if err != nil {
		t.Fatal(err)
	}
	p.Lessons["x"] = LessonProgress{Plays: 1} // must not panic
	if p.Submissions == nil || p.Overrides == nil || p.Exercises == nil {
		t.Errorf("loaded %+v", p)
	}
}

func TestLoadBrokenIsQuarantined(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"not JSON", "{ nope"},
		{"empty file", ""},
		{"bad version", `{"format":1,"cassette_id":"org.example.test-1","cassette_version":"1.0"}`},
		{"no format", `{"cassette_id":"org.example.test-1","cassette_version":"1.0.0"}`},
		{"newer format", `{"format":2,"cassette_id":"org.example.test-1","cassette_version":"1.0.0"}`},
		{"other cassette", `{"format":1,"cassette_id":"org.example.other-1","cassette_version":"1.0.0"}`},
		{"unknown state", `{"format":1,"cassette_id":"org.example.test-1","cassette_version":"1.0.0","exercises":{"e1":{"state":"done"}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, paths := newStore(t)
			path := progressFile(paths, "ann")
			writeRaw(t, path, tt.content)

			p, err := s.Load(context.Background(), "ann", cassetteID)
			wantTag(t, err, "ProgressFileBroken", "Store.Load")
			appErr, _ := errors.FromError(err)
			moved := path + ".broken-20261030T170000Z"
			if appErr.Metadata["quarantined_to"] != moved || appErr.Metadata["path"] != path || appErr.Metadata["reason"] == "" {
				t.Errorf("metadata = %v", appErr.Metadata)
			}
			if p == nil || !reflect.DeepEqual(p, New(cassetteID)) {
				t.Errorf("progress = %+v, want a fresh one", p)
			}
			if data, err := os.ReadFile(moved); err != nil || string(data) != tt.content {
				t.Errorf("quarantined file: %q, %v", data, err)
			}
			if _, err := os.Stat(path); err == nil {
				t.Error("the broken file is still in place")
			}
			// the next Load is clean, and a Save does not overwrite the quarantined file
			if again, err := s.Load(context.Background(), "ann", cassetteID); err != nil || len(again.Lessons) != 0 {
				t.Errorf("second Load: %+v, %v", again, err)
			}
			if err := s.Save(context.Background(), "ann", p); err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(moved); err != nil || string(data) != tt.content {
				t.Errorf("quarantined file after Save: %q, %v", data, err)
			}
		})
	}
}

func TestLoadFailures(t *testing.T) {
	t.Run("unreadable file", func(t *testing.T) {
		s, paths := newStore(t)
		if err := os.MkdirAll(progressFile(paths, "ann"), 0o700); err != nil { // a folder where the file should be
			t.Fatal(err)
		}
		p, err := s.Load(context.Background(), "ann", cassetteID)
		wantTag(t, err, "JSONReadFailed", "Store.Load")
		if p != nil {
			t.Errorf("progress = %+v", p)
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		s, _ := newStore(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := s.Load(ctx, "ann", cassetteID)
		wantTag(t, err, "ProgressLoadCancelled", "Store.Load.ProgressLoadCancelled")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("not context.Canceled: %v", err)
		}
	})
	t.Run("quarantine fails", func(t *testing.T) {
		storagetest.SkipIfModesDoNotBlock(t)
		s, paths := newStore(t)
		path := progressFile(paths, "ann")
		writeRaw(t, path, "{ nope")
		if err := os.Chmod(filepath.Dir(path), 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(filepath.Dir(path), 0o700) })
		p, err := s.Load(context.Background(), "ann", cassetteID)
		wantTag(t, err, "QuarantineFailed", "Store.loadFile")
		if p != nil {
			t.Errorf("progress = %+v", p)
		}
	})
}

func TestNamesAreGuarded(t *testing.T) {
	s, paths := newStore(t)
	ctx := context.Background()
	bad := []string{"", ".", "..", "../x", "a/b", `a\b`, "x\x00y", ".hidden", "ü", "a b", strings.Repeat("a", 129)}
	for _, name := range bad {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			_, err := s.Load(ctx, name, cassetteID)
			wantTag(t, err, "ProgressUsernameInvalid", "Store.Load->Store.file.ProgressUsernameInvalid")
			_, err = s.Load(ctx, "ann", name)
			wantTag(t, err, "ProgressCassetteIDInvalid", "Store.Load->Store.file.ProgressCassetteIDInvalid")
			wantTag(t, s.Save(ctx, name, New(cassetteID)), "ProgressUsernameInvalid", "Store.Save->Store.file.ProgressUsernameInvalid")
			wantTag(t, s.Save(ctx, "ann", New(name)), "ProgressCassetteIDInvalid", "Store.Save->Store.file.ProgressCassetteIDInvalid")
			wantTag(t, s.Update(ctx, name, cassetteID, func(*Progress) error { return nil }), "ProgressUsernameInvalid", "Store.Update->Store.file")
			wantTag(t, s.DeleteAll(ctx, name, cassetteID), "ProgressUsernameInvalid", "Store.DeleteAll->Store.file")
			wantTag(t, s.DeleteAll(ctx, "ann", name), "ProgressCassetteIDInvalid", "Store.DeleteAll->Store.file")
			wantTag(t, s.DeleteUser(ctx, name), "ProgressUserDirUsernameInvalid", "Store.DeleteUser->Store.userDir.ProgressUserDirUsernameInvalid")
			wantTag(t, s.DeleteCassette(ctx, name), "ProgressCassetteDeleteIDInvalid", "Store.DeleteCassette.ProgressCassetteDeleteIDInvalid")
		})
	}
	for _, name := range []string{"ann", "a_b-c", "org.example.fractions-1", "X9"} {
		if !safeName(name) {
			t.Errorf("%q should be a valid name", name)
		}
	}
	if entries, err := os.ReadDir(paths.Data); err == nil && len(entries) > 0 {
		t.Errorf("a refused name created %v", entries)
	}
}

func TestSaveRefusesInvalidProgress(t *testing.T) {
	s, paths := newStore(t)
	p := New(cassetteID)
	p.Exercises["e1"] = ExerciseProgress{State: "done"}
	wantTag(t, s.Save(context.Background(), "ann", p), "ProgressExerciseStateInvalid", "Store.saveFile->Progress.Validate")
	if _, err := os.Stat(progressFile(paths, "ann")); err == nil {
		t.Error("an invalid progress was written")
	}
}

func TestSaveNormalizes(t *testing.T) {
	s, paths := newStore(t)
	if err := s.Save(context.Background(), "ann", &Progress{CassetteID: cassetteID}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(progressFile(paths, "ann"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"format": 1`, `"lessons": {}`, `"exercises": {}`, `"submissions": []`, `"overrides": []`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("file lacks %s:\n%s", want, data)
		}
	}
	if strings.Contains(string(data), "orphaned") {
		t.Errorf("file has orphaned:\n%s", data)
	}
}

func TestSaveCancelled(t *testing.T) {
	s, _ := newStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	wantTag(t, s.Save(ctx, "ann", New(cassetteID)), "WriteCancelled", "Store.Save->Store.saveFile")
}

func TestUpdate(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	c := testCassette()

	err := s.Update(ctx, "ann", cassetteID, func(p *Progress) error {
		return CompleteLesson(c, p, "l1", t0)
	})
	if err != nil {
		t.Fatal(err)
	}
	err = s.Update(ctx, "ann", cassetteID, func(p *Progress) error {
		_, err := ApplyResults(c, p, []Result{res("e1", VerdictCorrect)}, t0)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Load(ctx, "ann", cassetteID)
	if err != nil || !p.Lessons["l1"].Completed() || p.Exercises["e1"].State != StatePassed {
		t.Errorf("after two updates: %+v, %v", p, err)
	}

	// a failing fn saves nothing and its error is returned untouched
	err = s.Update(ctx, "ann", cassetteID, func(p *Progress) error {
		p.LastItem = "changed"
		return CompleteLesson(c, p, "gone", t0)
	})
	wantTag(t, err, "CompleteLessonUnknown", "CompleteLesson.CompleteLessonUnknown")
	if p, _ := s.Load(ctx, "ann", cassetteID); p.LastItem != "" {
		t.Errorf("a failed update was saved: %q", p.LastItem)
	}

	// fn leaving an invalid Progress is refused by the save
	err = s.Update(ctx, "ann", cassetteID, func(p *Progress) error {
		p.Exercises["e1"] = ExerciseProgress{State: "done"}
		return nil
	})
	wantTag(t, err, "ProgressExerciseStateInvalid", "Store.Update->Store.saveFile->Progress.Validate")

	// cancelled
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	err = s.Update(cctx, "ann", cassetteID, func(*Progress) error { return nil })
	wantTag(t, err, "ProgressUpdateCancelled", "Store.Update.ProgressUpdateCancelled")

	// a cassette without a save file starts fresh
	err = s.Update(ctx, "ann", "org.example.zzz", func(*Progress) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateOnABrokenFile(t *testing.T) {
	s, paths := newStore(t)
	path := progressFile(paths, "ann")
	writeRaw(t, path, "{ nope")
	ran := false
	err := s.Update(context.Background(), "ann", cassetteID, func(p *Progress) error {
		ran = true
		p.LastItem = "e1"
		return nil
	})
	wantTag(t, err, "ProgressFileBroken", "Store.Update->Store.loadFile->Store.quarantine")
	if !ran {
		t.Fatal("fn did not run")
	}
	p, err := s.Load(context.Background(), "ann", cassetteID)
	if err != nil || p.LastItem != "e1" {
		t.Errorf("saved progress: %+v, %v", p, err)
	}
}

func TestUpdateLoadFailure(t *testing.T) {
	s, paths := newStore(t)
	if err := os.MkdirAll(progressFile(paths, "ann"), 0o700); err != nil {
		t.Fatal(err)
	}
	ran := false
	err := s.Update(context.Background(), "ann", cassetteID, func(*Progress) error { ran = true; return nil })
	wantTag(t, err, "JSONReadFailed", "Store.Update")
	if ran {
		t.Error("fn ran after a failed load")
	}
}

func TestConcurrentUpdatesLoseNothing(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	const workers, each = 8, 5
	var wg sync.WaitGroup
	errs := make(chan error, workers*each)
	for w := range workers {
		wg.Go(func() {
			for i := range each {
				id := fmt.Sprintf("x%d-%d", w, i)
				errs <- s.Update(ctx, "ann", cassetteID, func(p *Progress) error {
					p.Lessons[id] = LessonProgress{Plays: 1}
					return nil
				})
				if _, err := s.Load(ctx, "ann", cassetteID); err != nil {
					errs <- err
				}
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.Load(ctx, "ann", cassetteID)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Lessons) != workers*each {
		t.Errorf("%d lessons survived, want %d", len(p.Lessons), workers*each)
	}
}

func seed(t *testing.T, s *Store, username string, ids ...string) {
	t.Helper()
	for _, id := range ids {
		if err := s.Save(context.Background(), username, New(id)); err != nil {
			t.Fatal(err)
		}
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestDeleteAll(t *testing.T) {
	s, paths := newStore(t)
	ctx := context.Background()
	seed(t, s, "ann", cassetteID, "org.example.other-1")
	seed(t, s, "bob", cassetteID)
	broken := progressFile(paths, "ann") + ".broken-20261030T170000Z"
	writeRaw(t, broken, "x")
	lookalike := filepath.Join(paths.ProgressDir(), "ann", cassetteID+".jsonx")
	writeRaw(t, lookalike, "x")

	if err := s.DeleteAll(ctx, "ann", cassetteID); err != nil {
		t.Fatal(err)
	}
	if exists(progressFile(paths, "ann")) || exists(broken) {
		t.Error("the student's files for the cassette are still there")
	}
	for _, keep := range []string{
		filepath.Join(paths.ProgressDir(), "ann", "org.example.other-1.json"),
		progressFile(paths, "bob"),
		lookalike,
	} {
		if !exists(keep) {
			t.Errorf("%s was deleted", keep)
		}
	}
	// nothing there is fine, also for a student without a folder
	if err := s.DeleteAll(ctx, "ann", cassetteID); err != nil {
		t.Errorf("second DeleteAll: %v", err)
	}
	if err := s.DeleteAll(ctx, "zed", cassetteID); err != nil {
		t.Errorf("DeleteAll of an unknown student: %v", err)
	}
	p, err := s.Load(ctx, "ann", cassetteID)
	if err != nil || len(p.Lessons) != 0 {
		t.Errorf("after DeleteAll: %+v, %v", p, err)
	}
}

func TestDeleteAllFailures(t *testing.T) {
	ctx := context.Background()
	t.Run("cancelled", func(t *testing.T) {
		s, _ := newStore(t)
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		wantTag(t, s.DeleteAll(cctx, "ann", cassetteID), "ProgressDeleteCancelled", "Store.DeleteAll.ProgressDeleteCancelled")
	})
	t.Run("the student's folder is a file", func(t *testing.T) {
		storagetest.SkipOnWindows(t, "a path through a file reports not found, not not-a-folder")
		s, paths := newStore(t)
		writeRaw(t, filepath.Join(paths.ProgressDir(), "ann"), "x")
		wantTag(t, s.DeleteAll(ctx, "ann", cassetteID), "ProgressDirReadFailed", "Store.DeleteAll->removeCassetteFiles.ProgressDirReadFailed")
	})
	t.Run("the file cannot be removed", func(t *testing.T) {
		storagetest.SkipIfModesDoNotBlock(t)
		s, paths := newStore(t)
		seed(t, s, "ann", cassetteID)
		dir := filepath.Join(paths.ProgressDir(), "ann")
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
		wantTag(t, s.DeleteAll(ctx, "ann", cassetteID), "ProgressFileDeleteFailed", "Store.DeleteAll->removeCassetteFiles.ProgressFileDeleteFailed")
	})
}

func TestDeleteUser(t *testing.T) {
	s, paths := newStore(t)
	ctx := context.Background()
	seed(t, s, "ann", cassetteID, "org.example.other-1")
	seed(t, s, "bob", cassetteID)
	writeRaw(t, progressFile(paths, "ann")+".broken-20261030T170000Z", "x")

	if err := s.DeleteUser(ctx, "ann"); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(paths.ProgressDir(), "ann")) {
		t.Error("ann's folder is still there")
	}
	if !exists(progressFile(paths, "bob")) {
		t.Error("bob's file was deleted")
	}
	if err := s.DeleteUser(ctx, "ann"); err != nil {
		t.Errorf("second DeleteUser: %v", err)
	}

	cctx, cancel := context.WithCancel(ctx)
	cancel()
	wantTag(t, s.DeleteUser(cctx, "bob"), "ProgressUserDeleteCancelled", "Store.DeleteUser.ProgressUserDeleteCancelled")

	if os.Geteuid() != 0 && storagetest.UnixModes {
		dir := filepath.Join(paths.ProgressDir(), "bob")
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
		wantTag(t, s.DeleteUser(ctx, "bob"), "ProgressUserDeleteFailed", "Store.DeleteUser.ProgressUserDeleteFailed")
	}
}

func TestDeleteCassette(t *testing.T) {
	s, paths := newStore(t)
	ctx := context.Background()
	other := "org.example.other-1"
	// no progress folder yet
	if err := s.DeleteCassette(ctx, cassetteID); err != nil {
		t.Fatalf("DeleteCassette with no folder: %v", err)
	}
	seed(t, s, "ann", cassetteID, other)
	seed(t, s, "bob", cassetteID)
	seed(t, s, "cy", other)
	writeRaw(t, progressFile(paths, "bob")+".broken-20261030T170000Z", "x")
	writeRaw(t, filepath.Join(paths.ProgressDir(), "stray-file"), "x") // not a student folder

	if err := s.DeleteCassette(ctx, cassetteID); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{progressFile(paths, "ann"), progressFile(paths, "bob"), progressFile(paths, "bob") + ".broken-20261030T170000Z"} {
		if exists(gone) {
			t.Errorf("%s is still there", gone)
		}
	}
	for _, keep := range []string{
		filepath.Join(paths.ProgressDir(), "ann", other+".json"),
		filepath.Join(paths.ProgressDir(), "cy", other+".json"),
		filepath.Join(paths.ProgressDir(), "stray-file"),
	} {
		if !exists(keep) {
			t.Errorf("%s was deleted", keep)
		}
	}

	cctx, cancel := context.WithCancel(ctx)
	cancel()
	wantTag(t, s.DeleteCassette(cctx, cassetteID), "ProgressCassetteDeleteCancelled", "Store.DeleteCassette.ProgressCassetteDeleteCancelled")
}

func TestDeleteCassetteFailures(t *testing.T) {
	ctx := context.Background()
	t.Run("the progress folder is a file", func(t *testing.T) {
		storagetest.SkipOnWindows(t, "a path through a file reports not found, not not-a-folder")
		s, paths := newStore(t)
		writeRaw(t, paths.ProgressDir(), "x")
		wantTag(t, s.DeleteCassette(ctx, cassetteID), "ProgressUsersListFailed", "Store.DeleteCassette.ProgressUsersListFailed")
	})
	t.Run("a student's folder fails", func(t *testing.T) {
		storagetest.SkipIfModesDoNotBlock(t)
		s, paths := newStore(t)
		seed(t, s, "ann", cassetteID)
		dir := filepath.Join(paths.ProgressDir(), "ann")
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
		wantTag(t, s.DeleteCassette(ctx, cassetteID), "ProgressFileDeleteFailed", "Store.DeleteCassette->removeCassetteFiles.ProgressFileDeleteFailed")
	})
}

func TestWithClockIgnoresNil(t *testing.T) {
	paths := storagetest.Isolate(t)
	s := NewStore(paths, WithClock(nil))
	if s.now == nil {
		t.Fatal("nil clock replaced the default")
	}
}

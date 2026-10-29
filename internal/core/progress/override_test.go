package progress

import (
	"reflect"
	"testing"
	"time"
)

func TestApplyOverride(t *testing.T) {
	at := t0.Add(time.Hour + 900*time.Millisecond)
	atStored := t0.Add(time.Hour)
	tests := []struct {
		name          string
		before        func(p *Progress)
		o             Override
		wantExercises map[string]ExerciseProgress
		wantLessons   map[string]LessonProgress
		wantRecorded  Override
	}{
		{
			name: "mark passed after attempts ran out",
			before: func(p *Progress) {
				p.Exercises["e4"] = ExerciseProgress{State: StateNeedsReview, Attempts: 1, LastVerdict: VerdictIncorrect}
			},
			o: Override{ExerciseID: "e4", Action: ActionMarkPassed, By: "mom", At: at, Note: "she showed me"},
			wantExercises: map[string]ExerciseProgress{"e4": {State: StatePassed, Attempts: 1, PassedBy: PassedByParent,
				PassedAt: atStored, LastVerdict: VerdictIncorrect}},
			wantLessons:  map[string]LessonProgress{},
			wantRecorded: Override{ExerciseID: "e4", Action: ActionMarkPassed, By: "mom", At: atStored, Note: "she showed me"},
		},
		{
			name: "mark passed on a fresh exercise",
			o:    Override{ExerciseID: "e1", Action: ActionMarkPassed, By: "mom", At: at},
			wantExercises: map[string]ExerciseProgress{"e1": {State: StatePassed, PassedBy: PassedByParent,
				PassedAt: atStored}},
			wantLessons:  map[string]LessonProgress{},
			wantRecorded: Override{ExerciseID: "e1", Action: ActionMarkPassed, By: "mom", At: atStored},
		},
		{
			name: "mark not passed removes the pass and keeps the attempts",
			before: func(p *Progress) {
				p.Exercises["e1"] = ExerciseProgress{State: StatePassed, Attempts: 2, HintsShown: 1, PassedBy: PassedByAI,
					PassedAt: t0, LastVerdict: VerdictCorrect}
			},
			o: Override{ExerciseID: "e1", Action: ActionMarkNotPassed, By: "mom", At: at},
			wantExercises: map[string]ExerciseProgress{"e1": {Attempts: 2, HintsShown: 1,
				LastVerdict: VerdictCorrect}},
			wantLessons:  map[string]LessonProgress{},
			wantRecorded: Override{ExerciseID: "e1", Action: ActionMarkNotPassed, By: "mom", At: atStored},
		},
		{
			name: "mark not passed computes the state again: attempts used up is needs_review",
			before: func(p *Progress) {
				p.Exercises["e4"] = ExerciseProgress{State: StatePassed, Attempts: 1, PassedBy: PassedByParent, PassedAt: t0}
			},
			o:             Override{ExerciseID: "e4", Action: ActionMarkNotPassed, By: "mom", At: at},
			wantExercises: map[string]ExerciseProgress{"e4": {State: StateNeedsReview, Attempts: 1}},
			wantLessons:   map[string]LessonProgress{},
			wantRecorded:  Override{ExerciseID: "e4", Action: ActionMarkNotPassed, By: "mom", At: atStored},
		},
		{
			name:          "mark not passed on an exercise that was not passed",
			o:             Override{ExerciseID: "e2", Action: ActionMarkNotPassed, By: "mom", At: at},
			wantExercises: map[string]ExerciseProgress{"e2": {}},
			wantLessons:   map[string]LessonProgress{},
			wantRecorded:  Override{ExerciseID: "e2", Action: ActionMarkNotPassed, By: "mom", At: atStored},
		},
		{
			name: "reset attempts clears attempts, hints and state",
			before: func(p *Progress) {
				p.Exercises["e3"] = ExerciseProgress{State: StateNeedsReview, Attempts: 2, HintsShown: 1, LastVerdict: VerdictIncorrect}
				p.Exercises["e1"] = ExerciseProgress{Attempts: 1}
			},
			o:             Override{ExerciseID: "e3", Action: ActionResetAttempts, By: "mom", At: at},
			wantExercises: map[string]ExerciseProgress{"e1": {Attempts: 1}},
			wantLessons:   map[string]LessonProgress{},
			wantRecorded:  Override{ExerciseID: "e3", Action: ActionResetAttempts, By: "mom", At: atStored},
		},
		{
			name: "reset attempts also clears a pass",
			before: func(p *Progress) {
				pass(p, "e3")
			},
			o:             Override{ExerciseID: "e3", Action: ActionResetAttempts, By: "mom", At: at},
			wantExercises: map[string]ExerciseProgress{},
			wantLessons:   map[string]LessonProgress{},
			wantRecorded:  Override{ExerciseID: "e3", Action: ActionResetAttempts, By: "mom", At: atStored},
		},
		{
			name:          "mark completed",
			o:             Override{LessonID: "l2", Action: ActionMarkCompleted, By: "mom", At: at},
			wantExercises: map[string]ExerciseProgress{},
			wantLessons:   map[string]LessonProgress{"l2": {CompletedAt: atStored}},
			wantRecorded:  Override{LessonID: "l2", Action: ActionMarkCompleted, By: "mom", At: atStored},
		},
		{
			name: "mark completed keeps the first completion time",
			before: func(p *Progress) {
				p.Lessons["l2"] = LessonProgress{CompletedAt: t0, Plays: 3}
			},
			o:             Override{LessonID: "l2", Action: ActionMarkCompleted, By: "mom", At: at},
			wantExercises: map[string]ExerciseProgress{},
			wantLessons:   map[string]LessonProgress{"l2": {CompletedAt: t0, Plays: 3}},
			wantRecorded:  Override{LessonID: "l2", Action: ActionMarkCompleted, By: "mom", At: atStored},
		},
		{
			name: "mark not completed keeps the plays",
			before: func(p *Progress) {
				p.Lessons["l2"] = LessonProgress{CompletedAt: t0, Plays: 3}
			},
			o:             Override{LessonID: "l2", Action: ActionMarkNotCompleted, By: "mom", At: at},
			wantExercises: map[string]ExerciseProgress{},
			wantLessons:   map[string]LessonProgress{"l2": {Plays: 3}},
			wantRecorded:  Override{LessonID: "l2", Action: ActionMarkNotCompleted, By: "mom", At: atStored},
		},
		{
			name:          "the target of the other kind is dropped from the record",
			o:             Override{LessonID: "l1", ExerciseID: "e1", Action: ActionMarkCompleted, By: "mom", At: at},
			wantExercises: map[string]ExerciseProgress{},
			wantLessons:   map[string]LessonProgress{"l1": {CompletedAt: atStored}},
			wantRecorded:  Override{LessonID: "l1", Action: ActionMarkCompleted, By: "mom", At: atStored},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, p := testCassette(), newProgress()
			if tt.before != nil {
				tt.before(p)
			}
			if err := ApplyOverride(c, p, tt.o); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(p.Exercises, tt.wantExercises) {
				t.Errorf("exercises\n got %+v\nwant %+v", p.Exercises, tt.wantExercises)
			}
			if !reflect.DeepEqual(p.Lessons, tt.wantLessons) {
				t.Errorf("lessons\n got %+v\nwant %+v", p.Lessons, tt.wantLessons)
			}
			if len(p.Overrides) != 1 || p.Overrides[0] != tt.wantRecorded {
				t.Errorf("overrides = %+v, want [%+v]", p.Overrides, tt.wantRecorded)
			}
		})
	}
}

func TestApplyOverrideFailures(t *testing.T) {
	tests := []struct {
		name  string
		o     Override
		tag   string
		route string
	}{
		{"unknown exercise", Override{ExerciseID: "gone", Action: ActionMarkPassed}, "OverrideExerciseUnknown", "ApplyOverride.OverrideExerciseUnknown"},
		{"a lesson id for an exercise action", Override{ExerciseID: "l1", Action: ActionResetAttempts}, "OverrideExerciseUnknown", "ApplyOverride.OverrideExerciseUnknown"},
		{"unknown lesson", Override{LessonID: "gone", Action: ActionMarkCompleted}, "OverrideLessonUnknown", "ApplyOverride.OverrideLessonUnknown"},
		{"an exercise id for a lesson action", Override{LessonID: "e1", Action: ActionMarkNotCompleted}, "OverrideLessonUnknown", "ApplyOverride.OverrideLessonUnknown"},
		{"unknown action", Override{ExerciseID: "e1", Action: "explode"}, "OverrideActionInvalid", "ApplyOverride.OverrideActionInvalid"},
		{"reset_all is not an override of one target", Override{Action: ActionResetAll}, "OverrideActionInvalid", "ApplyOverride.OverrideActionInvalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, p := testCassette(), newProgress()
			wantTag(t, ApplyOverride(c, p, tt.o), tt.tag, tt.route)
			if len(p.Overrides) != 0 || len(p.Exercises) != 0 || len(p.Lessons) != 0 {
				t.Errorf("a failed override changed the progress: %+v", p)
			}
		})
	}
}

func TestOverrideUnblocksTheWorksheet(t *testing.T) {
	c, p := testCassette(), newProgress()
	complete(t, c, p, "l1", "l2")
	pass(p, "e1", "e2")
	if _, err := ApplyResults(c, p, []Result{res("e3", VerdictIncorrect), res("e4", VerdictIncorrect)}, t0); err != nil {
		t.Fatal(err)
	}
	sameStates(t, States(c, p), map[string]ItemState{"l1": StateCompleted, "e1": StatePassed, "e2": StatePassed,
		"l2": StateCompleted, "e3": StateAvailable, "e4": StateNeedsReview, "l3": StateLocked})
	for _, id := range []string{"e3", "e4"} {
		if err := ApplyOverride(c, p, Override{ExerciseID: id, Action: ActionMarkPassed, By: "dad", At: t0}); err != nil {
			t.Fatal(err)
		}
	}
	if got := States(c, p)["l3"]; got != StateAvailable {
		t.Errorf("l3 = %s after the parent passed the worksheet", got)
	}
	// resetting the attempts of e4 reopens it
	if err := ApplyOverride(c, p, Override{ExerciseID: "e4", Action: ActionResetAttempts, By: "dad", At: t0}); err != nil {
		t.Fatal(err)
	}
	if got := States(c, p)["e4"]; got != StateAvailable {
		t.Errorf("e4 = %s after reset, want available", got)
	}
}

func TestResetAll(t *testing.T) {
	build := func() (*Progress, Submission) {
		// the submission is normalized (empty, not nil, slices) as p.normalize does

		c, p := testCassette(), newProgress()
		complete(t, c, p, "l1")
		pass(p, "e1")
		p.LastItem = "e2"
		p.Settings = Settings{SkipLessons: true, FreeNavigation: true}
		p.Orphaned = &Orphaned{Lessons: map[string]LessonProgress{"old": {Plays: 1}}}
		sub := Submission{ID: "s1", Status: SubmissionGraded, ExerciseIDs: []string{}, Photos: []string{}, Results: []Result{res("e1", VerdictCorrect)}}
		p.Submissions = []Submission{sub}
		p.Overrides = []Override{{ExerciseID: "e1", Action: ActionMarkPassed, By: "mom", At: t0}}
		return p, sub
	}
	at := t0.Add(time.Hour)

	t.Run("keeps submissions, settings and the earlier overrides", func(t *testing.T) {
		p, sub := build()
		ResetAll(p, "mom", "new school year", at, false)
		if len(p.Lessons) != 0 || len(p.Exercises) != 0 || p.LastItem != "" || p.Orphaned != nil {
			t.Errorf("progress not cleared: %+v", p)
		}
		if !p.Settings.SkipLessons || !p.Settings.FreeNavigation {
			t.Errorf("settings changed: %+v", p.Settings)
		}
		if !reflect.DeepEqual(p.Submissions, []Submission{sub}) {
			t.Errorf("submissions = %+v", p.Submissions)
		}
		want := Override{Action: ActionResetAll, By: "mom", At: at, Note: "new school year"}
		if len(p.Overrides) != 2 || p.Overrides[1] != want {
			t.Errorf("overrides = %+v", p.Overrides)
		}
	})
	t.Run("drops the submissions when asked", func(t *testing.T) {
		p, _ := build()
		ResetAll(p, "mom", "", at, true)
		if p.Submissions == nil || len(p.Submissions) != 0 {
			t.Errorf("submissions = %#v, want empty non-nil", p.Submissions)
		}
		if len(p.Overrides) != 2 {
			t.Errorf("overrides = %+v", p.Overrides)
		}
	})
	t.Run("everything is locked again", func(t *testing.T) {
		p, _ := build()
		p.Settings = Settings{}
		ResetAll(p, "mom", "", at, false)
		if got := States(testCassette(), p)["e1"]; got != StateLocked {
			t.Errorf("e1 = %s", got)
		}
	})
}

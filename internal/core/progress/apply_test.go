package progress

import (
	"reflect"
	"testing"
	"time"
)

func res(id string, v Verdict) Result { return Result{ExerciseID: id, Verdict: v} }

func TestApplyResults(t *testing.T) {
	now := t0.Add(5*time.Minute + 700*time.Millisecond)
	tests := []struct {
		name     string
		before   map[string]ExerciseProgress
		results  []Result
		want     map[string]ExerciseProgress
		changes  []ExerciseChange
		passed   []string
		hints    []string
		review   []string
		counted  []bool
		wantNone bool
	}{
		{
			name:    "correct passes",
			results: []Result{res("e1", VerdictCorrect)},
			want: map[string]ExerciseProgress{"e1": {State: StatePassed, Attempts: 1, PassedBy: PassedByAI,
				PassedAt: now.Truncate(time.Second), LastVerdict: VerdictCorrect}},
			changes: []ExerciseChange{{ExerciseID: "e1", Verdict: VerdictCorrect, CountedAttempt: true, Passed: true}},
			passed:  []string{"e1"},
			counted: []bool{true},
		},
		{
			name:    "partial uses an attempt and unlocks the first hint",
			results: []Result{res("e1", VerdictPartial)},
			want:    map[string]ExerciseProgress{"e1": {Attempts: 1, HintsShown: 1, LastVerdict: VerdictPartial}},
			changes: []ExerciseChange{{ExerciseID: "e1", Verdict: VerdictPartial, CountedAttempt: true, HintShown: 1}},
			hints:   []string{"e1"},
			counted: []bool{true},
		},
		{
			name:    "incorrect uses an attempt and unlocks the next hint",
			before:  map[string]ExerciseProgress{"e1": {Attempts: 1, HintsShown: 1, LastVerdict: VerdictPartial}},
			results: []Result{res("e1", VerdictIncorrect)},
			want:    map[string]ExerciseProgress{"e1": {Attempts: 2, HintsShown: 2, LastVerdict: VerdictIncorrect}},
			changes: []ExerciseChange{{ExerciseID: "e1", Verdict: VerdictIncorrect, CountedAttempt: true, HintShown: 2}},
			hints:   []string{"e1"},
			counted: []bool{true},
		},
		{
			name:    "no hint left: the attempt counts, hints stay",
			before:  map[string]ExerciseProgress{"e1": {Attempts: 0, HintsShown: 2}},
			results: []Result{res("e1", VerdictIncorrect)},
			want:    map[string]ExerciseProgress{"e1": {Attempts: 1, HintsShown: 2, LastVerdict: VerdictIncorrect}},
			changes: []ExerciseChange{{ExerciseID: "e1", Verdict: VerdictIncorrect, CountedAttempt: true}},
			counted: []bool{true},
		},
		{
			name:    "an exercise without hints",
			results: []Result{res("e2", VerdictIncorrect)},
			want:    map[string]ExerciseProgress{"e2": {Attempts: 1, LastVerdict: VerdictIncorrect}},
			changes: []ExerciseChange{{ExerciseID: "e2", Verdict: VerdictIncorrect, CountedAttempt: true}},
			counted: []bool{true},
		},
		{
			name:    "unreadable and not_found use no attempt and no hint but are remembered",
			results: []Result{res("e1", VerdictUnreadable), res("e3", VerdictNotFound)},
			want: map[string]ExerciseProgress{
				"e1": {LastVerdict: VerdictUnreadable},
				"e3": {LastVerdict: VerdictNotFound},
			},
			changes: []ExerciseChange{
				{ExerciseID: "e1", Verdict: VerdictUnreadable},
				{ExerciseID: "e3", Verdict: VerdictNotFound},
			},
			counted: []bool{false, false},
		},
		{
			name:    "the last attempt used becomes needs_review, with the hint shown too",
			before:  map[string]ExerciseProgress{"e3": {Attempts: 1}},
			results: []Result{res("e3", VerdictIncorrect)},
			want:    map[string]ExerciseProgress{"e3": {State: StateNeedsReview, Attempts: 2, HintsShown: 1, LastVerdict: VerdictIncorrect}},
			changes: []ExerciseChange{{ExerciseID: "e3", Verdict: VerdictIncorrect, CountedAttempt: true, HintShown: 1, NeedsReview: true}},
			hints:   []string{"e3"},
			review:  []string{"e3"},
			counted: []bool{true},
		},
		{
			name:    "correct on the last attempt passes",
			before:  map[string]ExerciseProgress{"e4": {}},
			results: []Result{res("e4", VerdictCorrect)},
			want: map[string]ExerciseProgress{"e4": {State: StatePassed, Attempts: 1, PassedBy: PassedByAI,
				PassedAt: now.Truncate(time.Second), LastVerdict: VerdictCorrect}},
			changes: []ExerciseChange{{ExerciseID: "e4", Verdict: VerdictCorrect, CountedAttempt: true, Passed: true}},
			passed:  []string{"e4"},
			counted: []bool{true},
		},
		{
			name:    "max_attempts 1: one wrong try is needs_review",
			results: []Result{res("e4", VerdictPartial)},
			want:    map[string]ExerciseProgress{"e4": {State: StateNeedsReview, Attempts: 1, LastVerdict: VerdictPartial}},
			changes: []ExerciseChange{{ExerciseID: "e4", Verdict: VerdictPartial, CountedAttempt: true, NeedsReview: true}},
			review:  []string{"e4"},
			counted: []bool{true},
		},
		{
			name:    "an unlimited exercise never needs review",
			before:  map[string]ExerciseProgress{"e2": {Attempts: 99}},
			results: []Result{res("e2", VerdictIncorrect)},
			want:    map[string]ExerciseProgress{"e2": {Attempts: 100, LastVerdict: VerdictIncorrect}},
			changes: []ExerciseChange{{ExerciseID: "e2", Verdict: VerdictIncorrect, CountedAttempt: true}},
			counted: []bool{true},
		},
		{
			name: "results for a passed exercise change nothing",
			before: map[string]ExerciseProgress{"e1": {State: StatePassed, Attempts: 1, PassedBy: PassedByParent,
				PassedAt: t0, LastVerdict: VerdictPartial}},
			results: []Result{res("e1", VerdictIncorrect)},
			want: map[string]ExerciseProgress{"e1": {State: StatePassed, Attempts: 1, PassedBy: PassedByParent,
				PassedAt: t0, LastVerdict: VerdictPartial}},
			changes:  []ExerciseChange{{ExerciseID: "e1", Verdict: VerdictIncorrect, Ignored: IgnoredAlreadyPassed}},
			counted:  []bool{false},
			wantNone: true,
		},
		{
			name:     "results for an exercise waiting for a parent change nothing",
			before:   map[string]ExerciseProgress{"e4": {State: StateNeedsReview, Attempts: 1}},
			results:  []Result{res("e4", VerdictCorrect)},
			want:     map[string]ExerciseProgress{"e4": {State: StateNeedsReview, Attempts: 1}},
			changes:  []ExerciseChange{{ExerciseID: "e4", Verdict: VerdictCorrect, Ignored: IgnoredNeedsReview}},
			counted:  []bool{false},
			wantNone: true,
		},
		{
			name:     "a result for an exercise the cassette lacks is ignored",
			results:  []Result{res("gone", VerdictCorrect), res("l1", VerdictCorrect)},
			want:     map[string]ExerciseProgress{},
			changes:  []ExerciseChange{{ExerciseID: "gone", Verdict: VerdictCorrect, Ignored: IgnoredUnknownExercise}, {ExerciseID: "l1", Verdict: VerdictCorrect, Ignored: IgnoredUnknownExercise}},
			counted:  []bool{false, false},
			wantNone: true,
		},
		{
			name:    "several exercises of one worksheet at once",
			results: []Result{res("e3", VerdictCorrect), res("e4", VerdictIncorrect)},
			want: map[string]ExerciseProgress{
				"e3": {State: StatePassed, Attempts: 1, PassedBy: PassedByAI, PassedAt: now.Truncate(time.Second), LastVerdict: VerdictCorrect},
				"e4": {State: StateNeedsReview, Attempts: 1, LastVerdict: VerdictIncorrect},
			},
			changes: []ExerciseChange{
				{ExerciseID: "e3", Verdict: VerdictCorrect, CountedAttempt: true, Passed: true},
				{ExerciseID: "e4", Verdict: VerdictIncorrect, CountedAttempt: true, NeedsReview: true},
			},
			passed:  []string{"e3"},
			review:  []string{"e4"},
			counted: []bool{true, true},
		},
		{
			name:    "the same exercise twice in one call is applied twice",
			results: []Result{res("e1", VerdictIncorrect), res("e1", VerdictCorrect)},
			want: map[string]ExerciseProgress{"e1": {State: StatePassed, Attempts: 2, HintsShown: 1, PassedBy: PassedByAI,
				PassedAt: now.Truncate(time.Second), LastVerdict: VerdictCorrect}},
			changes: []ExerciseChange{
				{ExerciseID: "e1", Verdict: VerdictIncorrect, CountedAttempt: true, HintShown: 1},
				{ExerciseID: "e1", Verdict: VerdictCorrect, CountedAttempt: true, Passed: true},
			},
			passed: []string{"e1"}, hints: []string{"e1"},
			counted: []bool{true, true},
		},
		{
			name:    "no results",
			results: nil,
			want:    map[string]ExerciseProgress{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, p := testCassette(), newProgress()
			for id, ep := range tt.before {
				p.Exercises[id] = ep
			}
			// stale CountedAttempt values must be overwritten
			for i := range tt.results {
				tt.results[i].CountedAttempt = true
			}
			got, err := ApplyResults(c, p, tt.results, now)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(p.Exercises, tt.want) {
				t.Errorf("exercises\n got %+v\nwant %+v", p.Exercises, tt.want)
			}
			if len(got.Exercises) != len(tt.changes) {
				t.Fatalf("changes = %+v, want %+v", got.Exercises, tt.changes)
			}
			for i := range tt.changes {
				if got.Exercises[i] != tt.changes[i] {
					t.Errorf("change %d = %+v, want %+v", i, got.Exercises[i], tt.changes[i])
				}
			}
			for name, pair := range map[string][2][]string{
				"NewlyPassed":   {got.NewlyPassed, tt.passed},
				"HintsUnlocked": {got.HintsUnlocked, tt.hints},
				"NeedsReview":   {got.NeedsReview, tt.review},
			} {
				if len(pair[0]) != len(pair[1]) || (len(pair[0]) > 0 && !reflect.DeepEqual(pair[0], pair[1])) {
					t.Errorf("%s = %v, want %v", name, pair[0], pair[1])
				}
			}
			for i, w := range tt.counted {
				if tt.results[i].CountedAttempt != w {
					t.Errorf("results[%d].CountedAttempt = %v, want %v", i, tt.results[i].CountedAttempt, w)
				}
			}
		})
	}
}

func TestApplyResultsInvalidVerdictChangesNothing(t *testing.T) {
	c, p := testCassette(), newProgress()
	results := []Result{res("e1", VerdictIncorrect), res("e2", "maybe")}
	_, err := ApplyResults(c, p, results, t0)
	wantTag(t, err, "ResultVerdictInvalid", "ApplyResults.ResultVerdictInvalid")
	if len(p.Exercises) != 0 {
		t.Errorf("exercises changed: %+v", p.Exercises)
	}
}

func TestApplyResultsThenStates(t *testing.T) {
	// A whole story: a child fails e3 once, passes e4, then a parent decides.
	c, p := testCassette(), newProgress()
	complete(t, c, p, "l1")
	pass(p, "e1", "e2")
	complete(t, c, p, "l2")
	if _, err := ApplyResults(c, p, []Result{res("e3", VerdictPartial), res("e4", VerdictCorrect)}, t0); err != nil {
		t.Fatal(err)
	}
	sameStates(t, States(c, p), map[string]ItemState{"l1": StateCompleted, "e1": StatePassed, "e2": StatePassed,
		"l2": StateCompleted, "e3": StateAvailable, "e4": StatePassed, "l3": StateLocked})
	if _, err := ApplyResults(c, p, []Result{res("e3", VerdictCorrect)}, t0); err != nil {
		t.Fatal(err)
	}
	if got := States(c, p)["l3"]; got != StateAvailable {
		t.Errorf("l3 = %s, want available once the worksheet is passed", got)
	}
}

func TestLessons(t *testing.T) {
	c, p := testCassette(), newProgress()
	if err := PlayLesson(c, p, "l1"); err != nil {
		t.Fatal(err)
	}
	if err := PlayLesson(c, p, "l1"); err != nil {
		t.Fatal(err)
	}
	if p.Lessons["l1"].Plays != 2 || p.Lessons["l1"].Completed() {
		t.Errorf("after two plays: %+v", p.Lessons["l1"])
	}
	first := t0.Add(1500 * time.Millisecond)
	if err := CompleteLesson(c, p, "l1", first); err != nil {
		t.Fatal(err)
	}
	if err := CompleteLesson(c, p, "l1", first.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	want := LessonProgress{CompletedAt: t0.Add(time.Second), Plays: 2}
	if p.Lessons["l1"] != want {
		t.Errorf("l1 = %+v, want %+v (first completion kept, whole seconds, UTC)", p.Lessons["l1"], want)
	}

	wantTag(t, PlayLesson(c, p, "e1"), "PlayLessonUnknown", "PlayLesson.PlayLessonUnknown")
	wantTag(t, CompleteLesson(c, p, "gone", t0), "CompleteLessonUnknown", "CompleteLesson.CompleteLessonUnknown")
	if _, ok := p.Lessons["gone"]; ok {
		t.Error("an unknown lesson got an entry")
	}
}

func TestSetLastItem(t *testing.T) {
	c, p := testCassette(), newProgress()
	for _, id := range []string{"l1", "e3"} {
		if err := SetLastItem(c, p, id); err != nil || p.LastItem != id {
			t.Errorf("SetLastItem(%s): %v, last item %q", id, err, p.LastItem)
		}
	}
	wantTag(t, SetLastItem(c, p, "gone"), "LastItemUnknown", "SetLastItem.LastItemUnknown")
	if p.LastItem != "e3" {
		t.Errorf("a failed SetLastItem changed the last item to %q", p.LastItem)
	}
}

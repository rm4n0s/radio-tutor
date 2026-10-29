package progress

import (
	"reflect"
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
)

// v2 is testCassette changed like a new version of a cassette: l3 and e2 are
// removed, "e4" becomes a lesson, "l2" becomes an exercise, e1 has one hint and
// 2 attempts, e3 has no limit, and there is a new lesson.
func v2() *cassette.Cassette {
	c := testCassette()
	c.Manifest.Version = cassette.Version{Major: 2}
	c.Manifest.Items = []cassette.Item{
		item(cassette.ItemLesson, "l1"),
		item(cassette.ItemExercise, "e1"),
		item(cassette.ItemExercise, "l2"),
		item(cassette.ItemExercise, "e3"),
		item(cassette.ItemLesson, "e4"),
		item(cassette.ItemLesson, "new"),
	}
	c.Lessons = map[string]*cassette.Lesson{"l1": {ID: "l1"}, "e4": {ID: "e4"}, "new": {ID: "new"}}
	c.Exercises = map[string]*cassette.Exercise{
		"e1": {ID: "e1", MaxAttempts: 2, Hints: []string{"h1"}},
		"l2": {ID: "l2", MaxAttempts: 3},
		"e3": {ID: "e3", MaxAttempts: 0, Hints: []string{"a"}},
	}
	return c
}

func TestMigrate(t *testing.T) {
	c, p := v2(), newProgress()
	p.CassetteVersion = cassette.Version{Major: 1}
	p.LastItem = "l3" // removed
	p.Lessons = map[string]LessonProgress{
		"l1": {CompletedAt: t0, Plays: 2}, // kept
		"l2": {CompletedAt: t0, Plays: 1}, // now an exercise
		"l3": {CompletedAt: t0, Plays: 4}, // removed
	}
	p.Exercises = map[string]ExerciseProgress{
		"e1": {Attempts: 2, HintsShown: 2, LastVerdict: VerdictPartial},                 // limit now 2: needs review; one hint
		"e2": {State: StatePassed, Attempts: 1, PassedBy: PassedByAI, PassedAt: t0},     // removed
		"e3": {State: StateNeedsReview, Attempts: 2, HintsShown: 1},                     // no limit now
		"e4": {State: StatePassed, Attempts: 1, PassedBy: PassedByParent, PassedAt: t0}, // now a lesson
	}
	p.Submissions = []Submission{{ID: "s1", Status: SubmissionGraded, Results: []Result{res("e2", VerdictCorrect)}}}
	p.Overrides = []Override{{ExerciseID: "e2", Action: ActionMarkPassed, By: "mom", At: t0}}

	m, err := Migrate(c, p)
	if err != nil {
		t.Fatal(err)
	}
	wantM := Migration{
		OrphanedLessons:   []string{"l2", "l3"},
		OrphanedExercises: []string{"e2", "e4"},
		AdjustedExercises: []string{"e1", "e3"},
		LastItemCleared:   true,
	}
	if !reflect.DeepEqual(m, wantM) {
		t.Errorf("migration = %+v\nwant        %+v", m, wantM)
	}
	if p.CassetteVersion != c.Manifest.Version {
		t.Errorf("version = %v", p.CassetteVersion)
	}
	if p.LastItem != "" {
		t.Errorf("last item = %q", p.LastItem)
	}
	wantLessons := map[string]LessonProgress{"l1": {CompletedAt: t0, Plays: 2}}
	if !reflect.DeepEqual(p.Lessons, wantLessons) {
		t.Errorf("lessons = %+v", p.Lessons)
	}
	wantExercises := map[string]ExerciseProgress{
		"e1": {State: StateNeedsReview, Attempts: 2, HintsShown: 1, LastVerdict: VerdictPartial},
		"e3": {Attempts: 2, HintsShown: 1},
	}
	if !reflect.DeepEqual(p.Exercises, wantExercises) {
		t.Errorf("exercises = %+v", p.Exercises)
	}
	wantOrphaned := &Orphaned{
		Lessons: map[string]LessonProgress{"l2": {CompletedAt: t0, Plays: 1}, "l3": {CompletedAt: t0, Plays: 4}},
		Exercises: map[string]ExerciseProgress{
			"e2": {State: StatePassed, Attempts: 1, PassedBy: PassedByAI, PassedAt: t0},
			"e4": {State: StatePassed, Attempts: 1, PassedBy: PassedByParent, PassedAt: t0},
		},
	}
	if !reflect.DeepEqual(p.Orphaned, wantOrphaned) {
		t.Errorf("orphaned = %+v", p.Orphaned)
	}
	if len(p.Submissions) != 1 || len(p.Overrides) != 1 {
		t.Error("history was changed")
	}
	// the orphaned entries are ignored by the rules
	if got := States(c, p)["e4"]; got != StateAvailable && got != StateLocked {
		t.Errorf("e4 (now a lesson) = %s, want an entry-less state", got)
	}

	// idempotent
	before := *p
	m2, err := Migrate(c, p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m2, Migration{}) || !reflect.DeepEqual(*p, before) {
		t.Errorf("second Migrate changed things: %+v", m2)
	}
}

func TestMigrateKeepsWhatStays(t *testing.T) {
	// A newer version that only adds an item: nothing moves.
	c := testCassette()
	c.Manifest.Version = cassette.Version{Major: 1, Minor: 1}
	c.Manifest.Items = append(c.Manifest.Items, item(cassette.ItemLesson, "l4"))
	c.Lessons["l4"] = &cassette.Lesson{ID: "l4"}

	p := newProgress()
	p.LastItem = "e3"
	complete(t, c, p, "l1", "l2")
	pass(p, "e1")
	p.Exercises["e3"] = ExerciseProgress{Attempts: 1, HintsShown: 1, LastVerdict: VerdictIncorrect}
	want := *p

	m, err := Migrate(c, p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m, Migration{}) {
		t.Errorf("migration = %+v", m)
	}
	want.CassetteVersion = c.Manifest.Version
	if !reflect.DeepEqual(*p, want) {
		t.Errorf("progress changed:\n got %+v\nwant %+v", *p, want)
	}
	if p.Orphaned != nil {
		t.Errorf("orphaned = %+v, want none", p.Orphaned)
	}
	if got := States(c, p)["l2"]; got != StateCompleted {
		t.Errorf("l2 = %s: a kept lesson stays completed", got)
	}
}

func TestMigrateReplacesAnEarlierOrphan(t *testing.T) {
	c, p := v2(), newProgress()
	p.Orphaned = &Orphaned{Lessons: map[string]LessonProgress{"l3": {Plays: 1}, "keep": {Plays: 7}}}
	p.Lessons["l3"] = LessonProgress{Plays: 9}
	if _, err := Migrate(c, p); err != nil {
		t.Fatal(err)
	}
	want := map[string]LessonProgress{"l3": {Plays: 9}, "keep": {Plays: 7}}
	if !reflect.DeepEqual(p.Orphaned.Lessons, want) {
		t.Errorf("orphaned lessons = %+v", p.Orphaned.Lessons)
	}
}

func TestMigrateOtherCassette(t *testing.T) {
	p := New("org.example.other-1")
	_, err := Migrate(testCassette(), p)
	wantTag(t, err, "MigrateCassetteMismatch", "Migrate.MigrateCassetteMismatch")
}

package progress

import (
	"maps"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
)

// t0 is the time of the tests; later times add to it.
var t0 = time.Date(2026, 10, 30, 17, 0, 0, 0, time.UTC)

// item makes a manifest item.
func item(typ cassette.ItemType, id string) cassette.Item {
	return cassette.Item{Type: typ, ID: id}
}

// testCassette is a cassette with two worksheets:
//
//	l1  e1 e2  l2  e3 e4  l3
//
// e1: 3 attempts, 2 hints; e2: no limit, no hints; e3: 2 attempts, 1 hint;
// e4: 1 attempt, no hints.
func testCassette() *cassette.Cassette {
	return &cassette.Cassette{
		Manifest: cassette.Manifest{
			ID:      "org.example.test-1",
			Version: cassette.Version{Major: 1},
			Items: []cassette.Item{
				item(cassette.ItemLesson, "l1"),
				item(cassette.ItemExercise, "e1"), item(cassette.ItemExercise, "e2"),
				item(cassette.ItemLesson, "l2"),
				item(cassette.ItemExercise, "e3"), item(cassette.ItemExercise, "e4"),
				item(cassette.ItemLesson, "l3"),
			},
		},
		Lessons: map[string]*cassette.Lesson{"l1": {ID: "l1"}, "l2": {ID: "l2"}, "l3": {ID: "l3"}},
		Exercises: map[string]*cassette.Exercise{
			"e1": {ID: "e1", MaxAttempts: 3, Hints: []string{"h1", "h2"}},
			"e2": {ID: "e2", MaxAttempts: 0},
			"e3": {ID: "e3", MaxAttempts: 2, Hints: []string{"a"}},
			"e4": {ID: "e4", MaxAttempts: 1},
		},
	}
}

// newProgress is an empty Progress for testCassette.
func newProgress() *Progress { return New("org.example.test-1") }

// complete completes lessons through CompleteLesson.
func complete(t *testing.T, c *cassette.Cassette, p *Progress, ids ...string) {
	t.Helper()
	for _, id := range ids {
		if err := CompleteLesson(c, p, id, t0); err != nil {
			t.Fatal(err)
		}
	}
}

// pass marks exercises passed by the AI.
func pass(p *Progress, ids ...string) {
	for _, id := range ids {
		p.Exercises[id] = ExerciseProgress{State: StatePassed, Attempts: 1, PassedBy: PassedByAI, PassedAt: t0, LastVerdict: VerdictCorrect}
	}
}

// wantTag fails unless err is an *errors.Error with the tag and the route.
func wantTag(t *testing.T, err error, tag, route string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s, got no error", tag)
	}
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T: %v", err, err)
	}
	if appErr.Tag != tag || !appErr.HasRoute(route) {
		t.Fatalf("unexpected failure: tag %s, route %s; want tag %s, route %s", appErr.Tag, appErr.Route(), tag, route)
	}
}

// sameStates fails unless got equals want.
func sameStates(t *testing.T, got, want map[string]ItemState) {
	t.Helper()
	if !maps.Equal(got, want) {
		t.Errorf("states\n got %v\nwant %v", got, want)
	}
}

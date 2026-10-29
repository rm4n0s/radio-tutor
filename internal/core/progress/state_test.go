package progress

import (
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
)

func TestStates(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, c *cassette.Cassette, p *Progress)
		want  map[string]ItemState
	}{
		{
			name:  "fresh progress opens only the first lesson",
			setup: func(*testing.T, *cassette.Cassette, *Progress) {},
			want: map[string]ItemState{"l1": StateAvailable, "e1": StateLocked, "e2": StateLocked,
				"l2": StateLocked, "e3": StateLocked, "e4": StateLocked, "l3": StateLocked},
		},
		{
			name: "completing the lesson opens the whole worksheet at once",
			setup: func(t *testing.T, c *cassette.Cassette, p *Progress) {
				complete(t, c, p, "l1")
			},
			want: map[string]ItemState{"l1": StateCompleted, "e1": StateAvailable, "e2": StateAvailable,
				"l2": StateLocked, "e3": StateLocked, "e4": StateLocked, "l3": StateLocked},
		},
		{
			name: "one exercise passed keeps the next lesson locked",
			setup: func(t *testing.T, c *cassette.Cassette, p *Progress) {
				complete(t, c, p, "l1")
				pass(p, "e1")
			},
			want: map[string]ItemState{"l1": StateCompleted, "e1": StatePassed, "e2": StateAvailable,
				"l2": StateLocked, "e3": StateLocked, "e4": StateLocked, "l3": StateLocked},
		},
		{
			name: "the whole worksheet passed opens the next lesson",
			setup: func(t *testing.T, c *cassette.Cassette, p *Progress) {
				complete(t, c, p, "l1")
				pass(p, "e1", "e2")
			},
			want: map[string]ItemState{"l1": StateCompleted, "e1": StatePassed, "e2": StatePassed,
				"l2": StateAvailable, "e3": StateLocked, "e4": StateLocked, "l3": StateLocked},
		},
		{
			name: "needs_review blocks what follows",
			setup: func(t *testing.T, c *cassette.Cassette, p *Progress) {
				complete(t, c, p, "l1")
				pass(p, "e2")
				p.Exercises["e1"] = ExerciseProgress{State: StateNeedsReview, Attempts: 3}
			},
			want: map[string]ItemState{"l1": StateCompleted, "e1": StateNeedsReview, "e2": StatePassed,
				"l2": StateLocked, "e3": StateLocked, "e4": StateLocked, "l3": StateLocked},
		},
		{
			name: "attempts used up show needs_review even without the stored state",
			setup: func(t *testing.T, c *cassette.Cassette, p *Progress) {
				complete(t, c, p, "l1")
				p.Exercises["e1"] = ExerciseProgress{Attempts: 3}
			},
			want: map[string]ItemState{"l1": StateCompleted, "e1": StateNeedsReview, "e2": StateAvailable,
				"l2": StateLocked, "e3": StateLocked, "e4": StateLocked, "l3": StateLocked},
		},
		{
			name: "attempts below the limit stay available; no limit never needs review",
			setup: func(t *testing.T, c *cassette.Cassette, p *Progress) {
				complete(t, c, p, "l1")
				p.Exercises["e1"] = ExerciseProgress{Attempts: 2}
				p.Exercises["e2"] = ExerciseProgress{Attempts: 50}
			},
			want: map[string]ItemState{"l1": StateCompleted, "e1": StateAvailable, "e2": StateAvailable,
				"l2": StateLocked, "e3": StateLocked, "e4": StateLocked, "l3": StateLocked},
		},
		{
			name: "skip lessons: exercises open without the lesson, lessons still show locked or available by the rule",
			setup: func(_ *testing.T, _ *cassette.Cassette, p *Progress) {
				p.Settings.SkipLessons = true
			},
			want: map[string]ItemState{"l1": StateAvailable, "e1": StateAvailable, "e2": StateAvailable,
				"l2": StateLocked, "e3": StateLocked, "e4": StateLocked, "l3": StateLocked},
		},
		{
			name: "skip lessons: passing the worksheet opens the next lesson and its exercises",
			setup: func(_ *testing.T, _ *cassette.Cassette, p *Progress) {
				p.Settings.SkipLessons = true
				pass(p, "e1", "e2")
			},
			want: map[string]ItemState{"l1": StateAvailable, "e1": StatePassed, "e2": StatePassed,
				"l2": StateAvailable, "e3": StateAvailable, "e4": StateAvailable, "l3": StateLocked},
		},
		{
			name: "skip lessons does not skip exercises",
			setup: func(_ *testing.T, _ *cassette.Cassette, p *Progress) {
				p.Settings.SkipLessons = true
				pass(p, "e1")
			},
			want: map[string]ItemState{"l1": StateAvailable, "e1": StatePassed, "e2": StateAvailable,
				"l2": StateLocked, "e3": StateLocked, "e4": StateLocked, "l3": StateLocked},
		},
		{
			name: "free navigation opens everything and keeps stored states",
			setup: func(t *testing.T, c *cassette.Cassette, p *Progress) {
				p.Settings.FreeNavigation = true
				complete(t, c, p, "l2")
				pass(p, "e3")
				p.Exercises["e4"] = ExerciseProgress{State: StateNeedsReview, Attempts: 1}
			},
			want: map[string]ItemState{"l1": StateAvailable, "e1": StateAvailable, "e2": StateAvailable,
				"l2": StateCompleted, "e3": StatePassed, "e4": StateNeedsReview, "l3": StateAvailable},
		},
		{
			name: "a passed exercise behind a locked item keeps its state",
			setup: func(_ *testing.T, _ *cassette.Cassette, p *Progress) {
				pass(p, "e3")
			},
			want: map[string]ItemState{"l1": StateAvailable, "e1": StateLocked, "e2": StateLocked,
				"l2": StateLocked, "e3": StatePassed, "e4": StateLocked, "l3": StateLocked},
		},
		{
			name: "everything done",
			setup: func(t *testing.T, c *cassette.Cassette, p *Progress) {
				complete(t, c, p, "l1", "l2", "l3")
				pass(p, "e1", "e2", "e3", "e4")
			},
			want: map[string]ItemState{"l1": StateCompleted, "e1": StatePassed, "e2": StatePassed,
				"l2": StateCompleted, "e3": StatePassed, "e4": StatePassed, "l3": StateCompleted},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, p := testCassette(), newProgress()
			tt.setup(t, c, p)
			sameStates(t, States(c, p), tt.want)
		})
	}
}

func TestContinue(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(t *testing.T, c *cassette.Cassette, p *Progress)
		want   string
		wantOK bool
	}{
		{"fresh: the first item", func(*testing.T, *cassette.Cassette, *Progress) {}, "l1", true},
		{
			"last item is opened again",
			func(t *testing.T, c *cassette.Cassette, p *Progress) {
				complete(t, c, p, "l1")
				p.LastItem = "e2"
			},
			"e2", true,
		},
		{
			"last item that is a completed lesson is opened again",
			func(t *testing.T, c *cassette.Cassette, p *Progress) {
				complete(t, c, p, "l1")
				p.LastItem = "l1"
			},
			"l1", true,
		},
		{
			"last item that is locked falls back to the first available",
			func(t *testing.T, c *cassette.Cassette, p *Progress) {
				complete(t, c, p, "l1")
				p.LastItem = "l3"
			},
			"e1", true,
		},
		{
			"last item that no longer exists falls back to the first available",
			func(t *testing.T, c *cassette.Cassette, p *Progress) {
				complete(t, c, p, "l1")
				p.LastItem = "gone"
			},
			"e1", true,
		},
		{
			"no last item: the first available item",
			func(t *testing.T, c *cassette.Cassette, p *Progress) {
				complete(t, c, p, "l1")
				pass(p, "e1", "e2")
			},
			"l2", true,
		},
		{
			"first available skips a needs_review exercise",
			func(t *testing.T, c *cassette.Cassette, p *Progress) {
				complete(t, c, p, "l1")
				p.Exercises["e1"] = ExerciseProgress{State: StateNeedsReview, Attempts: 3}
			},
			"e2", true,
		},
		{
			"nothing available (a needs_review blocks): the first item",
			func(t *testing.T, c *cassette.Cassette, p *Progress) {
				complete(t, c, p, "l1")
				p.Exercises["e1"] = ExerciseProgress{State: StateNeedsReview, Attempts: 3}
				pass(p, "e2")
			},
			"l1", true,
		},
		{
			"everything done: the first item",
			func(t *testing.T, c *cassette.Cassette, p *Progress) {
				complete(t, c, p, "l1", "l2", "l3")
				pass(p, "e1", "e2", "e3", "e4")
			},
			"l1", true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, p := testCassette(), newProgress()
			tt.setup(t, c, p)
			got, ok := Continue(c, p)
			if ok != tt.wantOK || got.ID != tt.want {
				t.Errorf("Continue = %q, %v; want %q, %v", got.ID, ok, tt.want, tt.wantOK)
			}
		})
	}

	t.Run("a cassette without items", func(t *testing.T) {
		if _, ok := Continue(&cassette.Cassette{}, newProgress()); ok {
			t.Error("Continue found an item in an empty cassette")
		}
	})
}

func TestVisibleHintsAndAttemptsLeft(t *testing.T) {
	c, p := testCassette(), newProgress()
	p.Exercises["e1"] = ExerciseProgress{Attempts: 1, HintsShown: 1}
	p.Exercises["e3"] = ExerciseProgress{Attempts: 5, HintsShown: 9} // more than the exercise has
	p.Exercises["e4"] = ExerciseProgress{HintsShown: -1}

	hints := []struct {
		id   string
		want []string
	}{
		{"e1", []string{"h1"}},
		{"e2", nil},
		{"e3", []string{"a"}},
		{"e4", nil},
		{"nope", nil},
	}
	for _, h := range hints {
		got := VisibleHints(c, p, h.id)
		if len(got) != len(h.want) || (len(got) > 0 && got[0] != h.want[0]) {
			t.Errorf("VisibleHints(%s) = %v, want %v", h.id, got, h.want)
		}
	}
	// the visible slice cannot be used to reach the hidden hints
	if got := VisibleHints(c, p, "e1"); cap(got) != 1 {
		t.Errorf("cap of visible hints = %d, want 1", cap(got))
	}

	left := []struct {
		id      string
		left    int
		limited bool
	}{
		{"e1", 2, true},
		{"e2", 0, false},
		{"e3", 0, true},
		{"nope", 0, false},
	}
	for _, l := range left {
		got, limited := AttemptsLeft(c, p, l.id)
		if got != l.left || limited != l.limited {
			t.Errorf("AttemptsLeft(%s) = %d, %v; want %d, %v", l.id, got, limited, l.left, l.limited)
		}
	}
}

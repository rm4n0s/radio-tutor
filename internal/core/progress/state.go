package progress

import (
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
)

// derivedState is the stored state an exercise has when its entry is looked at
// alone: passed when it is, needs_review when it is not passed and its
// attempts have run out (SPEC §7.6), otherwise empty. max_attempts 0 means no
// limit.
func derivedState(ex *cassette.Exercise, ep ExerciseProgress) ItemState {
	switch {
	case ep.State == StatePassed:
		return StatePassed
	case ex != nil && ex.MaxAttempts > 0 && ep.Attempts >= ex.MaxAttempts:
		return StateNeedsReview
	}
	return ""
}

// States computes the state of every item of the cassette by the unlock rule
// of SPEC §7.3 (D17), keyed by item id. An item is unlocked when free
// navigation is on, or when every item before it is done: a lesson is done when
// it is completed or skip lessons is on, an exercise when it is passed. The
// exercises of a worksheet do not wait for each other (they are all available
// at once), so for an exercise only the items before its worksheet count. An
// exercise in needs_review is not done, so it blocks what follows the
// worksheet until a parent decides. A completed lesson, a passed exercise and
// an exercise in needs_review show their own state even when the rule would
// lock them.
func States(c *cassette.Cassette, p *Progress) map[string]ItemState {
	states := make(map[string]ItemState, len(c.Manifest.Items))
	open := true      // every item before the current one is done
	worksheet := true // every item before the current worksheet is done
	for i, it := range c.Manifest.Items {
		if it.Type == cassette.ItemLesson || i == 0 || c.Manifest.Items[i-1].Type != cassette.ItemExercise {
			worksheet = open
		}
		unlocked := p.Settings.FreeNavigation
		switch it.Type {
		case cassette.ItemLesson:
			unlocked = unlocked || open
			completed := p.Lessons[it.ID].Completed()
			switch {
			case completed:
				states[it.ID] = StateCompleted
			case unlocked:
				states[it.ID] = StateAvailable
			default:
				states[it.ID] = StateLocked
			}
			open = open && (completed || p.Settings.SkipLessons)
		case cassette.ItemExercise:
			unlocked = unlocked || worksheet
			ep := p.Exercises[it.ID]
			ex := c.Exercises[it.ID]
			stored := derivedState(ex, ep)
			switch {
			case stored != "":
				states[it.ID] = stored
			case unlocked:
				states[it.ID] = StateAvailable
			default:
				states[it.ID] = StateLocked
			}
			open = open && stored == StatePassed
		}
	}
	return states
}

// Continue returns the item to open when the child opens the cassette (SPEC
// §7.4): the last item opened, if the cassette still has it and it is not
// locked; otherwise the first item that is available; otherwise (everything is
// done) the first item. The second result is false only for a cassette without
// items.
func Continue(c *cassette.Cassette, p *Progress) (cassette.Item, bool) {
	items := c.Manifest.Items
	if len(items) == 0 {
		return cassette.Item{}, false
	}
	states := States(c, p)
	if p.LastItem != "" {
		for _, it := range items {
			if it.ID == p.LastItem && states[it.ID] != StateLocked {
				return it, true
			}
		}
	}
	for _, it := range items {
		if states[it.ID] == StateAvailable {
			return it, true
		}
	}
	return items[0], true
}

// VisibleHints returns the hints of the exercise the child may see: the first
// HintsShown of them. It returns nil for an unknown exercise.
func VisibleHints(c *cassette.Cassette, p *Progress, exerciseID string) []string {
	ex := c.Exercises[exerciseID]
	if ex == nil {
		return nil
	}
	n := min(max(p.Exercises[exerciseID].HintsShown, 0), len(ex.Hints))
	return ex.Hints[:n:n]
}

// AttemptsLeft returns how many counted attempts the exercise still allows and
// whether it has a limit; left is 0 when limited is false. An unknown exercise
// has no limit.
func AttemptsLeft(c *cassette.Cassette, p *Progress, exerciseID string) (left int, limited bool) {
	ex := c.Exercises[exerciseID]
	if ex == nil || ex.MaxAttempts <= 0 {
		return 0, false
	}
	return max(ex.MaxAttempts-p.Exercises[exerciseID].Attempts, 0), true
}

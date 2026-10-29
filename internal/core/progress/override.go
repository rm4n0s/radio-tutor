package progress

import (
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
)

// ApplyOverride performs a parent's action (SPEC §7.7) and appends it to
// p.Overrides. The caller fills Action, By, Note and At (the time comes from
// the caller, never from this package) and the target: ExerciseID for the
// exercise actions, LessonID for the lesson actions.
//
//   - ActionMarkPassed: the exercise becomes passed, passed_by "parent".
//   - ActionMarkNotPassed: the pass is removed and the state is computed again,
//     so an exercise whose attempts have run out becomes needs_review.
//   - ActionResetAttempts: attempts, hints and state of the exercise are
//     cleared.
//   - ActionMarkCompleted and ActionMarkNotCompleted: complete a lesson or
//     remove its completion (its plays are kept).
//
// ActionResetAll is not accepted here; use ResetAll. Nothing is changed when
// the call fails, with the tags OverrideActionInvalid, OverrideExerciseUnknown
// or OverrideLessonUnknown.
func ApplyOverride(c *cassette.Cassette, p *Progress, o Override) error {
	o.At = stamp(o.At)
	p.normalize()
	switch o.Action {
	case ActionMarkPassed, ActionMarkNotPassed, ActionResetAttempts:
		ex := c.Exercises[o.ExerciseID]
		if ex == nil {
			return errors.New("OverrideExerciseUnknown", "the cassette has no such exercise",
				"exercise_id", o.ExerciseID, "action", string(o.Action))
		}
		o.LessonID = ""
		ep := p.Exercises[o.ExerciseID]
		switch o.Action {
		case ActionMarkPassed:
			ep.State = StatePassed
			ep.PassedBy = PassedByParent
			ep.PassedAt = o.At
			p.Exercises[o.ExerciseID] = ep
		case ActionMarkNotPassed:
			ep.PassedBy = ""
			ep.PassedAt = time.Time{}
			ep.State = "" // so that derivedState looks at the attempts only
			ep.State = derivedState(ex, ep)
			p.Exercises[o.ExerciseID] = ep
		default: // ActionResetAttempts
			delete(p.Exercises, o.ExerciseID)
		}
	case ActionMarkCompleted, ActionMarkNotCompleted:
		if _, ok := c.Lessons[o.LessonID]; !ok {
			return errors.New("OverrideLessonUnknown", "the cassette has no such lesson",
				"lesson_id", o.LessonID, "action", string(o.Action))
		}
		o.ExerciseID = ""
		lp := p.Lessons[o.LessonID]
		if o.Action == ActionMarkCompleted {
			if !lp.Completed() {
				lp.CompletedAt = o.At
			}
		} else {
			lp.CompletedAt = time.Time{}
		}
		p.Lessons[o.LessonID] = lp
	default:
		return errors.New("OverrideActionInvalid", "this action is not an override of an exercise or a lesson",
			"action", string(o.Action))
	}
	p.Overrides = append(p.Overrides, o)
	return nil
}

// ResetAll is "Reset all progress" of a student in a cassette (SPEC §7.7): it
// clears every lesson and exercise entry, the last item and the entries set
// aside by Migrate, and appends an ActionResetAll override by the admin. The
// settings and the overrides stay. The submissions stay too, so that their
// photos can still be reviewed, unless dropSubmissions is true (the admin
// chose to delete the photos as well).
func ResetAll(p *Progress, by, note string, now time.Time, dropSubmissions bool) {
	p.normalize()
	p.Lessons = map[string]LessonProgress{}
	p.Exercises = map[string]ExerciseProgress{}
	p.LastItem = ""
	p.Orphaned = nil
	if dropSubmissions {
		p.Submissions = []Submission{}
	}
	p.Overrides = append(p.Overrides, Override{Action: ActionResetAll, By: by, At: stamp(now), Note: note})
}

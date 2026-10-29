package progress

import (
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
)

// IgnoredReason says why ApplyResults changed nothing for a result.
type IgnoredReason string

// The reasons a result is ignored.
const (
	// NotIgnored is the zero value: the result was applied.
	NotIgnored IgnoredReason = ""
	// IgnoredUnknownExercise is a result for an id that is not an exercise of
	// the cassette (it may have been removed by an upgrade while grading).
	IgnoredUnknownExercise IgnoredReason = "unknown_exercise"
	// IgnoredAlreadyPassed is a result for an exercise that is already passed
	// (SPEC §7.6): it is recorded in the submission but changes nothing.
	IgnoredAlreadyPassed IgnoredReason = "already_passed"
	// IgnoredNeedsReview is a result for an exercise waiting for a parent
	// (D16): the parent decides, so the result changes nothing.
	IgnoredNeedsReview IgnoredReason = "needs_review"
)

// ExerciseChange is what ApplyResults did for one result.
type ExerciseChange struct {
	// ExerciseID is the exercise.
	ExerciseID string
	// Verdict is the result's verdict.
	Verdict Verdict
	// Ignored is why nothing was changed; NotIgnored when the result applied.
	Ignored IgnoredReason
	// CountedAttempt is true when the result used an attempt.
	CountedAttempt bool
	// Passed is true when the exercise became passed with this result.
	Passed bool
	// HintShown is the 1-based number of the hint this result unlocked, 0 if
	// none.
	HintShown int
	// NeedsReview is true when the exercise became needs_review with this
	// result.
	NeedsReview bool
}

// Changes is what ApplyResults did, for the UI to show.
type Changes struct {
	// Exercises has one entry per result, in the order of the results.
	Exercises []ExerciseChange
	// NewlyPassed are the ids of the exercises that became passed.
	NewlyPassed []string
	// HintsUnlocked are the ids of the exercises that got a new hint.
	HintsUnlocked []string
	// NeedsReview are the ids of the exercises that became needs_review.
	NeedsReview []string
}

// ApplyResults applies the grading results of one submission to p (SPEC §7.6).
// For each result of an exercise that is neither passed nor in needs_review:
//   - the verdict is stored as last_verdict;
//   - correct, partial and incorrect count as an attempt; unreadable and
//     not_found do not (D15);
//   - correct passes the exercise (passed_by "ai", passed_at now);
//   - a counted attempt that did not pass shows the next hint, if one is left;
//   - when the attempts reach the exercise's max_attempts without a pass, the
//     exercise becomes needs_review (D16).
//
// Results for an exercise that is already passed or in needs_review, or that
// the cassette does not have, change nothing (see IgnoredReason). The
// CountedAttempt field of each element of results is set in place, so the
// caller can store the slice in the submission. Nothing is changed when a
// verdict is invalid: the call fails with the tag ResultVerdictInvalid.
func ApplyResults(c *cassette.Cassette, p *Progress, results []Result, now time.Time) (Changes, error) {
	for i, r := range results {
		if !r.Verdict.Valid() {
			return Changes{}, errors.New("ResultVerdictInvalid", "a grading result has an unknown verdict",
				"index", i, "exercise_id", r.ExerciseID, "verdict", string(r.Verdict))
		}
	}
	p.normalize()
	now = stamp(now)
	changes := Changes{Exercises: make([]ExerciseChange, 0, len(results))}
	for i := range results {
		r := &results[i]
		r.CountedAttempt = false
		ch := ExerciseChange{ExerciseID: r.ExerciseID, Verdict: r.Verdict}
		ex := c.Exercises[r.ExerciseID]
		ep := p.Exercises[r.ExerciseID]
		switch {
		case ex == nil:
			ch.Ignored = IgnoredUnknownExercise
		case derivedState(ex, ep) == StatePassed:
			ch.Ignored = IgnoredAlreadyPassed
		case derivedState(ex, ep) == StateNeedsReview:
			ch.Ignored = IgnoredNeedsReview
		default:
			applyOne(ex, &ep, r, &ch, now)
			p.Exercises[r.ExerciseID] = ep
		}
		changes.Exercises = append(changes.Exercises, ch)
		if ch.Passed {
			changes.NewlyPassed = append(changes.NewlyPassed, ch.ExerciseID)
		}
		if ch.HintShown > 0 {
			changes.HintsUnlocked = append(changes.HintsUnlocked, ch.ExerciseID)
		}
		if ch.NeedsReview {
			changes.NeedsReview = append(changes.NeedsReview, ch.ExerciseID)
		}
	}
	return changes, nil
}

// applyOne applies one result to an exercise that is open for attempts.
func applyOne(ex *cassette.Exercise, ep *ExerciseProgress, r *Result, ch *ExerciseChange, now time.Time) {
	ep.LastVerdict = r.Verdict
	if !r.Verdict.CountsAsAttempt() {
		return
	}
	ep.Attempts++
	r.CountedAttempt = true
	ch.CountedAttempt = true
	if r.Verdict == VerdictCorrect {
		ep.State = StatePassed
		ep.PassedBy = PassedByAI
		ep.PassedAt = now
		ch.Passed = true
		return
	}
	if ep.HintsShown < len(ex.Hints) {
		ep.HintsShown++
		ch.HintShown = ep.HintsShown
	}
	if derivedState(ex, *ep) == StateNeedsReview {
		ep.State = StateNeedsReview
		ch.NeedsReview = true
	}
}

// PlayLesson counts one more play of the lesson. It fails with the tag
// PlayLessonUnknown when the cassette has no such lesson.
func PlayLesson(c *cassette.Cassette, p *Progress, lessonID string) error {
	if _, ok := c.Lessons[lessonID]; !ok {
		return errors.New("PlayLessonUnknown", "the cassette has no such lesson", "lesson_id", lessonID)
	}
	p.normalize()
	lp := p.Lessons[lessonID]
	lp.Plays++
	p.Lessons[lessonID] = lp
	return nil
}

// CompleteLesson marks the lesson completed at now. A lesson that is already
// completed keeps its first completion time. It fails with the tag
// CompleteLessonUnknown when the cassette has no such lesson.
func CompleteLesson(c *cassette.Cassette, p *Progress, lessonID string, now time.Time) error {
	if _, ok := c.Lessons[lessonID]; !ok {
		return errors.New("CompleteLessonUnknown", "the cassette has no such lesson", "lesson_id", lessonID)
	}
	p.normalize()
	lp := p.Lessons[lessonID]
	if !lp.Completed() {
		lp.CompletedAt = stamp(now)
	}
	p.Lessons[lessonID] = lp
	return nil
}

// SetLastItem records the item the child opened last. It fails with the tag
// LastItemUnknown when the cassette has no such item.
func SetLastItem(c *cassette.Cassette, p *Progress, itemID string) error {
	for _, it := range c.Manifest.Items {
		if it.ID == itemID {
			p.LastItem = itemID
			return nil
		}
	}
	return errors.New("LastItemUnknown", "the cassette has no such item", "item_id", itemID)
}

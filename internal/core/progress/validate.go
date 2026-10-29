package progress

import (
	"github.com/rm4n0s/errors"
)

// Validate checks a save file as read from disk or about to be written: the
// format, that it belongs to the cassette, and that every state, verdict,
// status, action and count is one the package can work with. It fails with the
// tags ProgressFormatUnsupported, ProgressCassetteMismatch,
// ProgressLessonCountInvalid, ProgressExerciseStateInvalid,
// ProgressExerciseCountInvalid, ProgressPassedByInvalid,
// ProgressExerciseVerdictInvalid, ProgressSubmissionStatusInvalid,
// ProgressResultVerdictInvalid and ProgressOverrideActionInvalid. It does not
// look at the cassette's contents; Migrate deals with ids that changed.
func (p *Progress) Validate(cassetteID string) error {
	if p.Format != FormatVersion {
		return errors.New("ProgressFormatUnsupported", "the save file has an unsupported format", "format", p.Format)
	}
	if p.CassetteID != cassetteID {
		return errors.New("ProgressCassetteMismatch", "the save file belongs to another cassette",
			"cassette_id", cassetteID, "file_cassette_id", p.CassetteID)
	}
	for id, lp := range p.Lessons {
		if lp.Plays < 0 {
			return errors.New("ProgressLessonCountInvalid", "a lesson has a negative number of plays", "lesson_id", id)
		}
	}
	for id, ep := range p.Exercises {
		switch {
		case ep.State != "" && ep.State != StatePassed && ep.State != StateNeedsReview:
			return errors.New("ProgressExerciseStateInvalid", "an exercise has a state that is neither passed nor needs_review",
				"exercise_id", id, "state", string(ep.State))
		case ep.Attempts < 0 || ep.HintsShown < 0:
			return errors.New("ProgressExerciseCountInvalid", "an exercise has a negative count", "exercise_id", id)
		case ep.PassedBy != "" && ep.PassedBy != PassedByAI && ep.PassedBy != PassedByParent:
			return errors.New("ProgressPassedByInvalid", "an exercise was passed by someone unknown",
				"exercise_id", id, "passed_by", string(ep.PassedBy))
		case ep.LastVerdict != "" && !ep.LastVerdict.Valid():
			return errors.New("ProgressExerciseVerdictInvalid", "an exercise has an unknown last verdict",
				"exercise_id", id, "verdict", string(ep.LastVerdict))
		}
	}
	for _, s := range p.Submissions {
		if !s.Status.Valid() {
			return errors.New("ProgressSubmissionStatusInvalid", "a submission has an unknown status",
				"submission_id", s.ID, "status", string(s.Status))
		}
		for _, r := range s.Results {
			if !r.Verdict.Valid() {
				return errors.New("ProgressResultVerdictInvalid", "a result has an unknown verdict",
					"submission_id", s.ID, "exercise_id", r.ExerciseID, "verdict", string(r.Verdict))
			}
		}
	}
	for i, o := range p.Overrides {
		switch o.Action {
		case ActionMarkPassed, ActionMarkNotPassed, ActionResetAttempts,
			ActionMarkCompleted, ActionMarkNotCompleted, ActionResetAll:
		default:
			return errors.New("ProgressOverrideActionInvalid", "an override has an unknown action",
				"index", i, "action", string(o.Action))
		}
	}
	return nil
}

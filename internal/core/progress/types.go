// Package progress keeps what each student has done in each cassette (SPEC §7):
// the save file and its types, the unlock rule, attempts and hints, parent
// overrides, the merge when a newer version of a cassette is installed, and
// the Store that reads and writes the save files.
//
// Almost everything here is a pure function over a *Progress and the
// *cassette.Cassette it belongs to: States, Continue, ApplyResults,
// ApplyOverride, CompleteLesson, ResetAll and Migrate. Time is always a
// parameter. The functions that change a Progress change it in place and leave
// saving to the caller, who uses Store.Update (or Load and Save).
package progress

import (
	"time"

	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
)

// FormatVersion is the "format" of the save file, the only one this package
// reads.
const FormatVersion = 1

// ItemState is the state of a lesson or an exercise (SPEC §7.2). A lesson is
// StateLocked, StateAvailable or StateCompleted; an exercise is StateLocked,
// StateAvailable, StatePassed or StateNeedsReview. In the save file an
// exercise's stored state is only StatePassed or StateNeedsReview (or absent).
type ItemState string

// The item states.
const (
	// StateLocked is an item that the unlock rule does not open yet.
	StateLocked ItemState = "locked"
	// StateAvailable is an item that can be opened or solved.
	StateAvailable ItemState = "available"
	// StateCompleted is a lesson the child has finished.
	StateCompleted ItemState = "completed"
	// StatePassed is an exercise that was solved correctly or marked passed by
	// a parent.
	StatePassed ItemState = "passed"
	// StateNeedsReview is an exercise whose attempts ran out; a parent decides.
	StateNeedsReview ItemState = "needs_review"
)

// Verdict is the grader's judgement of one exercise (SPEC §11.4). The grading
// package has its own reply types; the submission service maps between them.
type Verdict string

// The verdicts. Only VerdictCorrect passes an exercise (D15).
const (
	// VerdictCorrect passes the exercise.
	VerdictCorrect Verdict = "correct"
	// VerdictPartial uses an attempt.
	VerdictPartial Verdict = "partial"
	// VerdictIncorrect uses an attempt.
	VerdictIncorrect Verdict = "incorrect"
	// VerdictUnreadable means the photo could not be read; no attempt is used.
	VerdictUnreadable Verdict = "unreadable"
	// VerdictNotFound means the solution is not on the page; no attempt is
	// used.
	VerdictNotFound Verdict = "not_found"
)

// Valid reports whether v is one of the five verdicts.
func (v Verdict) Valid() bool {
	switch v {
	case VerdictCorrect, VerdictPartial, VerdictIncorrect, VerdictUnreadable, VerdictNotFound:
		return true
	}
	return false
}

// CountsAsAttempt reports whether a result with this verdict uses an attempt
// of its exercise (SPEC §7.6): correct, partial and incorrect do.
func (v Verdict) CountsAsAttempt() bool {
	return v == VerdictCorrect || v == VerdictPartial || v == VerdictIncorrect
}

// PassedBy says who passed an exercise.
type PassedBy string

// The two ways to pass.
const (
	// PassedByAI is a pass from a correct verdict.
	PassedByAI PassedBy = "ai"
	// PassedByParent is a pass from the admin's "Mark passed".
	PassedByParent PassedBy = "parent"
)

// SubmissionStatus is where a submission is in its life (SPEC §7.4).
type SubmissionStatus string

// The submission statuses.
const (
	// SubmissionWaitingPhotos is a submission whose page was opened on a phone
	// and has no photos yet.
	SubmissionWaitingPhotos SubmissionStatus = "waiting_photos"
	// SubmissionGrading is a submission being graded.
	SubmissionGrading SubmissionStatus = "grading"
	// SubmissionGraded is a submission with results.
	SubmissionGraded SubmissionStatus = "graded"
	// SubmissionFailed is a submission that could not be graded; Error says why.
	SubmissionFailed SubmissionStatus = "failed"
	// SubmissionCancelled is a submission the child or the app cancelled.
	SubmissionCancelled SubmissionStatus = "cancelled"
)

// Valid reports whether s is one of the five statuses.
func (s SubmissionStatus) Valid() bool {
	switch s {
	case SubmissionWaitingPhotos, SubmissionGrading, SubmissionGraded, SubmissionFailed, SubmissionCancelled:
		return true
	}
	return false
}

// Action is what an Override did (SPEC §7.7).
type Action string

// The override actions.
const (
	// ActionMarkPassed passes an exercise (PassedByParent).
	ActionMarkPassed Action = "mark_passed"
	// ActionMarkNotPassed removes an exercise's pass.
	ActionMarkNotPassed Action = "mark_not_passed"
	// ActionResetAttempts sets an exercise's attempts and hints back to 0.
	ActionResetAttempts Action = "reset_attempts"
	// ActionMarkCompleted completes a lesson.
	ActionMarkCompleted Action = "mark_completed"
	// ActionMarkNotCompleted makes a lesson not completed.
	ActionMarkNotCompleted Action = "mark_not_completed"
	// ActionResetAll is "Reset all progress" (SPEC §7.7); it has no target.
	ActionResetAll Action = "reset_all"
)

// Settings are the per-student, per-cassette switches an admin sets (D17).
type Settings struct {
	// SkipLessons lets the child go on without completing the lessons.
	SkipLessons bool `json:"skip_lessons"`
	// FreeNavigation opens every item.
	FreeNavigation bool `json:"free_navigation"`
}

// LessonProgress is what is stored for one lesson.
type LessonProgress struct {
	// CompletedAt is when the lesson was completed; the zero time means it is
	// not completed.
	CompletedAt time.Time `json:"completed_at,omitzero"`
	// Plays is how many times the child started the lesson.
	Plays int `json:"plays"`
}

// Completed reports whether the lesson is completed.
func (l LessonProgress) Completed() bool { return !l.CompletedAt.IsZero() }

// ExerciseProgress is what is stored for one exercise.
type ExerciseProgress struct {
	// State is StatePassed, StateNeedsReview or empty.
	State ItemState `json:"state,omitempty"`
	// Attempts is how many counted attempts the child has used.
	Attempts int `json:"attempts"`
	// HintsShown is how many of the exercise's hints the child may see.
	HintsShown int `json:"hints_shown"`
	// PassedBy is set when State is StatePassed.
	PassedBy PassedBy `json:"passed_by,omitempty"`
	// PassedAt is set when State is StatePassed.
	PassedAt time.Time `json:"passed_at,omitzero"`
	// LastVerdict is the verdict of the latest result applied.
	LastVerdict Verdict `json:"last_verdict,omitempty"`
}

// Result is the grading result of one exercise inside a submission (SPEC §7.4).
type Result struct {
	// ExerciseID is the exercise the result is about.
	ExerciseID string `json:"exercise_id"`
	// Verdict is the grader's verdict.
	Verdict Verdict `json:"verdict"`
	// Transcription is what the grader read of the child's solution.
	Transcription string `json:"transcription"`
	// FeedbackForChild is the message for the child.
	FeedbackForChild string `json:"feedback_for_child"`
	// NotesForParent is for the admin only.
	NotesForParent string `json:"notes_for_parent"`
	// CountedAttempt is set by ApplyResults: true when the result used an
	// attempt.
	CountedAttempt bool `json:"counted_attempt"`
}

// Submission is one set of photos sent for some exercises of a worksheet and
// what came of grading them. The submission service (SPEC §10) fills it.
type Submission struct {
	// ID is "<yyyymmddThhmmss>-<4 hex>".
	ID string `json:"id"`
	// CreatedAt is when the submission was opened.
	CreatedAt time.Time `json:"created_at"`
	// ExerciseIDs are the exercises the child selected.
	ExerciseIDs []string `json:"exercise_ids"`
	// Photos are the photo file names, "1.jpg", "2.jpg", ...
	Photos []string `json:"photos"`
	// Status is where the submission is.
	Status SubmissionStatus `json:"status"`
	// Provider is the name of the AI provider that graded.
	Provider string `json:"provider"`
	// Model is the model that graded.
	Model string `json:"model"`
	// PageNotes are the grader's remarks on the page as a whole.
	PageNotes string `json:"page_notes"`
	// Results are the grading results.
	Results []Result `json:"results"`
	// Error says why a failed submission failed.
	Error string `json:"error"`
}

// Override is a parent's action on an exercise or a lesson (SPEC §7.7).
type Override struct {
	// ExerciseID is the target of the exercise actions.
	ExerciseID string `json:"exercise_id,omitempty"`
	// LessonID is the target of the lesson actions.
	LessonID string `json:"lesson_id,omitempty"`
	// Action is what was done.
	Action Action `json:"action"`
	// By is the username of the admin.
	By string `json:"by"`
	// At is when it was done.
	At time.Time `json:"at"`
	// Note is the admin's free text.
	Note string `json:"note"`
}

// Orphaned holds the entries of ids that a newer version of the cassette no
// longer has, or has with another type (SPEC §7.5). They are kept and ignored.
type Orphaned struct {
	// Lessons are the lesson entries set aside.
	Lessons map[string]LessonProgress `json:"lessons,omitempty"`
	// Exercises are the exercise entries set aside.
	Exercises map[string]ExerciseProgress `json:"exercises,omitempty"`
}

// Progress is the save file of one student in one cassette (SPEC §7.4).
type Progress struct {
	// Format is FormatVersion.
	Format int `json:"format"`
	// CassetteID is the id of the cassette.
	CassetteID string `json:"cassette_id"`
	// CassetteVersion is the version of the cassette the file was last
	// merged with; 0.0.0 in a file that was never merged (Migrate sets it).
	CassetteVersion cassette.Version `json:"cassette_version"`
	// Settings are the admin's switches.
	Settings Settings `json:"settings"`
	// LastItem is the id of the item the child last opened.
	LastItem string `json:"last_item"`
	// Lessons are the lesson entries by id.
	Lessons map[string]LessonProgress `json:"lessons"`
	// Exercises are the exercise entries by id.
	Exercises map[string]ExerciseProgress `json:"exercises"`
	// Submissions are all the submissions, oldest first.
	Submissions []Submission `json:"submissions"`
	// Overrides are the parent's actions, oldest first.
	Overrides []Override `json:"overrides"`
	// Orphaned is the entries set aside by Migrate.
	Orphaned *Orphaned `json:"orphaned,omitempty"`
}

// New returns an empty Progress for the cassette id.
func New(cassetteID string) *Progress {
	p := &Progress{Format: FormatVersion, CassetteID: cassetteID}
	p.normalize()
	return p
}

// normalize sets Format and replaces nil maps and slices with empty ones, so
// the file always has "lessons": {} and "submissions": [].
func (p *Progress) normalize() {
	p.Format = FormatVersion
	if p.Lessons == nil {
		p.Lessons = map[string]LessonProgress{}
	}
	if p.Exercises == nil {
		p.Exercises = map[string]ExerciseProgress{}
	}
	if p.Submissions == nil {
		p.Submissions = []Submission{}
	}
	if p.Overrides == nil {
		p.Overrides = []Override{}
	}
	for i := range p.Submissions {
		s := &p.Submissions[i]
		if s.ExerciseIDs == nil {
			s.ExerciseIDs = []string{}
		}
		if s.Photos == nil {
			s.Photos = []string{}
		}
		if s.Results == nil {
			s.Results = []Result{}
		}
	}
}

// Submission returns the submission with the id, or nil. The pointer is into
// p.Submissions and is valid until the slice grows.
func (p *Progress) Submission(id string) *Submission {
	for i := range p.Submissions {
		if p.Submissions[i].ID == id {
			return &p.Submissions[i]
		}
	}
	return nil
}

// stamp is a time as stored: UTC, whole seconds.
func stamp(t time.Time) time.Time { return t.UTC().Truncate(time.Second) }

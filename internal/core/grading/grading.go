// Package grading is the core of AI grading (SPEC §10.3 and §11): it prepares
// photos, builds the prompt and the reply schema, parses and validates the
// model's reply, and defines the Provider interface that the OpenAI-compatible
// provider (step 3.2) and the fake provider implement. It makes no network
// request itself.
//
// The app owns the prompt (system_prompt.tmpl); the cassette supplies data
// (the exercises, their reference solutions and grading instructions) that is
// put in the user message as quoted values (SPEC D9). One submission can hold
// the solutions of several exercises on the same page and the model must
// separate them by the labels the child wrote (SPEC D10).
package grading

import (
	"context"
	"time"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
)

// Limits of one grading request (SPEC §9 and §10.1).
const (
	// MaxExercises is the most exercises one submission may hold.
	MaxExercises = 10
	// MaxPhotos is the most photos one submission may hold.
	MaxPhotos = 6
)

// Verdict is the grader's judgement of one exercise (SPEC §11.5, D15).
type Verdict string

// The verdicts of SPEC §11.5.
const (
	// VerdictCorrect passes the exercise.
	VerdictCorrect Verdict = "correct"
	// VerdictPartial is on the right way but incomplete or with a small
	// mistake; it uses an attempt.
	VerdictPartial Verdict = "partial"
	// VerdictIncorrect is a wrong solution; it uses an attempt.
	VerdictIncorrect Verdict = "incorrect"
	// VerdictUnreadable means the solution is on the photos but cannot be
	// read; it does not use an attempt.
	VerdictUnreadable Verdict = "unreadable"
	// VerdictNotFound means the solution is not on the photos; it does not
	// use an attempt.
	VerdictNotFound Verdict = "not_found"
)

// Verdicts lists the five verdicts in the order of the reply schema.
func Verdicts() []Verdict {
	return []Verdict{VerdictCorrect, VerdictPartial, VerdictIncorrect, VerdictUnreadable, VerdictNotFound}
}

// Valid reports whether v is one of the five verdicts.
func (v Verdict) Valid() bool {
	for _, k := range Verdicts() {
		if v == k {
			return true
		}
	}
	return false
}

// Passes reports whether the verdict passes the exercise: only "correct"
// does (SPEC D15).
func (v Verdict) Passes() bool { return v == VerdictCorrect }

// UsesAttempt reports whether the verdict uses one of the exercise's
// attempts: "partial" and "incorrect" do; "unreadable" and "not_found" do not
// (SPEC D15).
func (v Verdict) UsesAttempt() bool { return v == VerdictPartial || v == VerdictIncorrect }

// Request is what the grader is asked: the cassette's data for the selected
// exercises and the child's photos. It carries nothing about the child.
type Request struct {
	// Title is the cassette's title.
	Title string
	// Language is the BCP 47 tag of the cassette's content ("el").
	Language string
	// Ages is [min, max], the child's age range from the manifest.
	Ages [2]int
	// GraderInstructions are the manifest's cassette-wide instructions.
	GraderInstructions string
	// Exercises are the selected exercises, with their labels set. The ids
	// and the labels must each be unique.
	Exercises []cassette.Exercise
	// Photos are the normalized photos (JPEG, see NormalizePhoto) in order.
	Photos [][]byte
}

// NewRequest builds the Request for the exercises of c named by exerciseIDs,
// in that order, with the photos. It does not check the limits; Request.Validate
// does.
func NewRequest(c *cassette.Cassette, exerciseIDs []string, photos [][]byte) (Request, error) {
	req := Request{
		Title:              c.Manifest.Title,
		Language:           c.Manifest.Language,
		GraderInstructions: c.Manifest.GraderInstructions,
		Photos:             photos,
	}
	if len(c.Manifest.Ages) == 2 {
		req.Ages = [2]int{c.Manifest.Ages[0], c.Manifest.Ages[1]}
	}
	for _, id := range exerciseIDs {
		ex, ok := c.Exercises[id]
		if !ok || ex == nil {
			return Request{}, errors.New("GradingExerciseUnknown", "the cassette has no such exercise", "exercise", id)
		}
		req.Exercises = append(req.Exercises, *ex)
	}
	return req, nil
}

// Validate checks the limits of SPEC §9 and §10.1: 1 to MaxExercises
// exercises with unique ids and labels, 1 to MaxPhotos photos.
func (r Request) Validate() error {
	if n := len(r.Exercises); n == 0 || n > MaxExercises {
		return errors.New("GradingExerciseCount", "a request needs 1 to 10 exercises", "count", n, "max", MaxExercises)
	}
	if n := len(r.Photos); n == 0 || n > MaxPhotos {
		return errors.New("GradingPhotoCount", "a request needs 1 to 6 photos", "count", n, "max", MaxPhotos)
	}
	ids := make(map[string]bool, len(r.Exercises))
	labels := make(map[string]bool, len(r.Exercises))
	for _, ex := range r.Exercises {
		if ids[ex.ID] {
			return errors.New("GradingExerciseDuplicate", "an exercise is selected twice", "exercise", ex.ID)
		}
		ids[ex.ID] = true
		if labels[ex.Label] {
			return errors.New("GradingLabelDuplicate", "two selected exercises have the same label", "label", ex.Label)
		}
		labels[ex.Label] = true
	}
	return nil
}

// ExerciseIDs returns the ids of the request's exercises, in order.
func (r Request) ExerciseIDs() []string {
	ids := make([]string, len(r.Exercises))
	for i, ex := range r.Exercises {
		ids[i] = ex.ID
	}
	return ids
}

// ExerciseResult is the grader's answer for one exercise (one entry of the
// reply's "results", SPEC §11.5).
type ExerciseResult struct {
	// ExerciseID is the id of the exercise, as given in the request.
	ExerciseID string `json:"exercise_id"`
	// Verdict is the judgement.
	Verdict Verdict `json:"verdict"`
	// Transcription is what the model read on the paper, as written.
	Transcription string `json:"transcription"`
	// FeedbackForChild is the short, warm message for the child.
	FeedbackForChild string `json:"feedback_for_child"`
	// NotesForParent says what the child wrote and why the verdict.
	NotesForParent string `json:"notes_for_parent"`
}

// Reply is a validated reply of the model: PageNotes and exactly one result
// for each exercise asked, in the order they were asked.
type Reply struct {
	// PageNotes are notes about the photos as a whole.
	PageNotes string `json:"page_notes"`
	// Results has one entry per asked exercise.
	Results []ExerciseResult `json:"results"`
}

// Meta describes one grading call.
type Meta struct {
	// Provider is the provider's Name (set by the Grader).
	Provider string
	// Model is the model that answered, as the provider reports it.
	Model string
	// FinishReason is the provider's finish reason of the last answer
	// ("stop", "length", ...); "length" makes the answer invalid.
	FinishReason string
	// Duration is how long grading took, including the retry (set by the
	// Grader).
	Duration time.Duration
	// Retried is true when the first answer was invalid and the retry of
	// SPEC §11.5 was made (set by the Grader).
	Retried bool
}

// RetryHint tells a provider that the previous answer was invalid, so it can
// repeat the request with the answer and a correction appended (SPEC §11.5).
type RetryHint struct {
	// PreviousAnswer is the model's invalid answer, as received.
	PreviousAnswer string
	// Problem says what was wrong with it.
	Problem string
}

// Message returns the user message to append after the previous answer:
// "Your previous answer was not valid JSON of the required schema: <problem>.
// Answer again with only the JSON object."
func (h RetryHint) Message() string {
	return "Your previous answer was not valid JSON of the required schema: " + h.Problem +
		". Answer again with only the JSON object."
}

// Provider asks a model to grade a Request. The OpenAI-compatible provider
// and the fake provider implement it.
type Provider interface {
	// Name is the name shown to the parent.
	Name() string
	// Grade sends the request (prompt from BuildPrompt, the photos) and
	// returns the model's raw answer text and what is known about the call
	// (Model, FinishReason). With prior non-nil it is the single retry: the
	// provider sends the same request, then the assistant message
	// prior.PreviousAnswer, then the user message prior.Message(). Errors
	// are *errors.Error values the provider created; they are not retried.
	Grade(ctx context.Context, req Request, prior *RetryHint) (raw string, meta Meta, err error)
}

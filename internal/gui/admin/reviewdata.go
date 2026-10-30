package admin

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// waitingAttempt is one attempt of a waiting exercise that did not pass: the
// result of the grader and the submission (with its photos) it came from.
type waitingAttempt struct {
	// SubmissionID is the submission the result belongs to.
	SubmissionID string
	// At is when the submission was opened.
	At time.Time
	// Photos are the photo file names of the submission; empty when the
	// photos were deleted.
	Photos []string
	// Result is the grader's result for the exercise.
	Result progress.Result
}

// waitingItem is an exercise in needs_review with what the parent needs to
// decide about it (SPEC §12.3).
type waitingItem struct {
	// Username and DisplayName are the child.
	Username, DisplayName string
	// CassetteID and CassetteTitle are the cassette.
	CassetteID, CassetteTitle string
	// Cassette is the installed cassette, which the overrides need.
	Cassette *cassette.Cassette
	// ExerciseID, Label, Title, Prompt and Reference are the exercise's id, its
	// label, its title, its prompt text and its reference solution.
	ExerciseID, Label, Title, Prompt, Reference string
	// Attempts are the attempts that did not pass since the last reset, newest
	// first.
	Attempts []waitingAttempt
	// Since is the date of the newest attempt; the zero time when there is none.
	Since time.Time
}

// ID identifies the item among all the items of the queue; the keys of its
// widgets are made from it.
func (w waitingItem) ID() string { return QueueID(w.Username, w.CassetteID, w.ExerciseID) }

// QueueID is the identity of a waiting exercise in the review queue: the
// username, the cassette id and the exercise id, joined. The keys of the queue
// are the Key prefixes of the queue followed by it.
func QueueID(username, cassetteID, exerciseID string) string {
	return username + "/" + cassetteID + "/" + exerciseID
}

// failedAttempts are the results for the exercise that did not pass, newest
// first, since the latest Reset attempts of the exercise or Reset all progress
// (older attempts were given up by the parent).
func failedAttempts(prog *progress.Progress, exerciseID string) []waitingAttempt {
	var cutoff time.Time
	for _, o := range prog.Overrides {
		if (o.Action == progress.ActionResetAttempts && o.ExerciseID == exerciseID) || o.Action == progress.ActionResetAll {
			if o.At.After(cutoff) {
				cutoff = o.At
			}
		}
	}
	var out []waitingAttempt
	for _, sub := range prog.Submissions {
		if !cutoff.IsZero() && !sub.CreatedAt.After(cutoff) {
			continue
		}
		for _, r := range sub.Results {
			if r.ExerciseID == exerciseID && r.Verdict != progress.VerdictCorrect {
				out = append(out, waitingAttempt{SubmissionID: sub.ID, At: sub.CreatedAt, Photos: sub.Photos, Result: r})
			}
		}
	}
	slices.SortStableFunc(out, func(a, b waitingAttempt) int { return b.At.Compare(a.At) })
	return out
}

// scanQueue lists the exercises that wait for a parent, over every profile and
// every installed cassette that can be read, oldest first (by the date of the
// newest attempt that did not pass; the order of the profiles and of the
// cassette decides between equals). It is the list behind the count of the home
// (scanWaiting) and fails with the tags of progress.Store.Load and
// archive.Store.List.
func scanQueue(ctx context.Context, env *app.Env) ([]waitingItem, error) {
	var out []waitingItem
	err := walkWaiting(ctx, env, nil, func(p profile.Profile, c *cassette.Cassette, prog *progress.Progress, states map[string]progress.ItemState) {
		for _, it := range c.Manifest.Items {
			ex := c.Exercises[it.ID]
			if it.Type != cassette.ItemExercise || ex == nil || states[it.ID] != progress.StateNeedsReview {
				continue
			}
			w := waitingItem{
				Username: p.Username, DisplayName: p.DisplayName,
				CassetteID: c.Manifest.ID, CassetteTitle: c.Manifest.Title, Cassette: c,
				ExerciseID: it.ID, Label: ex.Label, Title: ex.Title, Prompt: ex.Prompt, Reference: ex.ReferenceSolution,
				Attempts: failedAttempts(prog, it.ID),
			}
			if len(w.Attempts) > 0 {
				w.Since = w.Attempts[0].At
			}
			out = append(out, w)
		}
	})
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(out, func(a, b waitingItem) int { return a.Since.Compare(b.Since) })
	return out, nil
}

// listLine is the one line that names a waiting exercise in the notice at
// login.
func (w waitingItem) listLine(scope *app.Scope) string {
	return strings.Join([]string{
		scope.T("admin.review.child_cassette", "name", w.DisplayName, "cassette", w.CassetteTitle),
		scope.T("admin.progress.exercise_title", "label", w.Label, "title", w.Title),
	}, " - ")
}

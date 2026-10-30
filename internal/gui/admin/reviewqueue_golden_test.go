package admin

import (
	"testing"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/glutter/ui"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

// TestReviewGoldens takes the pictures of the notice at login and of the review
// screen, in both languages, with a second child whose exercise has attempts
// whose photos were deleted.
func TestReviewGoldens(t *testing.T) {
	setup := func(t *testing.T, env *app.Env) {
		richSetup(t, env)
		// mum (who also plays) has an exercise of the other cassette that waits;
		// its photos were deleted.
		setProgress(t, env, "mum", otherID, func(p *progress.Progress) {
			p.Exercises[exSum] = progress.ExerciseProgress{State: progress.StateNeedsReview, Attempts: 5}
			p.Submissions = []progress.Submission{
				graded("20261029T150000-0001", at(15, 0, 0).AddDate(0, 0, -1), "Home Ollama", []string{},
					progress.Result{
						ExerciseID: exSum, Verdict: progress.VerdictUnreadable, Transcription: "",
						FeedbackForChild: "I could not read it. Can you write it bigger?", NotesForParent: "The photo is blurred.", CountedAttempt: false,
					}),
				graded("20261029T151500-0002", at(15, 15, 0).AddDate(0, 0, -1), "Home Ollama", []string{},
					progress.Result{
						ExerciseID: exSum, Verdict: progress.VerdictPartial, Transcription: "3/8",
						FeedbackForChild: "Almost. Do the bottom numbers change?", NotesForParent: "Added the bottoms.", CountedAttempt: true,
					}),
			}
		})
	}
	tests := []struct {
		name   string
		locale string
		size   ui.Size
		open   func(t *testing.T, a *testutil.App)
	}{
		{"review_notice_en", "en_US.UTF-8", ui.Size{Width: 800, Height: 700}, nil},
		{"review_notice_el", "el_GR.UTF-8", ui.Size{Width: 800, Height: 700}, nil},
		{"review_queue_en", "en_US.UTF-8", ui.Size{Width: 800, Height: 2300}, openQueue},
		{"review_queue_el", "el_GR.UTF-8", ui.Size{Width: 800, Height: 2300}, openQueue},
		{"review_confirm_en", "en_US.UTF-8", ui.Size{Width: 800, Height: 1000}, func(t *testing.T, a *testutil.App) {
			openQueue(t, a)
			a.TapKey(KeyQueueMorePrefix + QueueID("mum", otherID, exSum))
			waitKey(t, a, KeyDialogOK)
		}},
		{"review_empty_en", "en_US.UTF-8", ui.Size{Width: 800, Height: 600}, func(t *testing.T, a *testutil.App) {
			openQueue(t, a)
			a.TapKey(KeyQueuePassPrefix + sumID)
			waitKey(t, a, KeyQueuePassPrefix+QueueID("mum", otherID, exSum))
			a.TapKey(KeyQueuePassPrefix + QueueID("mum", otherID, exSum))
			waitKey(t, a, KeyQueueEmpty)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The window is tall while the test taps its way to the screen, then it
			// gets the size of the golden.
			a := pumpNotice(t, config{locale: tt.locale, setup: setup, size: m.Size{Width: 800, Height: 2400}})
			if tt.open != nil {
				tt.open(t, a)
			}
			a.SetViewSize(tt.size)
			a.PumpAndSettle()
			a.MatchesGolden(tt.name)
		})
	}
}

// openQueue taps Review now and waits for the queue and its photos.
func openQueue(t *testing.T, a *testutil.App) {
	t.Helper()
	a.TapKey(KeyNoticeNow)
	waitKey(t, a, KeyQueueItemPrefix+sumID)
	settleImages(a)
}

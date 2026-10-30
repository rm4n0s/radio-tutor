package worksheet

import (
	"testing"
	"time"

	"github.com/rm4n0s/errors"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/grading/fake"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/submission"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// goldenSize is the window of the golden images: the width of the smallest
// window and enough height for the whole screen.
var goldenSize = m.Size{Width: app.MinWindowWidth, Height: 1000}

// goldenProgress is a student who tried the second exercise once and was given
// its first hint.
func goldenProgress(p *progress.Progress) {
	lessonDone(p)
	p.Exercises[exName] = progress.ExerciseProgress{Attempts: 1, HintsShown: 1, LastVerdict: progress.VerdictPartial}
}

func TestWorksheetGoldens(t *testing.T) {
	tests := []struct {
		name   string
		locale string
	}{
		{"worksheet_en", "en_US.UTF-8"},
		{"worksheet_el", "el_GR.UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := pump(t, config{locale: tt.locale, size: goldenSize, change: goldenProgress})
			// The pictures are drawn asynchronously.
			h.whileBusy("the picture", func() bool { return h.Count(key(KeyCardPrefix+exShade)) > 0 })
			h.PumpAndSettle()
			h.MatchesGolden(tt.name)
		})
	}
}

func TestWorksheetWithAStateGolden(t *testing.T) {
	h := pump(t, config{size: goldenSize, change: func(p *progress.Progress) {
		lessonDone(p)
		p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StateNeedsReview, Attempts: 3}
		p.Exercises[exName] = progress.ExerciseProgress{State: progress.StatePassed, Attempts: 1, PassedBy: progress.PassedByAI}
	}})
	h.PumpAndSettle()
	h.MatchesGolden("worksheet_states_en")
}

// pumpQR opens the QR dialog with a fixed address: a fixed port and the token
// of the cycle reader.
func pumpQR(t *testing.T, locale string) *harness {
	t.Helper()
	h := pump(t, config{locale: locale, size: goldenSize, port: goldenPort})
	h.check()
	h.PumpAndSettle()
	if got, want := h.url(), "http://127.0.0.1:38737/s/"; len(got) < len(want) || got[:len(want)] != want {
		t.Fatalf("the golden address is %q, want one that starts with %q", got, want)
	}
	return h
}

func TestQRDialogGoldens(t *testing.T) {
	t.Run("qr_en", func(t *testing.T) {
		h := pumpQR(t, "en_US.UTF-8")
		h.MatchesGolden("qr_en")
	})
	t.Run("qr_el", func(t *testing.T) {
		h := pumpQR(t, "el_GR.UTF-8")
		h.MatchesGolden("qr_el")
	})
	t.Run("qr_firewall_en", func(t *testing.T) {
		h := pumpQR(t, "en_US.UTF-8")
		h.clock.Advance(submission.FirewallHintAfter)
		h.whileBusy("the hint", func() bool { return h.HasKey(KeyFirewall) })
		h.PumpAndSettle()
		h.MatchesGolden("qr_firewall_en")
	})
	t.Run("qr_small_window_en", func(t *testing.T) {
		h := pump(t, config{size: m.Size{Width: app.MinWindowWidth, Height: app.MinWindowHeight}, port: goldenPort})
		h.check()
		h.PumpAndSettle()
		h.MatchesGolden("qr_small_window_en")
	})
}

// gradeAndWait sends a photo and waits for the screen with the key.
func gradeAndWait(t *testing.T, h *harness, step fake.Step, wait string) {
	t.Helper()
	h.provider.use(fake.New("fake", step))
	h.check()
	if status := upload(t, h.url()); status != 200 {
		t.Fatalf("upload status %d", status)
	}
	h.waitKey(wait)
	// Let the pop-in of the screen finish.
	time.Sleep(10 * time.Millisecond)
	h.PumpAndSettle()
}

func TestResultsGoldens(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		reply  string
	}{
		{"results_en", "en_US.UTF-8", fake.JSONReply("",
			result(exShade, grading.VerdictCorrect, "Two equal halves, well done!"),
			result(exName, grading.VerdictPartial, "You are close, look at the slices again."))},
		{"results_el", "el_GR.UTF-8", fake.JSONReply("",
			result(exShade, grading.VerdictCorrect, "Δύο ίσα μισά, μπράβο!"),
			result(exName, grading.VerdictPartial, "Είσαι κοντά, κοίτα ξανά τις φέτες."))},
		{"results_mixed_en", "en_US.UTF-8", fake.JSONReply("",
			result(exShade, grading.VerdictIncorrect, "The two parts are not equal."),
			result(exName, grading.VerdictNotFound, "I could not find this one on the page. Is it on the photo?"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := pump(t, config{locale: tt.locale, size: goldenSize, port: goldenPort})
			gradeAndWait(t, h, fake.Reply(tt.reply), KeyResultPrefix+exShade)
			h.MatchesGolden(tt.name)
		})
	}
}

func TestResultNeedsReviewGolden(t *testing.T) {
	h := pump(t, config{size: goldenSize, port: goldenPort, change: func(p *progress.Progress) {
		lessonDone(p)
		// One try is left on the first exercise (it has three), so a wrong
		// answer uses the last one.
		p.Exercises[exShade] = progress.ExerciseProgress{Attempts: 2, HintsShown: 0}
	}})
	gradeAndWait(t, h, fake.Reply(fake.JSONReply("",
		result(exShade, grading.VerdictIncorrect, "Not quite."),
		result(exName, grading.VerdictCorrect, "Yes, one quarter."))), KeyResultPrefix+exShade)
	if !h.HasKey(KeyReviewPrefix + exShade) {
		t.Fatal("the exercise whose tries ran out has no message for a parent")
	}
	h.MatchesGolden("results_review_en")
}

func TestFailureGoldens(t *testing.T) {
	tests := []struct {
		name       string
		locale     string
		noProvider bool
		step       fake.Step
	}{
		{"failure_en", "en_US.UTF-8", false, fake.Fail(errors.New("OpenAINetwork", "cannot be reached"))},
		{"failure_el", "el_GR.UTF-8", false, fake.Fail(errors.New("OpenAINetwork", "cannot be reached"))},
		{"failure_no_provider_en", "en_US.UTF-8", true, fake.Step{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := pump(t, config{locale: tt.locale, size: goldenSize, port: goldenPort, noProvider: tt.noProvider})
			gradeAndWait(t, h, tt.step, KeyFailure)
			h.MatchesGolden(tt.name)
		})
	}
}

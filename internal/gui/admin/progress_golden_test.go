package admin

import (
	"testing"
	"time"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/glutter/ui"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

// settleImages gives the images that were just shown time to load and decode on
// their goroutines.
func settleImages(a *testutil.App) {
	for range 25 {
		time.Sleep(20 * time.Millisecond)
		a.FlushPosted()
		a.PumpAndSettle()
	}
}

// inUTC makes the dates of the goldens the same on every machine.
func inUTC(t *testing.T) {
	t.Helper()
	old := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = old })
}

func TestProgressGoldens(t *testing.T) {
	withGone := func(t *testing.T, env *app.Env) {
		richSetup(t, env)
		setProgress(t, env, "mia", goneID, func(p *progress.Progress) {
			p.Lessons[lessonWhat] = progress.LessonProgress{CompletedAt: at(9, 0, 0)}
		})
	}
	tests := []struct {
		name   string
		locale string
		size   m.Size
		setup  func(t *testing.T, env *app.Env)
		open   func(t *testing.T, a *testutil.App)
	}{
		{"progress_people_en", "en_US.UTF-8", m.Size{Width: 800, Height: 700}, richSetup, func(t *testing.T, a *testutil.App) {
			a.TapKey(KeyProgress)
			waitText(t, a, "1 waiting for review")
		}},
		{"progress_student_en", "en_US.UTF-8", m.Size{Width: 800, Height: 700}, withGone, func(t *testing.T, a *testutil.App) {
			openStudentPage(t, a, "mia")
			waitText(t, a, "Not installed")
		}},
		{"progress_cassette_en", "en_US.UTF-8", m.Size{Width: 800, Height: 2100}, richSetup, func(t *testing.T, a *testutil.App) {
			openCassettePageOf(t, a, "mia", fractionsID)
		}},
		{"progress_cassette_el", "el_GR.UTF-8", m.Size{Width: 800, Height: 2100}, richSetup, func(t *testing.T, a *testutil.App) {
			openCassettePageOf(t, a, "mia", fractionsID)
		}},
		{"progress_cassette_readonly_en", "en_US.UTF-8", m.Size{Width: 800, Height: 900}, withGone, func(t *testing.T, a *testutil.App) {
			openCassettePageOf(t, a, "mia", goneID)
		}},
		{"progress_reset_en", "en_US.UTF-8", m.Size{Width: 800, Height: 1000}, richSetup, func(t *testing.T, a *testutil.App) {
			openCassettePageOf(t, a, "mia", fractionsID)
			a.TapKey(KeyResetAll)
			waitKey(t, a, KeyAlsoPhotos)
		}},
		{"submission_en", "en_US.UTF-8", m.Size{Width: 800, Height: 1700}, richSetup, func(t *testing.T, a *testutil.App) {
			openSubmissionOf(t, a, "mia", fractionsID, subOlder)
			settleImages(a)
		}},
		{"submission_el", "el_GR.UTF-8", m.Size{Width: 800, Height: 1700}, richSetup, func(t *testing.T, a *testutil.App) {
			openSubmissionOf(t, a, "mia", fractionsID, subOlder)
			settleImages(a)
		}},
		{"submission_delete_photos_en", "en_US.UTF-8", m.Size{Width: 800, Height: 1000}, richSetup, func(t *testing.T, a *testutil.App) {
			openSubmissionOf(t, a, "mia", fractionsID, subNewer)
			a.TapKey(KeyDeletePhotos)
			waitKey(t, a, KeyDialogOK)
		}},
		{"photo_viewer_en", "en_US.UTF-8", m.Size{Width: 800, Height: 700}, richSetup, func(t *testing.T, a *testutil.App) {
			openSubmissionOf(t, a, "mia", fractionsID, subOlder)
			a.TapKey(KeyPhotoPrefix + "1")
			waitKey(t, a, KeyPhotoViewer)
			settleImages(a)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inUTC(t)
			// The window is tall while the test taps its way to the screen, then
			// it gets the size of the golden.
			a := pumpProgress(t, config{locale: tt.locale, setup: tt.setup})
			tt.open(t, a)
			a.SetViewSize(ui.Size{Width: tt.size.Width, Height: tt.size.Height})
			a.PumpAndSettle()
			a.MatchesGolden(tt.name)
		})
	}
}

package admin

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/providers"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

// Ids of the fixture cassette's items (internal/core/archive/testdata/valid).
const (
	exName       = "name-the-fraction"
	exSum        = "write-the-sum"
	lessonWhat   = "what-is-a-fraction"
	lessonAdding = "adding-fractions"
	goneID       = "org.example.gone-9"
	subOlder     = "20261030T171955-3f9a"
	subNewer     = "20261030T180512-77c1"
	// progressHeight is tall enough for the whole page of the fixture.
	progressHeight = 3200.0
)

// progressScreens plugs the progress screen into the home.
var progressScreens = Screens{Progress: func() m.Widget { return NewProgress() }}

// at is a fixed time of 30 October 2026.
func at(hour, minute, second int) time.Time {
	return time.Date(2026, 10, 30, hour, minute, second, 0, time.UTC)
}

// pumpProgress is pump with the progress screen, in a window tall enough for the
// pages of the tests.
func pumpProgress(t *testing.T, c config) *testutil.App {
	t.Helper()
	c.screens = progressScreens
	if c.size == (m.Size{}) {
		c.size = m.Size{Width: app.MinWindowWidth, Height: progressHeight}
	}
	return pump(t, c)
}

// openStudentPage opens Progress and the page of the person.
func openStudentPage(t *testing.T, a *testutil.App, user string) {
	t.Helper()
	a.TapKey(KeyProgress)
	waitKey(t, a, KeyProgressStudentPrefix+user)
	a.TapKey(KeyProgressStudentPrefix + user)
	waitKey(t, a, KeyDeleteStudentPhotos)
}

// openCassettePageOf opens the page of the person's progress in the cassette.
func openCassettePageOf(t *testing.T, a *testutil.App, user, id string) {
	t.Helper()
	openStudentPage(t, a, user)
	waitKey(t, a, KeyProgressCassettePrefix+id)
	a.TapKey(KeyProgressCassettePrefix + id)
	waitKey(t, a, KeySkipLessons)
}

// openSubmissionOf opens the submission from the page of the cassette.
func openSubmissionOf(t *testing.T, a *testutil.App, user, id, sub string) {
	t.Helper()
	openCassettePageOf(t, a, user, id)
	a.TapKey(KeySubmissionPrefix + sub)
	waitKey(t, a, KeyPhotoPrefix+"1")
}

// textOf returns the text of the Text widget with the key.
func textOf(t *testing.T, a *testutil.App, k string) string {
	t.Helper()
	txt, ok := a.Widget(key(k)).(*m.Text)
	if !ok {
		t.Fatalf("%s is not a Text", k)
	}
	return txt.PlainText()
}

// loadSaved reads the save file of the user for the cassette.
func loadSaved(t testing.TB, env *app.Env, user, id string) *progress.Progress {
	t.Helper()
	p, err := env.Progress.Load(context.Background(), user, id)
	if err != nil {
		t.Fatalf("loading the save file: %v", err)
	}
	return p
}

// waitSaved waits until the save file satisfies cond.
func waitSaved(t *testing.T, a *testutil.App, user, id, what string, cond func(p *progress.Progress) bool) {
	t.Helper()
	a.WaitFor(t, what, func() bool { return cond(loadSaved(t, a.Env, user, id)) })
}

// writePhoto writes a small JPEG as the photo of a submission and returns its
// path.
func writePhoto(t testing.TB, env *app.Env, user, id, sub, name string) string {
	t.Helper()
	dir := filepath.Join(env.Paths.PhotosDir(), user, id, sub)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 300, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 300; x++ {
			c := color.RGBA{R: 250, G: 248, B: 240, A: 255}
			switch {
			case y%40 == 0: // the ruled lines of the page
				c = color.RGBA{R: 150, G: 180, B: 220, A: 255}
			case (x-150)*(x-150)+(y-140)*(y-140) < 70*70 && (x-150)*(x-150)+(y-140)*(y-140) > 62*62: // a circle
				c = color.RGBA{R: 30, G: 30, B: 90, A: 255}
			case x > 150 && x < 212 && (x-150)*(x-150)+(y-140)*(y-140) < 62*62: // half of it shaded
				c = color.RGBA{R: 90, G: 120, B: 200, A: 255}
			case y > 260 && y < 270 && x > 60 && x < 230: // a line of writing
				c = color.RGBA{R: 40, G: 40, B: 40, A: 255}
			}
			img.Set(x, y, c)
		}
	}
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// graded returns a graded submission with the results for the exercises.
func graded(id string, created time.Time, provider string, photos []string, results ...progress.Result) progress.Submission {
	ids := make([]string, 0, len(results))
	for _, r := range results {
		ids = append(ids, r.ExerciseID)
	}
	return progress.Submission{
		ID: id, CreatedAt: created, ExerciseIDs: ids, Photos: photos, Status: progress.SubmissionGraded,
		Provider: provider, Model: "qwen2.5vl:7b", Results: results,
	}
}

// richProgress is mia's progress in the fixture cassette: the first lesson done,
// one exercise passed, one (write-the-sum) waiting for the parent, and two submissions.
func richProgress(p *progress.Progress) {
	p.Lessons[lessonWhat] = progress.LessonProgress{CompletedAt: at(17, 2, 11), Plays: 2}
	p.Exercises[exShade] = progress.ExerciseProgress{
		State: progress.StatePassed, Attempts: 2, HintsShown: 1,
		PassedBy: progress.PassedByAI, PassedAt: at(17, 20, 40), LastVerdict: progress.VerdictCorrect,
	}
	p.Exercises[exSum] = progress.ExerciseProgress{State: progress.StateNeedsReview, Attempts: 5, HintsShown: 2, LastVerdict: progress.VerdictIncorrect}
	older := graded(subOlder, at(17, 19, 55), "Home Ollama", []string{"1.jpg", "2.jpg"},
		progress.Result{
			ExerciseID: exShade, Verdict: progress.VerdictCorrect, Transcription: "A circle cut in two halves, one half shaded.",
			FeedbackForChild: "Well done! Two equal halves.", NotesForParent: "Neat drawing.", CountedAttempt: true,
		},
		progress.Result{
			ExerciseID: exName, Verdict: progress.VerdictPartial, Transcription: "1/3",
			FeedbackForChild: "Close. How many parts are there in all?", NotesForParent: "Wrote the numerator right.", CountedAttempt: true,
		})
	older.PageNotes = "The page is a little dark but readable."
	newer := graded(subNewer, at(18, 5, 12), "OpenAI", []string{"1.jpg"},
		progress.Result{
			ExerciseID: exSum, Verdict: progress.VerdictIncorrect, Transcription: "1/4 + 2/4 = 2/4",
			FeedbackForChild: "Not yet. Add only the tops.", NotesForParent: "Added the bottoms as well.", CountedAttempt: true,
		})
	p.Submissions = []progress.Submission{older, newer}
}

// richSetup installs the cassettes, gives mia richProgress, writes the photos of
// the submissions and adds a local and a cloud provider.
func richSetup(t *testing.T, env *app.Env) {
	t.Helper()
	twoCassettes(t, env)
	setProgress(t, env, "mia", fractionsID, richProgress)
	writePhoto(t, env, "mia", fractionsID, subOlder, "1.jpg")
	writePhoto(t, env, "mia", fractionsID, subOlder, "2.jpg")
	writePhoto(t, env, "mia", fractionsID, subNewer, "1.jpg")
	addProvider(t, env, "Home Ollama", "", true)
	err := env.Providers.Add(context.Background(), providers.Config{
		Name: "OpenAI", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o", APIKey: "sk-1", Kind: providers.KindCloud,
	})
	if err != nil {
		t.Fatal(err)
	}
}

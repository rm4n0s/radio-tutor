package admin

import (
	"context"
	"testing"
	"time"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

// sumID is the identity of mia's waiting exercise of richSetup in the queue.
var sumID = QueueID("mia", fractionsID, exSum)

// pumpNotice logs in with richSetup and leaves the notice open.
func pumpNotice(t *testing.T, c config) *testutil.App {
	t.Helper()
	c.keepNotice = true
	c.screens = progressScreens
	if c.setup == nil {
		c.setup = richSetup
	}
	return pump(t, c)
}

// pumpQueue logs in with richSetup and opens the review queue with Review now.
func pumpQueue(t *testing.T, c config) *testutil.App {
	t.Helper()
	a := pumpNotice(t, c)
	a.TapKey(KeyNoticeNow)
	waitKey(t, a, KeyQueueItemPrefix+sumID)
	settleImages(a)
	return a
}

func TestNoticeAtLoginListsTheWaitingExercises(t *testing.T) {
	a := pumpNotice(t, config{})
	for _, text := range []string{"Exercises are waiting for you", "1 waiting for review", "Review now", "Later",
		"Mia in Fractions, part 1 - Exercise 3: Write the sum"} {
		if !has(a, text) {
			t.Errorf("the notice does not show %q", text)
		}
	}
	// Later closes it; the home keeps the count and the notice does not come back.
	a.TapKey(KeyNoticeLater)
	a.PumpAndSettle()
	waitGone(t, a, KeyReviewNotice)
	if !has(a, "1 waiting for review") {
		t.Error("the home lost the count after Later")
	}
	a.TapKey(KeyStudents)
	waitKey(t, a, KeyAddStudent)
	goBack(t, a)
	time.Sleep(100 * time.Millisecond)
	a.FlushPosted()
	a.PumpAndSettle()
	if a.HasKey(KeyReviewNotice) {
		t.Error("the notice came back while the admin was still logged in")
	}
}

func TestNoticeComesAtEveryLogin(t *testing.T) {
	a := pumpNotice(t, config{})
	a.TapKey(KeyNoticeLater)
	a.PumpAndSettle()
	a.TapKey(app.KeyLogout)
	waitKey(t, a, app.KeyProfilePrefix+"mum")
	a.TapKey(app.KeyProfilePrefix + "mum")
	a.EnterKey(app.KeyPassword, adminPassword)
	a.TapKey(app.KeySubmit)
	waitKey(t, a, KeyReviewNotice)
}

func TestNoNoticeWhenNothingWaits(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		twoCassettes(t, env)
		setProgress(t, env, "mia", fractionsID, func(p *progress.Progress) {
			p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StatePassed, Attempts: 1}
		})
	}})
	time.Sleep(150 * time.Millisecond)
	a.FlushPosted()
	a.PumpAndSettle()
	if a.HasKey(KeyReviewNotice) {
		t.Error("the notice opened although nothing waits")
	}
	if !has(a, "0 waiting for review") {
		t.Error("the home does not say that nothing waits")
	}
}

func TestNoticeCountsEveryStudentAndCassette(t *testing.T) {
	a := pumpNotice(t, config{size: m.Size{Width: app.MinWindowWidth, Height: 1400}, setup: func(t *testing.T, env *app.Env) {
		twoCassettes(t, env)
		for _, user := range []string{"mia", "mum"} {
			for _, id := range []string{fractionsID, otherID} {
				setProgress(t, env, user, id, func(p *progress.Progress) {
					p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StateNeedsReview, Attempts: 3}
					p.Exercises[exSum] = progress.ExerciseProgress{State: progress.StateNeedsReview, Attempts: 5}
				})
			}
		}
	}})
	if !has(a, "8 waiting for review") {
		t.Error("the notice does not count the exercises of every student and cassette")
	}
	// It lists the first ones and counts the rest.
	listed := 0
	for _, user := range []string{"mia", "mum"} {
		for _, id := range []string{fractionsID, otherID} {
			for _, ex := range []string{exShade, exSum} {
				if a.HasKey(KeyNoticeLinePrefix + QueueID(user, id, ex)) {
					listed++
				}
			}
		}
	}
	if listed != noticeMaxLines || !has(a, "and 2 more") {
		t.Errorf("the notice lists %d exercises, want %d and \"and 2 more\"", listed, noticeMaxLines)
	}
}

func TestReviewNowShowsTheFailedAttempts(t *testing.T) {
	a := pumpQueue(t, config{})
	attempt := sumID + "/" + subNewer
	for _, k := range []string{KeyQueueItemPrefix + sumID, KeyQueuePromptPrefix + sumID, KeyQueueAttemptPrefix + attempt,
		KeyQueuePhotoPrefix + attempt + "/1", KeyQueueVerdictPrefix + attempt, KeyQueuePassPrefix + sumID, KeyQueueMorePrefix + sumID,
		KeyQueueReferenceButtonPrefix + sumID} {
		if !a.HasKey(k) {
			t.Errorf("the queue has no %s", k)
		}
	}
	for _, text := range []string{"Waiting for you (1)", "Mia in Fractions, part 1", "Exercise 3: Write the sum",
		"1/4 + 2/4 = 2/4", "Not yet. Add only the tops.", "Added the bottoms as well.", "Verdict: incorrect", "Pass", "Give more attempts",
		"2026-10-30 18:05"} {
		if !has(a, text) {
			t.Errorf("the queue does not show %q", text)
		}
	}
	if got := textOf(t, a, KeyQueuePromptPrefix+sumID); got != "Add the fractions and write the result:\n1/4 + 2/4 = ?" {
		t.Errorf("the prompt is %q", got)
	}
	// The reference solution is shut until the parent opens it.
	if a.HasKey(KeyQueueReferencePrefix + sumID) {
		t.Fatal("the reference solution is open at first")
	}
	a.TapKey(KeyQueueReferenceButtonPrefix + sumID)
	a.PumpAndSettle()
	if got := textOf(t, a, KeyQueueReferencePrefix+sumID); got != "3/4" {
		t.Errorf("the reference solution is %q", got)
	}
	a.TapKey(KeyQueueReferenceButtonPrefix + sumID)
	a.PumpAndSettle()
	if a.HasKey(KeyQueueReferencePrefix + sumID) {
		t.Error("the reference solution did not close")
	}
	// The older submission of mia has no attempt at this exercise, and the
	// passed or partial results of the other exercises are not shown.
	if a.HasKey(KeyQueueAttemptPrefix + sumID + "/" + subOlder) {
		t.Error("an attempt at another exercise is shown")
	}
}

func TestQueuePhotoOpensTheZoomableView(t *testing.T) {
	a := pumpQueue(t, config{})
	a.TapKey(KeyQueuePhotoPrefix + sumID + "/" + subNewer + "/1")
	waitKey(t, a, KeyPhotoViewer)
	if !a.HasKey(KeyPhotoZoom) || !has(a, "Photo 1 of 1") {
		t.Error("the photo does not open in the zoomable full-size view")
	}
}

func TestQueuePass(t *testing.T) {
	a := pumpQueue(t, config{})
	a.TapKey(KeyQueuePassPrefix + sumID)
	waitSaved(t, a, "mia", fractionsID, "the exercise to be passed", func(p *progress.Progress) bool {
		return p.Exercises[exSum].State == progress.StatePassed
	})
	p := loadSaved(t, a.Env, "mia", fractionsID)
	if ep := p.Exercises[exSum]; ep.PassedBy != progress.PassedByParent {
		t.Errorf("passed_by is %q, want parent", ep.PassedBy)
	}
	if n := len(p.Overrides); n != 1 || p.Overrides[0].Action != progress.ActionMarkPassed || p.Overrides[0].By != "mum" {
		t.Errorf("overrides = %+v", p.Overrides)
	}
	waitKey(t, a, KeyQueueEmpty)
	if !has(a, "Nothing is waiting") {
		t.Error("an empty queue does not say so")
	}
	a.TapKey(KeyQueueHome)
	a.PumpAndSettle()
	waitKey(t, a, KeyStudents)
	waitText(t, a, "0 waiting for review")
}

func TestQueueGiveMoreAttempts(t *testing.T) {
	a := pumpQueue(t, config{})
	a.TapKey(KeyQueueMorePrefix + sumID)
	waitKey(t, a, KeyDialogOK)
	if !has(a, "The attempts and hints of exercise 3 are reset. Mia will have to solve it before going further.") {
		t.Error("the confirmation does not say that the child has to solve the exercise")
	}
	// Cancel changes nothing.
	a.TapKey(KeyDialogCancel)
	a.PumpAndSettle()
	if p := loadSaved(t, a.Env, "mia", fractionsID); p.Exercises[exSum].Attempts != 5 {
		t.Fatalf("Cancel reset the attempts: %+v", p.Exercises[exSum])
	}
	if !a.HasKey(KeyQueueItemPrefix + sumID) {
		t.Fatal("Cancel took the exercise out of the queue")
	}
	a.TapKey(KeyQueueMorePrefix + sumID)
	waitKey(t, a, KeyDialogOK)
	a.TapKey(KeyDialogOK)
	waitSaved(t, a, "mia", fractionsID, "the attempts to be reset", func(p *progress.Progress) bool {
		return p.Exercises[exSum].Attempts == 0
	})
	p := loadSaved(t, a.Env, "mia", fractionsID)
	if ep := p.Exercises[exSum]; ep.HintsShown != 0 || ep.State != "" {
		t.Errorf("the exercise after Give more attempts is %+v", ep)
	}
	if n := len(p.Overrides); n != 1 || p.Overrides[0].Action != progress.ActionResetAttempts {
		t.Errorf("overrides = %+v", p.Overrides)
	}
	waitKey(t, a, KeyQueueEmpty)
	a.TapKey(KeyQueueHome)
	a.PumpAndSettle()
	waitText(t, a, "0 waiting for review")
}

func TestQueueShowsDeletedPhotos(t *testing.T) {
	a := pumpNotice(t, config{setup: func(t *testing.T, env *app.Env) {
		richSetup(t, env)
		setProgress(t, env, "mia", fractionsID, func(p *progress.Progress) {
			p.Submission(subNewer).Photos = []string{}
		})
	}})
	a.TapKey(KeyNoticeNow)
	attempt := sumID + "/" + subNewer
	waitKey(t, a, KeyQueueAttemptPrefix+attempt)
	if !a.HasKey(KeyQueueNoPhotosPrefix+attempt) || !has(a, "Photos deleted") {
		t.Error("an attempt without photos does not say that they were deleted")
	}
	for _, text := range []string{"1/4 + 2/4 = 2/4", "Not yet. Add only the tops.", "Added the bottoms as well.", "Verdict: incorrect"} {
		if !has(a, text) {
			t.Errorf("the attempt without photos lost %q", text)
		}
	}
}

func TestQueueIsOpenedFromTheProgressScreenAfterLater(t *testing.T) {
	a := pumpNotice(t, config{})
	a.TapKey(KeyNoticeLater)
	a.PumpAndSettle()
	a.TapKey(KeyProgress)
	waitKey(t, a, KeyOpenReview)
	if !has(a, "Review the exercises that wait (1)") {
		t.Error("the progress screen does not count the exercises that wait")
	}
	a.TapKey(KeyOpenReview)
	waitKey(t, a, KeyQueueItemPrefix+sumID)
	// Back to the home goes all the way, also from here.
	a.TapKey(KeyQueueMorePrefix + sumID)
	waitKey(t, a, KeyDialogOK)
	a.TapKey(KeyDialogOK)
	waitKey(t, a, KeyQueueEmpty)
	a.TapKey(KeyQueueHome)
	a.PumpAndSettle()
	waitKey(t, a, KeyStudents)
}

func TestQueueGreek(t *testing.T) {
	a := pumpNotice(t, config{locale: "el_GR.UTF-8"})
	for _, text := range []string{"Υπάρχουν ασκήσεις που περιμένουν εσένα", "Έλεγχος τώρα", "Αργότερα"} {
		if !has(a, text) {
			t.Errorf("the Greek notice does not show %q", text)
		}
	}
	a.TapKey(KeyNoticeNow)
	waitKey(t, a, KeyQueueItemPrefix+sumID)
	for _, text := range []string{"Πέρασέ την", "Δώσε κι άλλες προσπάθειες", "Προσπάθειες που δεν πέρασαν", "Εμφάνιση της σωστής λύσης"} {
		if !has(a, text) {
			t.Errorf("the Greek queue does not show %q", text)
		}
	}
}

// TestFailedAttempts checks which results the queue shows for an exercise.
func TestFailedAttempts(t *testing.T) {
	res := func(id string, v progress.Verdict) progress.Result {
		return progress.Result{ExerciseID: id, Verdict: v}
	}
	sub := func(id string, h int, results ...progress.Result) progress.Submission {
		return graded(id, at(h, 0, 0), "P", []string{"1.jpg"}, results...)
	}
	tests := []struct {
		name      string
		subs      []progress.Submission
		overrides []progress.Override
		want      []string // submission ids, newest first
	}{
		{"none", nil, nil, nil},
		{"only the exercise and only what did not pass", []progress.Submission{
			sub("a", 9, res("x", progress.VerdictIncorrect)),
			sub("b", 10, res("y", progress.VerdictIncorrect)),
			sub("c", 11, res("x", progress.VerdictCorrect)),
			sub("d", 12, res("x", progress.VerdictPartial), res("y", progress.VerdictCorrect)),
			sub("e", 8, res("x", progress.VerdictUnreadable)),
		}, nil, []string{"d", "a", "e"}},
		{"newest first whatever the order in the file", []progress.Submission{
			sub("a", 11, res("x", progress.VerdictIncorrect)),
			sub("b", 9, res("x", progress.VerdictIncorrect)),
			sub("c", 10, res("x", progress.VerdictIncorrect)),
		}, nil, []string{"a", "c", "b"}},
		{"attempts before a reset of the exercise were given up", []progress.Submission{
			sub("a", 9, res("x", progress.VerdictIncorrect)),
			sub("b", 11, res("x", progress.VerdictIncorrect)),
		}, []progress.Override{{Action: progress.ActionResetAttempts, ExerciseID: "x", At: at(10, 0, 0)}}, []string{"b"}},
		{"a reset of another exercise changes nothing", []progress.Submission{
			sub("a", 9, res("x", progress.VerdictIncorrect)),
		}, []progress.Override{{Action: progress.ActionResetAttempts, ExerciseID: "y", At: at(10, 0, 0)}}, []string{"a"}},
		{"reset all progress gives all up", []progress.Submission{
			sub("a", 9, res("x", progress.VerdictIncorrect)),
			sub("b", 11, res("x", progress.VerdictIncorrect)),
		}, []progress.Override{{Action: progress.ActionResetAll, At: at(10, 0, 0)}}, []string{"b"}},
		{"mark passed does not hide attempts", []progress.Submission{
			sub("a", 9, res("x", progress.VerdictIncorrect)),
		}, []progress.Override{{Action: progress.ActionMarkNotPassed, ExerciseID: "x", At: at(10, 0, 0)}}, []string{"a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := progress.New("c")
			p.Submissions, p.Overrides = tt.subs, tt.overrides
			var got []string
			for _, a := range failedAttempts(p, "x") {
				got = append(got, a.SubmissionID)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("attempts = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("attempts = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestScanQueueIsOldestFirst(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		richSetup(t, env)
		// mum has an older failed attempt in the other cassette, and an exercise
		// with no attempt shown at all, which comes first.
		setProgress(t, env, "mum", otherID, func(p *progress.Progress) {
			p.Exercises[exSum] = progress.ExerciseProgress{State: progress.StateNeedsReview, Attempts: 5}
			p.Submissions = []progress.Submission{graded("20261030T090000-aaaa", at(9, 0, 0), "P", []string{},
				progress.Result{ExerciseID: exSum, Verdict: progress.VerdictIncorrect, FeedbackForChild: "No."})}
		})
		setProgress(t, env, "mum", fractionsID, func(p *progress.Progress) {
			p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StateNeedsReview, Attempts: 3}
		})
	}, keepNotice: true})
	items, err := scanQueue(context.Background(), a.Env)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, it := range items {
		got = append(got, it.ID())
	}
	want := []string{QueueID("mum", fractionsID, exShade), QueueID("mum", otherID, exSum), sumID}
	if len(got) != len(want) {
		t.Fatalf("queue = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("queue = %v, want %v", got, want)
		}
	}
}

package admin

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

func TestProgressListsThePeopleWithTheirWaitingExercises(t *testing.T) {
	a := pumpProgress(t, config{setup: richSetup})
	a.TapKey(KeyProgress)
	waitKey(t, a, KeyProgressStudentPrefix+"mia")
	for _, text := range []string{"Mum", "Mia", "Parent", "Student"} {
		if !has(a, text) {
			t.Errorf("the list does not show %q", text)
		}
	}
	waitText(t, a, "1 waiting for review")
}

func TestStudentPageListsInstalledCassettesAndSaveFilesOnly(t *testing.T) {
	a := pumpProgress(t, config{setup: func(t *testing.T, env *app.Env) {
		twoCassettes(t, env)
		setProgress(t, env, "mia", goneID, func(p *progress.Progress) {
			p.Lessons[lessonWhat] = progress.LessonProgress{CompletedAt: at(9, 0, 0)}
			p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StatePassed, Attempts: 1, PassedBy: progress.PassedByAI, PassedAt: at(9, 5, 0)}
		})
	}})
	openStudentPage(t, a, "mia")
	for _, id := range []string{fractionsID, otherID, goneID} {
		waitKey(t, a, KeyProgressCassettePrefix+id)
	}
	waitText(t, a, "Not installed")

	a.TapKey(KeyProgressCassettePrefix + goneID)
	waitKey(t, a, KeyReadOnly)
	if !has(a, "Lesson: "+lessonWhat) || !has(a, "Exercise "+exShade+": "+exShade) {
		t.Error("the save file's entries are not listed by id")
	}
	for _, k := range []string{
		OverrideKey(SectionItems, progress.ActionMarkPassed, exShade),
		OverrideKey(SectionItems, progress.ActionMarkNotPassed, exShade),
		OverrideKey(SectionItems, progress.ActionMarkNotCompleted, lessonWhat),
		KeyAlsoPhotos,
	} {
		if a.HasKey(k) {
			t.Errorf("a cassette that is not installed has %s", k)
		}
	}
	// The switches and Reset all are disabled: nothing changes.
	a.TapKey(KeySkipLessons)
	a.TapKey(KeyResetAll)
	if a.HasKey(KeyDialogOK) {
		t.Error("Reset all opened a dialog for a cassette that is not installed")
	}
	if p := loadSaved(t, a.Env, "mia", goneID); p.Settings.SkipLessons || len(p.Overrides) != 0 {
		t.Errorf("the save file of the cassette that is not installed changed: %+v", p)
	}
}

func TestStudentPageOfADamagedCassetteIsReadOnly(t *testing.T) {
	a := pumpProgress(t, config{setup: func(t *testing.T, env *app.Env) {
		install(t, env, fixture(t, fractionsID, "1.0.0", fractionsTitle))
		if err := os.WriteFile(filepath.Join(env.Paths.Data, "cassettes", fractionsID, "lessons", lessonWhat, "lesson.json"), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
	}})
	openCassettePageOf(t, a, "mia", fractionsID)
	waitKey(t, a, KeyReadOnly)
	if !has(a, "This cassette is damaged. You can look at what was saved, but nothing can be changed. Remove the cassette and import it again.") {
		t.Error("the page does not say the cassette is damaged")
	}
}

func TestOverridesChangeTheSaveFileAndTheShownStates(t *testing.T) {
	passedByAI := progress.ExerciseProgress{State: progress.StatePassed, Attempts: 1, PassedBy: progress.PassedByAI, PassedAt: at(10, 0, 0)}
	tests := []struct {
		name   string
		before func(p *progress.Progress)
		action progress.Action
		target string
		wantAt string // the item whose state text is checked
		want   string // the state text it shows afterwards
		check  func(t *testing.T, p *progress.Progress)
	}{
		{
			"mark passed", func(p *progress.Progress) {}, progress.ActionMarkPassed, exShade, exShade, "Passed",
			func(t *testing.T, p *progress.Progress) {
				ep := p.Exercises[exShade]
				if ep.State != progress.StatePassed || ep.PassedBy != progress.PassedByParent || ep.PassedAt.IsZero() {
					t.Errorf("the exercise is %+v, want passed by a parent", ep)
				}
			},
		},
		{
			"mark not passed", func(p *progress.Progress) { p.Exercises[exShade] = passedByAI }, progress.ActionMarkNotPassed, exShade, exShade, "Open",
			func(t *testing.T, p *progress.Progress) {
				if ep := p.Exercises[exShade]; ep.State != "" || ep.PassedBy != "" || !ep.PassedAt.IsZero() || ep.Attempts != 1 {
					t.Errorf("the exercise is %+v, want not passed and 1 attempt kept", ep)
				}
			},
		},
		{
			"mark not passed with the attempts used up", func(p *progress.Progress) {
				ep := passedByAI
				ep.Attempts = 3
				p.Exercises[exShade] = ep
			}, progress.ActionMarkNotPassed, exShade, exShade, "Needs you",
			func(t *testing.T, p *progress.Progress) {
				if ep := p.Exercises[exShade]; ep.State != progress.StateNeedsReview {
					t.Errorf("the exercise is %+v, want needs_review", ep)
				}
			},
		},
		{
			"reset attempts", func(p *progress.Progress) {
				p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StateNeedsReview, Attempts: 3, HintsShown: 2}
			}, progress.ActionResetAttempts, exShade, exShade, "Open",
			func(t *testing.T, p *progress.Progress) {
				if ep, ok := p.Exercises[exShade]; ok {
					t.Errorf("the exercise still has an entry: %+v", ep)
				}
			},
		},
		{
			"mark completed", func(p *progress.Progress) { p.Lessons[lessonWhat] = progress.LessonProgress{Plays: 3} },
			progress.ActionMarkCompleted, lessonWhat, lessonWhat, "Completed",
			func(t *testing.T, p *progress.Progress) {
				if lp := p.Lessons[lessonWhat]; !lp.Completed() || lp.Plays != 3 {
					t.Errorf("the lesson is %+v, want completed with 3 plays", lp)
				}
			},
		},
		{
			"mark not completed", func(p *progress.Progress) {
				p.Lessons[lessonWhat] = progress.LessonProgress{CompletedAt: at(10, 0, 0), Plays: 1}
			}, progress.ActionMarkNotCompleted, lessonWhat, lessonWhat, "Open",
			func(t *testing.T, p *progress.Progress) {
				if lp := p.Lessons[lessonWhat]; lp.Completed() || lp.Plays != 1 {
					t.Errorf("the lesson is %+v, want not completed with 1 play", lp)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pumpProgress(t, config{setup: func(t *testing.T, env *app.Env) {
				twoCassettes(t, env)
				setProgress(t, env, "mia", fractionsID, func(p *progress.Progress) {
					p.Lessons[lessonWhat] = progress.LessonProgress{CompletedAt: at(9, 0, 0)} // opens the exercises
					tt.before(p)
				})
			}})
			openCassettePageOf(t, a, "mia", fractionsID)
			a.TapKey(OverrideKey(SectionItems, tt.action, tt.target))
			waitSaved(t, a, "mia", fractionsID, "the override to be saved", func(p *progress.Progress) bool { return len(p.Overrides) == 1 })
			p := loadSaved(t, a.Env, "mia", fractionsID)
			tt.check(t, p)
			o := p.Overrides[0]
			if o.Action != tt.action || o.By != "mum" || o.At.IsZero() {
				t.Errorf("the override is %+v, want %s by mum", o, tt.action)
			}
			if o.ExerciseID != tt.target && o.LessonID != tt.target {
				t.Errorf("the override is about %q/%q, want %s", o.ExerciseID, o.LessonID, tt.target)
			}
			a.WaitFor(t, "the new state", func() bool { return textOf(t, a, StateKey(SectionItems, tt.wantAt)) == tt.want })
		})
	}
}

func TestOverrideButtonsFitTheState(t *testing.T) {
	a := pumpProgress(t, config{setup: richSetup})
	openCassettePageOf(t, a, "mia", fractionsID)
	tests := []struct {
		section string
		action  progress.Action
		id      string
		want    bool
	}{
		{SectionItems, progress.ActionMarkNotPassed, exShade, true}, // passed
		{SectionItems, progress.ActionMarkPassed, exShade, false},
		{SectionItems, progress.ActionMarkPassed, exSum, true}, // needs review
		{SectionItems, progress.ActionResetAttempts, exSum, true},
		{SectionItems, progress.ActionMarkNotPassed, exSum, false},
		{SectionItems, progress.ActionMarkPassed, exName, true}, // open, nothing to reset
		{SectionItems, progress.ActionResetAttempts, exName, false},
		{SectionItems, progress.ActionMarkNotCompleted, lessonWhat, true},
		{SectionItems, progress.ActionMarkCompleted, lessonWhat, false},
		{SectionItems, progress.ActionMarkCompleted, lessonAdding, true},
		{SectionWaiting, progress.ActionMarkPassed, exSum, true},
		{SectionWaiting, progress.ActionMarkPassed, exShade, false},
	}
	for _, tt := range tests {
		if got := a.HasKey(OverrideKey(tt.section, tt.action, tt.id)); got != tt.want {
			t.Errorf("%s %s %s: shown %v, want %v", tt.section, tt.action, tt.id, got, tt.want)
		}
	}
}

func TestItemsShowAttemptsHintsAndDates(t *testing.T) {
	a := pumpProgress(t, config{setup: richSetup})
	openCassettePageOf(t, a, "mia", fractionsID)
	for _, text := range []string{
		"Lesson: What is a fraction?", "Played 2 times", "Completed on " + formatTime(time.UTC, at(17, 2, 11)),
		"Exercise 1: Shade the half", "Attempts: 2 of 3", "Hints shown: 1", "Passed by the AI on " + formatTime(time.UTC, at(17, 20, 40)),
		"Attempts: 0", "Attempts: 5 of 5", "Hints shown: 2", "Not completed",
	} {
		if !has(a, text) {
			t.Errorf("the page does not show %q", text)
		}
	}
}

func TestSwitchesAreSavedAtOnce(t *testing.T) {
	a := pumpProgress(t, config{setup: func(t *testing.T, env *app.Env) { twoCassettes(t, env) }})
	openCassettePageOf(t, a, "mia", fractionsID)
	// With the first lesson not completed the exercises are locked.
	if got := textOf(t, a, StateKey(SectionItems, exShade)); got != "Locked" {
		t.Fatalf("the exercise is %q before the switches, want Locked", got)
	}
	a.TapKey(KeySkipLessons)
	waitSaved(t, a, "mia", fractionsID, "skip lessons", func(p *progress.Progress) bool { return p.Settings.SkipLessons })
	a.WaitFor(t, "the exercise to open", func() bool { return textOf(t, a, StateKey(SectionItems, exShade)) == "Open" })
	a.TapKey(KeyFreeNavigation)
	waitSaved(t, a, "mia", fractionsID, "free navigation", func(p *progress.Progress) bool { return p.Settings.FreeNavigation })
	if p := loadSaved(t, a.Env, "mia", fractionsID); !p.Settings.SkipLessons || len(p.Overrides) != 0 {
		t.Errorf("the settings are %+v with %d overrides", p.Settings, len(p.Overrides))
	}
	goBack(t, a)
	waitKey(t, a, KeyProgressCassettePrefix+fractionsID)
	a.TapKey(KeyProgressCassettePrefix + fractionsID)
	waitKey(t, a, KeySkipLessons)
	a.TapKey(KeySkipLessons) // off again
	waitSaved(t, a, "mia", fractionsID, "skip lessons off", func(p *progress.Progress) bool { return !p.Settings.SkipLessons })
	if p := loadSaved(t, a.Env, "mia", fractionsID); !p.Settings.FreeNavigation {
		t.Error("turning one switch off turned the other off")
	}
}

func TestWaitingExercisesAreListedFirst(t *testing.T) {
	a := pumpProgress(t, config{setup: richSetup})
	openCassettePageOf(t, a, "mia", fractionsID)
	waitKey(t, a, KeyWaitingSection)
	if !has(a, "Waiting for you (1)") {
		t.Error("the section does not say how many wait")
	}
	if !a.HasKey(ItemKey(SectionWaiting, exSum)) || a.HasKey(ItemKey(SectionWaiting, exShade)) {
		t.Error("the waiting section is not just the exercise that needs review")
	}
	waiting := a.TopLeft(key(KeyWaitingSection)).Dy
	for _, k := range []string{KeySkipLessons, ItemKey(SectionItems, lessonWhat), KeySubmissionPrefix + subOlder} {
		if below := a.TopLeft(key(k)).Dy; below <= waiting {
			t.Errorf("%s (y=%v) is not below the waiting section (y=%v)", k, below, waiting)
		}
	}

	// Mark passed from the waiting card: the section goes away.
	a.TapKey(OverrideKey(SectionWaiting, progress.ActionMarkPassed, exSum))
	waitGone(t, a, KeyWaitingSection)
	if got := textOf(t, a, StateKey(SectionItems, exSum)); got != "Passed" {
		t.Errorf("the exercise is %q, want Passed", got)
	}
}

func TestNoWaitingSectionWithoutWaitingExercises(t *testing.T) {
	a := pumpProgress(t, config{setup: func(t *testing.T, env *app.Env) { twoCassettes(t, env) }})
	openCassettePageOf(t, a, "mia", fractionsID)
	if a.HasKey(KeyWaitingSection) {
		t.Error("the page has a waiting section")
	}
}

func TestHomeCountFollowsTheDecisions(t *testing.T) {
	a := pumpProgress(t, config{setup: func(t *testing.T, env *app.Env) {
		twoCassettes(t, env)
		setProgress(t, env, "mia", fractionsID, func(p *progress.Progress) {
			p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StateNeedsReview, Attempts: 3}
			p.Exercises[exSum] = progress.ExerciseProgress{State: progress.StateNeedsReview, Attempts: 5}
		})
		setProgress(t, env, "mum", otherID, func(p *progress.Progress) {
			p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StateNeedsReview, Attempts: 3}
		})
		// Not installed: nothing can be decided, so it is not counted.
		setProgress(t, env, "mia", goneID, func(p *progress.Progress) {
			p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StateNeedsReview, Attempts: 3}
		})
	}})
	waitText(t, a, "3 waiting for review")
	openCassettePageOf(t, a, "mia", fractionsID)
	a.TapKey(OverrideKey(SectionWaiting, progress.ActionMarkPassed, exShade))
	waitSaved(t, a, "mia", fractionsID, "the pass", func(p *progress.Progress) bool { return p.Exercises[exShade].State == progress.StatePassed })
	goBack(t, a)
	goBack(t, a)
	goBack(t, a)
	waitText(t, a, "2 waiting for review")
}

func TestResetAllProgress(t *testing.T) {
	tests := []struct {
		name         string
		deletePhotos bool
	}{
		{"photos stay", false},
		{"photos deleted too", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pumpProgress(t, config{setup: func(t *testing.T, env *app.Env) {
				richSetup(t, env)
				setProgress(t, env, "mia", fractionsID, func(p *progress.Progress) {
					p.Settings.SkipLessons = true
					p.LastItem = exName
					p.Overrides = append(p.Overrides, progress.Override{ExerciseID: exSum, Action: progress.ActionMarkPassed, By: "mum", At: at(12, 0, 0)})
				})
			}})
			openCassettePageOf(t, a, "mia", fractionsID)
			a.TapKey(KeyResetAll)
			waitKey(t, a, KeyAlsoPhotos)
			if tt.deletePhotos {
				a.TapKey(KeyAlsoPhotos)
			}
			a.TapKey(KeyDialogOK)
			waitSaved(t, a, "mia", fractionsID, "the reset", func(p *progress.Progress) bool { return len(p.Exercises) == 0 })
			p := loadSaved(t, a.Env, "mia", fractionsID)
			if len(p.Lessons) != 0 || p.LastItem != "" {
				t.Errorf("lessons %v and last item %q are left", p.Lessons, p.LastItem)
			}
			if !p.Settings.SkipLessons {
				t.Error("the settings were cleared")
			}
			if n := len(p.Overrides); n != 2 || p.Overrides[0].Action != progress.ActionMarkPassed ||
				p.Overrides[1].Action != progress.ActionResetAll || p.Overrides[1].By != "mum" {
				t.Errorf("the overrides are %+v, want the old one and a reset_all by mum", p.Overrides)
			}
			photos := filepath.Join(a.Paths.PhotosDir(), "mia", fractionsID)
			_, statErr := os.Stat(photos)
			if tt.deletePhotos {
				if !os.IsNotExist(statErr) || len(p.Submissions) != 0 {
					t.Errorf("the photos (%v) or the submissions (%d) are left", statErr, len(p.Submissions))
				}
			} else if statErr != nil || len(p.Submissions) != 2 {
				t.Errorf("the photos (%v) or the submissions (%d) were removed", statErr, len(p.Submissions))
			}
			waitText(t, a, "The progress was reset.")
		})
	}
}

func TestResetAllCanBeCancelled(t *testing.T) {
	a := pumpProgress(t, config{setup: richSetup})
	openCassettePageOf(t, a, "mia", fractionsID)
	a.TapKey(KeyResetAll)
	waitKey(t, a, KeyAlsoPhotos)
	a.TapKey(KeyDialogCancel)
	waitGone(t, a, KeyAlsoPhotos)
	if p := loadSaved(t, a.Env, "mia", fractionsID); len(p.Exercises) != 2 || len(p.Overrides) != 0 {
		t.Errorf("a cancelled reset changed the save file: %+v", p)
	}
}

func TestSubmissionsListNewestFirstWithProviderKinds(t *testing.T) {
	a := pumpProgress(t, config{setup: func(t *testing.T, env *app.Env) {
		richSetup(t, env)
		setProgress(t, env, "mia", fractionsID, func(p *progress.Progress) {
			failed := graded("20261030T190000-0001", at(19, 0, 0), "Gone AI", nil)
			failed.Status, failed.Error = progress.SubmissionFailed, "the server did not answer"
			p.Submissions = append(p.Submissions, failed)
		})
	}})
	openCassettePageOf(t, a, "mia", fractionsID)
	y := func(id string) float64 { return a.TopLeft(key(KeySubmissionPrefix + id)).Dy }
	if !(y("20261030T190000-0001") < y(subNewer) && y(subNewer) < y(subOlder)) {
		t.Errorf("the submissions are not newest first: %v %v %v", y("20261030T190000-0001"), y(subNewer), y(subOlder))
	}
	for _, text := range []string{
		formatTime(time.UTC, at(17, 19, 55)), "Exercises: 1, B2", "Exercises: 3", "Home Ollama", "Local", "OpenAI", "Cloud",
		"Gone AI", "Removed provider", "Graded", "Failed",
	} {
		if !has(a, text) {
			t.Errorf("the list does not show %q", text)
		}
	}
}

func TestSubmissionDetail(t *testing.T) {
	a := pumpProgress(t, config{setup: richSetup})
	openSubmissionOf(t, a, "mia", fractionsID, subOlder)
	for _, k := range []string{
		KeyPhotoPrefix + "1", KeyPhotoPrefix + "2", KeyPageNotes, KeyProviderLine, KeyDeletePhotos,
		KeyResultPrefix + exShade, KeyResultPrefix + exName,
		KeyVerdictPrefix + exShade, KeyVerdictPrefix + exName,
		OverrideKey(SectionSubmission, progress.ActionMarkNotPassed, exShade), // passed now
		OverrideKey(SectionSubmission, progress.ActionMarkPassed, exName),
	} {
		if !a.HasKey(k) {
			t.Errorf("the submission has no %s", k)
		}
	}
	for k, want := range map[string]string{
		KeyPageNotes:                         "The page is a little dark but readable.",
		KeyTranscriptionPrefix + exName:      "1/3",
		KeyFeedbackPrefix + exName:           "Close. How many parts are there in all?",
		KeyParentNotesPrefix + exName:        "Wrote the numerator right.",
		KeyTranscriptionPrefix + exShade:     "A circle cut in two halves, one half shaded.",
		StateKey(SectionSubmission, exName):  "Open",
		StateKey(SectionSubmission, exShade): "Passed",
	} {
		if got := textOf(t, a, k); got != want {
			t.Errorf("%s shows %q, want %q", k, got, want)
		}
	}
	for _, text := range []string{"Verdict: correct", "Verdict: partly correct", "Model: qwen2.5vl:7b", "Exercise B2: Name the fraction"} {
		if !has(a, text) {
			t.Errorf("the submission does not show %q", text)
		}
	}

	// An override from here changes the save file and the state shown here.
	a.TapKey(OverrideKey(SectionSubmission, progress.ActionMarkPassed, exName))
	waitSaved(t, a, "mia", fractionsID, "the pass", func(p *progress.Progress) bool {
		return p.Exercises[exName].State == progress.StatePassed && p.Exercises[exName].PassedBy == progress.PassedByParent
	})
	a.WaitFor(t, "the new state", func() bool { return textOf(t, a, StateKey(SectionSubmission, exName)) == "Passed" })
	if p := loadSaved(t, a.Env, "mia", fractionsID); len(p.Overrides) != 1 || p.Overrides[0].By != "mum" {
		t.Errorf("the overrides are %+v", p.Overrides)
	}
}

func TestSubmissionWithoutItsPhotoFilesStillOpens(t *testing.T) {
	a := pumpProgress(t, config{setup: func(t *testing.T, env *app.Env) {
		twoCassettes(t, env)
		setProgress(t, env, "mia", fractionsID, richProgress)
	}})
	openSubmissionOf(t, a, "mia", fractionsID, subOlder)
	a.TapKey(KeyPhotoPrefix + "1")
	waitKey(t, a, KeyPhotoViewer)
}

func TestPhotoOpensFullSizeAndZoomable(t *testing.T) {
	a := pumpProgress(t, config{setup: richSetup})
	openSubmissionOf(t, a, "mia", fractionsID, subOlder)
	a.TapKey(KeyPhotoPrefix + "2")
	waitKey(t, a, KeyPhotoViewer)
	if !a.Exists(gluttertest.ByType[*m.InteractiveViewer]()) || !a.HasKey(KeyPhotoZoom) {
		t.Error("the photo is not in a zoomable viewer")
	}
	if !has(a, "Photo 2 of 2") {
		t.Error("the viewer does not say which photo it shows")
	}
	goBack(t, a)
	waitKey(t, a, KeyPhotoPrefix+"1")
}

func TestDeleteThePhotosOfASubmission(t *testing.T) {
	a := pumpProgress(t, config{setup: richSetup})
	openSubmissionOf(t, a, "mia", fractionsID, subOlder)

	a.TapKey(KeyDeletePhotos)
	waitKey(t, a, KeyDialogOK)
	a.TapKey(KeyDialogCancel)
	if _, err := os.Stat(filepath.Join(a.Paths.PhotosDir(), "mia", fractionsID, subOlder)); err != nil {
		t.Fatalf("cancelling deleted the photos: %v", err)
	}

	a.TapKey(KeyDeletePhotos)
	waitKey(t, a, KeyDialogOK)
	a.TapKey(KeyDialogOK)
	waitKey(t, a, KeyNoPhotos)
	if _, err := os.Stat(filepath.Join(a.Paths.PhotosDir(), "mia", fractionsID, subOlder)); !os.IsNotExist(err) {
		t.Errorf("the photos folder of the submission is still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(a.Paths.PhotosDir(), "mia", fractionsID, subNewer, "1.jpg")); err != nil {
		t.Errorf("the photos of another submission were deleted: %v", err)
	}
	p := loadSaved(t, a.Env, "mia", fractionsID)
	sub := p.Submission(subOlder)
	if sub == nil || len(sub.Photos) != 0 || len(sub.Results) != 2 || sub.PageNotes == "" || sub.Status != progress.SubmissionGraded {
		t.Errorf("the record is %+v, want it kept without photos", sub)
	}
	if other := p.Submission(subNewer); other == nil || len(other.Photos) != 1 {
		t.Errorf("the other submission is %+v", other)
	}
	if a.HasKey(KeyDeletePhotos) {
		t.Error("a submission without photos still offers to delete them")
	}
}

func TestDeleteAllPhotosOfAStudent(t *testing.T) {
	a := pumpProgress(t, config{setup: func(t *testing.T, env *app.Env) {
		richSetup(t, env)
		setProgress(t, env, "mia", otherID, func(p *progress.Progress) {
			p.Submissions = []progress.Submission{graded(subOlder, at(8, 0, 0), "Home Ollama", []string{"1.jpg"},
				progress.Result{ExerciseID: exShade, Verdict: progress.VerdictCorrect})}
		})
		writePhoto(t, env, "mia", otherID, subOlder, "1.jpg")
		setProgress(t, env, "mum", fractionsID, func(p *progress.Progress) {
			p.Submissions = []progress.Submission{graded(subOlder, at(8, 0, 0), "Home Ollama", []string{"1.jpg"})}
		})
		writePhoto(t, env, "mum", fractionsID, subOlder, "1.jpg")
	}})
	openStudentPage(t, a, "mia")

	a.TapKey(KeyDeleteStudentPhotos)
	waitKey(t, a, KeyDialogOK)
	a.TapKey(KeyDialogCancel)
	if _, err := os.Stat(filepath.Join(a.Paths.PhotosDir(), "mia")); err != nil {
		t.Fatalf("cancelling deleted the photos: %v", err)
	}

	a.TapKey(KeyDeleteStudentPhotos)
	waitKey(t, a, KeyDialogOK)
	a.TapKey(KeyDialogOK)
	waitText(t, a, "The photos were deleted.")
	if _, err := os.Stat(filepath.Join(a.Paths.PhotosDir(), "mia")); !os.IsNotExist(err) {
		t.Errorf("the photos of mia are still there: %v", err)
	}
	for _, id := range []string{fractionsID, otherID} {
		p := loadSaved(t, a.Env, "mia", id)
		if len(p.Submissions) == 0 {
			t.Errorf("%s: the records were removed", id)
		}
		for _, s := range p.Submissions {
			if len(s.Photos) != 0 {
				t.Errorf("%s: submission %s still lists photos %v", id, s.ID, s.Photos)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(a.Paths.PhotosDir(), "mum", fractionsID, subOlder, "1.jpg")); err != nil {
		t.Errorf("the photos of another person were deleted: %v", err)
	}
	if p := loadSaved(t, a.Env, "mum", fractionsID); len(p.Submissions[0].Photos) != 1 {
		t.Errorf("the record of another person changed: %+v", p.Submissions)
	}
}

func TestProgressInGreek(t *testing.T) {
	a := pumpProgress(t, config{locale: "el_GR.UTF-8", setup: richSetup})
	openCassettePageOf(t, a, "mia", fractionsID)
	for _, text := range []string{"Περιμένουν εσένα (1)", "Παράλειψη μαθημάτων", "Ελεύθερη πλοήγηση", "Μηδενισμός όλης της προόδου", "Μάθημα: What is a fraction?"} {
		if !has(a, text) {
			t.Errorf("the Greek page does not show %q", text)
		}
	}
}

// The cassette page reads the save file again when the submission page closes,
// and does it on the UI thread: Glutter reports State.Go from another goroutine
// as WrongThread, and the test fails for every error it did not take.
func TestCassettePageReloadsWhenASubmissionCloses(t *testing.T) {
	tests := []struct {
		name   string
		action progress.Action
		target string
		want   string // the state the cassette page shows afterwards
	}{
		{"mark passed", progress.ActionMarkPassed, exName, "Passed"},
		{"mark not passed", progress.ActionMarkNotPassed, exShade, "Open"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pumpProgress(t, config{setup: richSetup})
			openSubmissionOf(t, a, "mia", fractionsID, subOlder)
			a.TapKey(OverrideKey(SectionSubmission, tt.action, tt.target))
			waitSaved(t, a, "mia", fractionsID, "the override to be saved", func(p *progress.Progress) bool { return len(p.Overrides) == 1 })
			goBack(t, a)
			waitKey(t, a, KeySkipLessons)
			a.WaitFor(t, "the cassette page to show the override", func() bool {
				return textOf(t, a, StateKey(SectionItems, tt.target)) == tt.want
			})
			if errs := a.Errors(); len(errs) != 0 {
				t.Errorf("the framework reported %d error(s): %v", len(errs), errs)
			}
		})
	}
}

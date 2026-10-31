package student

import (
	"context"
	"testing"
	"time"

	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

const (
	lockedMessage  = "Finish the items before this one first."
	lessonSoon     = `The lesson "What is a fraction?" will open here soon.`
	worksheet1Soon = "The exercises of worksheet 1 will open here soon."
	worksheet2Soon = "The exercises of worksheet 2 will open here soon."
	doneTooltip    = "Done"
	lockedTooltip  = "Locked"
	passedTooltip  = "Passed"
	reviewTooltip  = "A parent will look at this"
	adding         = "Adding fractions"
)

var completedAt = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

// install1 installs the fixture cassette and applies change to mia's progress.
func install1(change func(p *progress.Progress)) func(t *testing.T, env *app.Env) {
	return func(t *testing.T, env *app.Env) {
		install(t, env, fixture(t, fractionsID, "1.0.0", fractionsTitle))
		if change != nil {
			setProgress(t, env, fractionsID, change)
		}
	}
}

// halfDone is lesson 1 completed and the first exercise of the first worksheet
// passed; the second one is still available, so the rest is locked.
func halfDone(p *progress.Progress) {
	p.Lessons[lesson1] = progress.LessonProgress{CompletedAt: completedAt, Plays: 1}
	p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StatePassed, Attempts: 1, PassedBy: progress.PassedByAI}
}

func TestCassetteScreenShowsTheStates(t *testing.T) {
	tests := []struct {
		name     string
		progress func(p *progress.Progress)
		// marks are item or exercise keys with the tooltip of their mark.
		marks map[string]string
		// unmarked are keys with no mark at all.
		unmarked []string
	}{
		{
			"a new student", nil,
			map[string]string{
				KeyItemPrefix + lesson2 + KeyStateSuffix: lockedTooltip,
				WorksheetKey(1) + KeyStateSuffix:         lockedTooltip,
				KeyExercisePrefix + exSum:                lockedTooltip,
				WorksheetKey(0) + KeyStateSuffix:         lockedTooltip,
				KeyExercisePrefix + exShade:              lockedTooltip,
				KeyExercisePrefix + exName:               lockedTooltip,
			},
			[]string{KeyItemPrefix + lesson1 + KeyStateSuffix},
		},
		{
			"lesson completed, one exercise passed", halfDone,
			map[string]string{
				KeyItemPrefix + lesson1 + KeyStateSuffix: doneTooltip,
				KeyExercisePrefix + exShade:              passedTooltip,
				KeyItemPrefix + lesson2 + KeyStateSuffix: lockedTooltip,
				WorksheetKey(1) + KeyStateSuffix:         lockedTooltip,
			},
			[]string{KeyExercisePrefix + exName, WorksheetKey(0) + KeyStateSuffix},
		},
		{
			"the first worksheet passed unlocks the next lesson", func(p *progress.Progress) {
				halfDone(p)
				p.Exercises[exName] = progress.ExerciseProgress{State: progress.StatePassed, Attempts: 1}
			},
			map[string]string{
				WorksheetKey(0) + KeyStateSuffix: passedTooltip,
				KeyExercisePrefix + exName:       passedTooltip,
				KeyExercisePrefix + exShade:      passedTooltip,
				WorksheetKey(1) + KeyStateSuffix: lockedTooltip,
			},
			[]string{KeyItemPrefix + lesson2 + KeyStateSuffix},
		},
		{
			"needs review with free navigation", func(p *progress.Progress) {
				p.Settings.FreeNavigation = true
				p.Exercises[exSum] = progress.ExerciseProgress{Attempts: 5}
			},
			map[string]string{
				KeyExercisePrefix + exSum:        reviewTooltip,
				WorksheetKey(1) + KeyStateSuffix: reviewTooltip,
			},
			[]string{KeyItemPrefix + lesson2 + KeyStateSuffix, KeyItemPrefix + lesson1 + KeyStateSuffix, WorksheetKey(0) + KeyStateSuffix},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pump(t, config{size: size(tallWindow), setup: install1(tt.progress)})
			waitShelf(t, a)
			openCassette(t, a, fractionsID)
			for k, tip := range tt.marks {
				if !hasMark(a, k, tip) {
					t.Errorf("%s does not show the mark %q", k, tip)
				}
			}
			for _, k := range tt.unmarked {
				for _, tip := range []string{doneTooltip, lockedTooltip, passedTooltip, reviewTooltip} {
					if hasMark(a, k, tip) {
						t.Errorf("%s shows the mark %q", k, tip)
					}
				}
			}
			// Every item of the cassette has its row, the worksheets one row each.
			for _, k := range []string{KeyItemPrefix + lesson1, WorksheetKey(0), KeyItemPrefix + lesson2, WorksheetKey(1),
				KeyExercisePrefix + exShade, KeyExercisePrefix + exName, KeyExercisePrefix + exSum} {
				if !a.HasKey(k) {
					t.Errorf("no row or label for %s", k)
				}
			}
			// The labels of the exercises are on the chips.
			for _, text := range []string{"1", "B2", "3", "What is a fraction?", adding, "Worksheet 1", "Worksheet 2", "Continue"} {
				if !has(a, text) {
					t.Errorf("the screen does not show %q", text)
				}
			}
		})
	}
}

func TestCassetteScreenContinue(t *testing.T) {
	tests := []struct {
		name     string
		progress func(p *progress.Progress)
		want     string
	}{
		{"a new student starts at the first lesson", nil, lessonSoon},
		{"the last item wins when it is open", func(p *progress.Progress) {
			halfDone(p)
			p.LastItem = lesson1
		}, lessonSoon},
		{"without a last item the first open item", halfDone, worksheet1Soon},
		{"a last item that is locked is ignored", func(p *progress.Progress) {
			halfDone(p)
			p.LastItem = lesson2
		}, worksheet1Soon},
		{"the last exercise opens its worksheet", func(p *progress.Progress) {
			halfDone(p)
			p.Exercises[exName] = progress.ExerciseProgress{State: progress.StatePassed}
			p.LastItem = exName
		}, worksheet1Soon},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pump(t, config{size: size(tallWindow), setup: install1(tt.progress)})
			waitShelf(t, a)
			openCassette(t, a, fractionsID)
			a.TapKey(KeyContinue)
			a.WaitForText(t, tt.want)
		})
	}
}

func TestCassetteScreenContinueFollowsTheFirstOpenItemWhenTheLessonsAreDone(t *testing.T) {
	a := pump(t, config{size: size(tallWindow), setup: install1(func(p *progress.Progress) {
		halfDone(p)
		p.Exercises[exName] = progress.ExerciseProgress{State: progress.StatePassed}
		p.Lessons[lesson2] = progress.LessonProgress{CompletedAt: completedAt}
	})})
	waitShelf(t, a)
	openCassette(t, a, fractionsID)
	a.TapKey(KeyContinue)
	a.WaitForText(t, worksheet2Soon)
}

func TestCassetteScreenLockedItemsOnlySayWait(t *testing.T) {
	for _, k := range []string{KeyItemPrefix + lesson2, WorksheetKey(0), WorksheetKey(1)} {
		t.Run(k, func(t *testing.T) {
			a := pump(t, config{size: size(tallWindow), setup: install1(nil)})
			waitShelf(t, a)
			openCassette(t, a, fractionsID)
			a.TapKey(k)
			a.WaitForText(t, lockedMessage)
			if has(a, lessonSoon) || has(a, worksheet2Soon) || has(a, "Adding fractions will open here soon.") {
				t.Error("a locked item opened")
			}
			if !a.HasKey(KeyContinue) {
				t.Error("the cassette screen was left")
			}
			time.Sleep(10 * time.Millisecond)
			p, err := a.Env.Progress.Load(context.Background(), "mia", fractionsID)
			if err != nil {
				t.Fatal(err)
			}
			if p.LastItem != "" {
				t.Errorf("last_item = %q after tapping a locked item", p.LastItem)
			}
		})
	}
}

func TestCassetteScreenOpensItemsAndRemembersTheLastOne(t *testing.T) {
	tests := []struct {
		name string
		tap  string
		want string
		last string
	}{
		{"a lesson", KeyItemPrefix + lesson1, lessonSoon, lesson1},
		{"a worksheet", WorksheetKey(0), worksheet1Soon, exShade},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pump(t, config{size: size(tallWindow), setup: install1(halfDone)})
			waitShelf(t, a)
			openCassette(t, a, fractionsID)
			a.TapKey(tt.tap)
			a.WaitForText(t, tt.want)
			waitProgress(t, a, fractionsID, func(p *progress.Progress) bool { return p.LastItem == tt.last })
		})
	}
}

// TestCassetteScreenGoesOnPastAWorksheetThatWaitsForAParent: when the attempts at
// an exercise run out, the child is not stopped; the next lesson opens, the
// exercise is marked as waiting for a parent, and Continue leads on.
func TestCassetteScreenGoesOnPastAWorksheetThatWaitsForAParent(t *testing.T) {
	waiting := func(p *progress.Progress) {
		halfDone(p)
		p.Exercises[exShade] = progress.ExerciseProgress{Attempts: 3}
		p.Exercises[exName] = progress.ExerciseProgress{State: progress.StatePassed, Attempts: 1}
	}
	a := pump(t, config{size: size(tallWindow), setup: install1(waiting)})
	waitShelf(t, a)
	openCassette(t, a, fractionsID)
	if !hasMark(a, KeyExercisePrefix+exShade, reviewTooltip) || !hasMark(a, WorksheetKey(0)+KeyStateSuffix, reviewTooltip) {
		t.Error("the exercise that waits for a parent is not marked")
	}
	if hasMark(a, KeyItemPrefix+lesson2+KeyStateSuffix, lockedTooltip) {
		t.Error("the lesson after the worksheet is locked")
	}
	a.TapKey(KeyItemPrefix + lesson2)
	a.WaitForText(t, `The lesson "Adding fractions" will open here soon.`)
	if has(a, lockedMessage) {
		t.Error("the lesson after the worksheet says it is locked")
	}
}

func TestCassetteScreenReadsTheProgressAgainWhenTheItemCloses(t *testing.T) {
	a := pump(t, config{size: size(tallWindow), setup: install1(nil)})
	waitShelf(t, a)
	openCassette(t, a, fractionsID)
	if hasMark(a, KeyItemPrefix+lesson1, doneTooltip) {
		t.Fatal("the lesson is done before it was played")
	}
	a.TapKey(KeyItemPrefix + lesson1)
	a.WaitForText(t, lessonSoon)
	waitProgress(t, a, fractionsID, func(p *progress.Progress) bool { return p.LastItem == lesson1 })
	setProgress(t, a.Env, fractionsID, func(p *progress.Progress) {
		p.Lessons[lesson1] = progress.LessonProgress{CompletedAt: completedAt, Plays: 1}
	})
	a.PopRoute()
	a.WaitFor(t, "the lesson to be done", func() bool { return hasMark(a, KeyItemPrefix+lesson1, doneTooltip) })
}

func TestCassetteScreenUsesTheScreensOfTheLaterSteps(t *testing.T) {
	var lesson LessonRequest
	var sheet WorksheetRequest
	a := pump(t, config{size: size(tallWindow), setup: install1(halfDone), screens: Screens{
		Lesson: func(r LessonRequest) m.Widget {
			lesson = r
			return plain("lesson player here")
		},
		Worksheet: func(r WorksheetRequest) m.Widget {
			sheet = r
			return plain("worksheet here")
		},
	}})
	waitShelf(t, a)
	openCassette(t, a, fractionsID)
	a.TapKey(KeyItemPrefix + lesson1)
	a.WaitForText(t, "lesson player here")
	if lesson.Username != "mia" || lesson.LessonID != lesson1 || lesson.Cassette.Manifest.ID != fractionsID || lesson.FS == nil {
		t.Errorf("lesson request = %+v", lesson)
	}
	a.PopRoute()
	a.WaitFor(t, "the cassette screen", func() bool { return a.HasKey(KeyContinue) })
	a.TapKey(WorksheetKey(0))
	a.WaitForText(t, "worksheet here")
	if sheet.Username != "mia" || sheet.Worksheet.Index != 0 || len(sheet.Worksheet.ExerciseIDs) != 2 || sheet.FS == nil {
		t.Errorf("worksheet request = %+v", sheet)
	}
}

func TestLessonScreenCanOpenTheNextItem(t *testing.T) {
	tests := []struct {
		name string
		// lesson is the lesson to open and wantNext whether its request can open
		// a next item; the second lesson is followed by a worksheet.
		lesson   string
		progress func(p *progress.Progress)
		want     string
	}{
		{"the worksheet after lesson 1", lesson1, halfDone, "worksheet here"},
		{"locked: the lesson was not completed", lesson1, func(p *progress.Progress) {
			p.Lessons[lesson1] = progress.LessonProgress{}
		}, lockedMessage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lesson LessonRequest
			var lessonCtx m.BuildContext
			a := pump(t, config{size: size(tallWindow), setup: install1(tt.progress), screens: Screens{
				Lesson: func(r LessonRequest) m.Widget {
					lesson = r
					return m.NewBuilder(func(bc m.BuildContext) m.Widget {
						lessonCtx = bc
						return plain("lesson player here")
					})
				},
				Worksheet: func(WorksheetRequest) m.Widget { return plain("worksheet here") },
			}})
			waitShelf(t, a)
			openCassette(t, a, fractionsID)
			a.TapKey(KeyItemPrefix + tt.lesson)
			a.WaitForText(t, "lesson player here")
			if lesson.OpenNext == nil {
				t.Fatal("the request of a lesson that has a next item cannot open it")
			}
			lesson.OpenNext(lessonCtx)
			a.WaitForText(t, tt.want)
			if tt.want == lockedMessage && !a.HasKey(KeyContinue) {
				t.Error("a locked next item did not leave the cassette screen open")
			}
		})
	}
}

func TestWorksheetScreenCanOpenTheNextItem(t *testing.T) {
	// The fixture is: lesson 1, worksheet 1 (two exercises), lesson 2, worksheet 2
	// (one exercise), which ends the cassette.
	allButTheLast := func(p *progress.Progress) {
		halfDone(p)
		p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StateNeedsReview, Attempts: 3}
		p.Exercises[exName] = progress.ExerciseProgress{State: progress.StatePassed, Attempts: 1, PassedBy: progress.PassedByAI}
		p.Lessons[lesson2] = progress.LessonProgress{CompletedAt: completedAt, Plays: 1}
	}
	tests := []struct {
		name      string
		worksheet int
		progress  func(p *progress.Progress)
		wantNext  bool
		want      string
	}{
		{"lesson 2 follows worksheet 1", 0, allButTheLast, true, `The lesson "Adding fractions" will open here soon.`},
		{"worksheet 2 ends the cassette", 1, allButTheLast, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sheet WorksheetRequest
			var sheetCtx m.BuildContext
			a := pump(t, config{size: size(tallWindow), setup: install1(tt.progress), screens: Screens{
				Worksheet: func(r WorksheetRequest) m.Widget {
					sheet = r
					return m.NewBuilder(func(bc m.BuildContext) m.Widget {
						sheetCtx = bc
						return plain("worksheet here")
					})
				},
			}})
			waitShelf(t, a)
			openCassette(t, a, fractionsID)
			a.TapKey(WorksheetKey(tt.worksheet))
			a.WaitForText(t, "worksheet here")
			if sheet.Worksheet.Index != tt.worksheet {
				t.Fatalf("opened worksheet %d, want %d", sheet.Worksheet.Index, tt.worksheet)
			}
			if (sheet.OpenNext != nil) != tt.wantNext {
				t.Fatalf("OpenNext set = %v, want %v", sheet.OpenNext != nil, tt.wantNext)
			}
			if !tt.wantNext {
				return
			}
			sheet.OpenNext(sheetCtx)
			a.WaitForText(t, tt.want)
		})
	}
}

func TestItemAfter(t *testing.T) {
	c := &cassette.Cassette{Manifest: cassette.Manifest{Items: []cassette.Item{
		{Type: cassette.ItemLesson, ID: "a"},
		{Type: cassette.ItemExercise, ID: "b"},
		{Type: cassette.ItemLesson, ID: "c"},
	}}}
	tests := []struct {
		id     string
		want   string
		wantOK bool
	}{
		{"a", "b", true},
		{"b", "c", true},
		{"c", "", false}, // the last item: the player says Done
		{"zzz", "", false},
	}
	for _, tt := range tests {
		got, ok := itemAfter(c, tt.id)
		if ok != tt.wantOK || got.ID != tt.want {
			t.Errorf("itemAfter(%q) = %q, %v; want %q, %v", tt.id, got.ID, ok, tt.want, tt.wantOK)
		}
	}
}

func TestCassetteScreenMergesAnOlderSaveFile(t *testing.T) {
	tests := []struct {
		name        string
		saved       string
		wantOrphan  bool
		wantVersion string
	}{
		{"older save file is merged", "0.9.0", true, "1.0.0"},
		{"newer save file is left alone", "2.0.0", false, "2.0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pump(t, config{size: size(tallWindow), setup: install1(func(p *progress.Progress) {
				v, err := cassette.ParseVersion(tt.saved)
				if err != nil {
					t.Fatal(err)
				}
				p.CassetteVersion = v
				p.Lessons[lesson1] = progress.LessonProgress{CompletedAt: completedAt}
				p.Lessons["gone-lesson"] = progress.LessonProgress{CompletedAt: completedAt}
			})})
			waitShelf(t, a)
			openCassette(t, a, fractionsID)
			if !hasMark(a, KeyItemPrefix+lesson1, doneTooltip) {
				t.Error("the completed lesson is not done after the merge")
			}
			p, err := a.Env.Progress.Load(context.Background(), "mia", fractionsID)
			if err != nil {
				t.Fatal(err)
			}
			if got := p.CassetteVersion.String(); got != tt.wantVersion {
				t.Errorf("saved version = %s, want %s", got, tt.wantVersion)
			}
			if orphan := p.Orphaned != nil && len(p.Orphaned.Lessons) == 1; orphan != tt.wantOrphan {
				t.Errorf("orphaned = %+v, want an orphan: %v", p.Orphaned, tt.wantOrphan)
			}
		})
	}
}

func TestCassetteScreenFailsKindly(t *testing.T) {
	a := pump(t, config{size: size(tallWindow), setup: install1(nil)})
	waitShelf(t, a)
	removeLessons(t, a.Env, fractionsID)
	a.TapKey(KeyCardPrefix + fractionsID)
	a.WaitForText(t, "This cassette could not be opened. Ask a parent.")
	if a.HasKey(KeyContinue) {
		t.Error("the Continue button is shown for a cassette that cannot be opened")
	}
}

func TestCassetteScreenGreek(t *testing.T) {
	a := pump(t, config{locale: "el_GR.UTF-8", size: size(tallWindow), setup: install1(halfDone)})
	waitShelf(t, a)
	openCassette(t, a, fractionsID)
	for _, text := range []string{"Συνέχεια", "Φύλλο ασκήσεων 1", "Φύλλο ασκήσεων 2", "Μάθημα", "2 από 5 έτοιμα"} {
		if !has(a, text) {
			t.Errorf("the Greek cassette screen does not show %q", text)
		}
	}
	if !hasMark(a, KeyItemPrefix+lesson2, "Κλειδωμένο") {
		t.Error("the locked lesson has no Greek tooltip")
	}
	a.TapKey(KeyItemPrefix + lesson2)
	a.WaitForText(t, "Τελείωσε πρώτα όσα είναι πριν από αυτό.")
}

func TestCassetteGoldens(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		change func(p *progress.Progress)
	}{
		{"cassette_en", "en_US.UTF-8", halfDone},
		{"cassette_el", "el_GR.UTF-8", halfDone},
		{"cassette_review_en", "en_US.UTF-8", func(p *progress.Progress) {
			halfDone(p)
			p.Exercises[exName] = progress.ExerciseProgress{State: progress.StatePassed}
			p.Lessons[lesson2] = progress.LessonProgress{CompletedAt: completedAt}
			p.Exercises[exSum] = progress.ExerciseProgress{Attempts: 5}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pump(t, config{locale: tt.locale, size: size(750), setup: install1(tt.change)})
			waitShelf(t, a)
			openCassette(t, a, fractionsID)
			a.WaitFor(t, "the cover", func() bool { return !a.Exists(gluttertest.ByIcon(m.Icons.MenuBook())) })
			a.MatchesGolden(tt.name)
		})
	}
}

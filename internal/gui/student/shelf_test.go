package student

import (
	"context"
	"testing"
	"time"

	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
)

const otherID = "org.example.second-2"

// twoCassettes installs the fixture cassette and a second one, with progress on
// the first: lesson 1 completed and one exercise passed (2 of 5).
func twoCassettes(t *testing.T, env *app.Env) {
	install(t, env, fixture(t, fractionsID, "1.0.0", fractionsTitle))
	install(t, env, fixture(t, otherID, "1.0.0", "Second steps"))
	setProgress(t, env, fractionsID, func(p *progress.Progress) {
		p.Lessons[lesson1] = progress.LessonProgress{CompletedAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), Plays: 1}
		p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StatePassed, Attempts: 1, PassedBy: progress.PassedByAI}
	})
}

func TestShelfShowsTheInstalledCassettes(t *testing.T) {
	a := pump(t, config{setup: twoCassettes})
	waitShelf(t, a)
	for _, text := range []string{"Hello, Mia!", "My cassettes", fractionsTitle, "Second steps", "math", "Import a cassette", "Log out"} {
		if !has(a, text) {
			t.Errorf("the shelf does not show %q", text)
		}
	}
	if !has(a, "2 of 5 done") || !has(a, "0 of 5 done") {
		t.Error("the shelf does not show the progress of both cassettes")
	}
	for _, id := range []string{fractionsID, otherID} {
		if !a.HasKey(KeyCardPrefix+id) || !a.HasKey(KeyProgressPrefix+id) {
			t.Errorf("no card or progress bar for %s", id)
		}
	}
	if a.Exists(gluttertest.ByType[*m.CircularProgressIndicator]()) {
		t.Error("the shelf still shows the loading spinner")
	}
}

func TestShelfFiltersCassettes(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, env *app.Env)
		shown []string
		gone  []string
		text  string
	}{
		{"hidden from this student", func(t *testing.T, env *app.Env) {
			twoCassettes(t, env)
			hide(t, env, otherID)
		}, []string{fractionsTitle}, []string{"Second steps"}, ""},
		{"hidden from another student only", func(t *testing.T, env *app.Env) {
			twoCassettes(t, env)
			err := env.Settings.Update(context.Background(), func(s *settings.Settings) {
				s.HiddenCassettes["leo"] = []string{otherID}
			})
			if err != nil {
				t.Fatal(err)
			}
		}, []string{fractionsTitle, "Second steps"}, nil, ""},
		{"broken cassette is not shown to a student", func(t *testing.T, env *app.Env) {
			twoCassettes(t, env)
			breakCassette(t, env, otherID)
		}, []string{fractionsTitle}, []string{"Second steps"}, ""},
		{"nothing installed", nil, nil, []string{fractionsTitle}, "There are no cassettes yet. Ask a parent to add one."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pump(t, config{setup: tt.setup})
			if tt.text != "" {
				a.WaitForText(t, tt.text)
			} else {
				waitShelf(t, a)
			}
			for _, s := range tt.shown {
				if !has(a, s) {
					t.Errorf("%q is not shown", s)
				}
			}
			for _, s := range tt.gone {
				if has(a, s) {
					t.Errorf("%q is shown", s)
				}
			}
		})
	}
}

func TestShelfImportButtonFollowsTheSetting(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		err := env.Settings.Update(context.Background(), func(s *settings.Settings) { s.StudentsCanImport = false })
		if err != nil {
			t.Fatal(err)
		}
	}})
	a.WaitForText(t, "There are no cassettes yet. Ask a parent to add one.")
	if a.HasKey(KeyImport) {
		t.Error("the Import button is shown although students may not import")
	}
}

func TestShelfOpensTheCassetteAndReloadsOnReturn(t *testing.T) {
	a := pump(t, config{setup: twoCassettes})
	waitShelf(t, a)
	openCassette(t, a, fractionsID)
	if !has(a, fractionsTitle) {
		t.Fatal("the cassette screen has no title")
	}
	setProgress(t, a.Env, fractionsID, func(p *progress.Progress) {
		p.Exercises[exName] = progress.ExerciseProgress{State: progress.StatePassed, Attempts: 1}
	})
	a.PopRoute()
	a.WaitForText(t, "3 of 5 done")
}

func TestShelfLoadFailureIsShown(t *testing.T) {
	storagetest.SkipOnWindows(t, "a path through a file reports not found, not not-a-folder")
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		twoCassettes(t, env)
		// A save file the progress store cannot even quarantine: its folder is a file.
		breakProgressFolder(t, env)
	}})
	a.WaitForText(t, "The cassettes could not be shown. Ask a parent.")
}

func TestShelfGreek(t *testing.T) {
	a := pump(t, config{locale: "el_GR.UTF-8", setup: twoCassettes})
	waitShelf(t, a)
	for _, text := range []string{"Οι κασέτες μου", "Εισαγωγή κασέτας", "2 από 5 έτοιμα", "Αποσύνδεση"} {
		if !has(a, text) {
			t.Errorf("the Greek shelf does not show %q", text)
		}
	}
}

func TestShelfGoldens(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		setup  func(t *testing.T, env *app.Env)
	}{
		{"shelf_en", "en_US.UTF-8", twoCassettes},
		{"shelf_el", "el_GR.UTF-8", twoCassettes},
		{"shelf_empty_en", "en_US.UTF-8", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pump(t, config{locale: tt.locale, setup: tt.setup})
			if tt.setup == nil {
				a.WaitForText(t, "There are no cassettes yet. Ask a parent to add one.")
			} else {
				waitShelf(t, a)
			}
			a.MatchesGolden(tt.name)
		})
	}
}

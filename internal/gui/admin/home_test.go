package admin

import (
	"testing"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

func TestHomeShowsTheTiles(t *testing.T) {
	a := pump(t, config{})
	for _, k := range []string{KeyStudents, KeyCassettes, KeyProgress, KeySettings, KeyProviders, KeyPlayAsMe, app.KeyLogout} {
		if !a.HasKey(k) {
			t.Errorf("the home has no %s", k)
		}
	}
	for _, text := range []string{"Parent area", "Hello, Mum", "Students", "Cassettes", "Progress", "Settings", "AI providers", "Play as me",
		"0 waiting for review", "None is active"} {
		if !has(a, text) {
			t.Errorf("the home does not show %q", text)
		}
	}
}

func TestHomeCountsTheExercisesToReview(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		twoCassettes(t, env)
		for _, user := range []string{"mia", "mum"} {
			setProgress(t, env, user, fractionsID, func(p *progress.Progress) {
				p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StateNeedsReview, Attempts: 3}
			})
		}
		setProgress(t, env, "mia", otherID, func(p *progress.Progress) {
			p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StatePassed, Attempts: 1}
		})
	}})
	waitText(t, a, "2 waiting for review")
}

func TestHomeNavigation(t *testing.T) {
	tests := []struct {
		name string
		tile string
		want string
	}{
		{"students", KeyStudents, "Add"},
		{"cassettes", KeyCassettes, "Import a cassette"},
		{"progress placeholder", KeyProgress, "The progress and review screens are coming soon."},
		{"settings", KeySettings, "Students can import cassettes"},
		{"providers", KeyProviders, "Add a provider"},
		{"play as me", KeyPlayAsMe, "Hello, Mum!"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pump(t, config{})
			a.TapKey(tt.tile)
			waitText(t, a, tt.want)
			goBack(t, a)
			waitKey(t, a, KeyStudents)
		})
	}
}

func TestHomeProgressScreenHook(t *testing.T) {
	a := pump(t, config{screens: Screens{Progress: func() m.Widget {
		return m.NewScaffold(m.ScaffoldOpts{Body: m.NewCenter(m.NewText("progress hook"))})
	}}})
	a.TapKey(KeyProgress)
	waitText(t, a, "progress hook")
}

func TestHomeShowsTheActiveProvider(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		addProvider(t, env, "Home AI", "sk-1", true)
	}})
	if !has(a, "Active: Home AI (Local)") {
		t.Error("the Providers tile does not name the active provider")
	}
}

func TestHomeLogOut(t *testing.T) {
	a := pump(t, config{})
	a.TapKey(app.KeyLogout)
	waitKey(t, a, app.KeyProfilePrefix+"mia")
}

func TestHomeInGreek(t *testing.T) {
	a := pump(t, config{locale: "el_GR.UTF-8"})
	for _, text := range []string{"Περιοχή γονέα", "Μαθητές", "Κασέτες", "Πρόοδος", "Ρυθμίσεις", "Παίξε ως εγώ", "0 περιμένουν έλεγχο"} {
		if !has(a, text) {
			t.Errorf("the Greek home does not show %q", text)
		}
	}
}

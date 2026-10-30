package admin

import (
	"context"
	"net/http"
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/providers"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

// goldenCase is one screenshot: the setup, the way to the screen, and a wait
// that says the screen has loaded.
type goldenCase struct {
	name   string
	locale string
	setup  func(t *testing.T, env *app.Env)
	// open leads from the home to the screen.
	open func(t *testing.T, a *testutil.App)
}

func TestGoldens(t *testing.T) {
	withProviders := func(t *testing.T, env *app.Env) {
		addProvider(t, env, "Home Ollama", "", true)
		err := env.Providers.Add(context.Background(), providers.Config{Name: "OpenAI", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o", APIKey: "sk-1", Kind: providers.KindCloud})
		if err != nil {
			t.Fatal(err)
		}
	}
	tests := []goldenCase{
		{"home_en", "en_US.UTF-8", func(t *testing.T, env *app.Env) {
			twoCassettes(t, env)
			setProgress(t, env, "mia", fractionsID, func(p *progress.Progress) {
				p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StateNeedsReview, Attempts: 3}
			})
			addProvider(t, env, "Home Ollama", "", true)
		}, func(t *testing.T, a *testutil.App) { waitText(t, a, "1 waiting for review") }},
		{"home_el", "el_GR.UTF-8", nil, nil},
		{"students_en", "en_US.UTF-8", func(t *testing.T, env *app.Env) {
			testutil.AddProfile(t, env, "leo", "Leo", profile.RoleStudent, studentPassword)
			testutil.AddProfile(t, env, "dad", "Dad", profile.RoleAdmin, adminPassword)
		}, func(t *testing.T, a *testutil.App) { openStudents(t, a) }},
		{"students_el", "el_GR.UTF-8", nil, func(t *testing.T, a *testutil.App) { openStudents(t, a) }},
		{"students_add_en", "en_US.UTF-8", nil, func(t *testing.T, a *testutil.App) {
			openStudents(t, a)
			a.TapKey(KeyAddStudent)
			waitKey(t, a, KeyDialogOK)
			a.EnterKey(app.KeyUsername, "leo")
			a.EnterKey(app.KeyDisplayName, "Leo")
			a.EnterKey(app.KeyPassword, "abcd")
			a.EnterKey(app.KeyPasswordAgain, "abc")
			a.TapKey(KeyDialogOK)
			waitText(t, a, "The two passwords are not the same.")
		}},
		{"students_delete_en", "en_US.UTF-8", nil, func(t *testing.T, a *testutil.App) {
			openStudents(t, a)
			a.TapKey(KeyDeletePrefix + "mia")
			waitKey(t, a, KeyDialogOK)
		}},
		{"students_cassettes_en", "en_US.UTF-8", func(t *testing.T, env *app.Env) {
			twoCassettes(t, env)
			hideFromMia(t, env, otherID)
		}, func(t *testing.T, a *testutil.App) {
			openStudents(t, a)
			a.TapKey(KeyVisiblePrefix + "mia")
			waitKey(t, a, KeyShowPrefix+fractionsID)
		}},
		{"cassettes_en", "en_US.UTF-8", func(t *testing.T, env *app.Env) {
			twoCassettes(t, env)
			install(t, env, fixture(t, "org.example.bad-3", "1.0.0", "Bad one"))
			breakCassette(t, env, "org.example.bad-3")
		}, func(t *testing.T, a *testutil.App) {
			openCassettes(t, a)
			waitText(t, a, fractionsTitle)
		}},
		{"cassettes_el", "el_GR.UTF-8", twoCassettes, func(t *testing.T, a *testutil.App) {
			openCassettes(t, a)
			waitText(t, a, fractionsTitle)
		}},
		{"cassettes_remove_en", "en_US.UTF-8", twoCassettes, func(t *testing.T, a *testutil.App) {
			openCassettes(t, a)
			waitText(t, a, fractionsTitle)
			a.TapKey(KeyRemovePrefix + fractionsID)
			waitKey(t, a, KeyDeleteData)
		}},
		{"settings_en", "en_US.UTF-8", nil, func(t *testing.T, a *testutil.App) { openSettings(t, a) }},
		{"settings_el", "el_GR.UTF-8", nil, func(t *testing.T, a *testutil.App) { openSettings(t, a) }},
		{"providers_en", "en_US.UTF-8", withProviders, func(t *testing.T, a *testutil.App) { openProviders(t, a) }},
		{"providers_empty_en", "en_US.UTF-8", nil, func(t *testing.T, a *testutil.App) { openProviders(t, a) }},
		{"providers_el", "el_GR.UTF-8", withProviders, func(t *testing.T, a *testutil.App) { openProviders(t, a) }},
		{"providers_presets_en", "en_US.UTF-8", nil, func(t *testing.T, a *testutil.App) {
			openProviders(t, a)
			a.TapKey(KeyProviderAdd)
			waitKey(t, a, KeyPresetPrefix+"openai")
		}},
		{"provider_form_en", "en_US.UTF-8", nil, func(t *testing.T, a *testutil.App) {
			openProviders(t, a)
			a.TapKey(KeyProviderAdd)
			waitKey(t, a, KeyPresetPrefix+"openai")
			a.TapKey(KeyPresetPrefix + "openai")
			waitKey(t, a, KeyProviderName)
		}},
		{"provider_form_el", "el_GR.UTF-8", nil, func(t *testing.T, a *testutil.App) {
			openProviders(t, a)
			a.TapKey(KeyProviderAdd)
			waitKey(t, a, KeyPresetPrefix+"ollama")
			a.TapKey(KeyPresetPrefix + "ollama")
			waitKey(t, a, KeyProviderName)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pump(t, config{locale: tt.locale, setup: tt.setup})
			if tt.open != nil {
				tt.open(t, a)
			}
			a.PumpAndSettle()
			a.MatchesGolden(tt.name)
		})
	}

	// The result of a Test: the server is the test's own. A good result shows
	// the time it took, which changes from run to run, so only failures have
	// goldens.
	for _, c := range []struct {
		name   string
		locale string
		reply  func(w http.ResponseWriter, n int)
	}{
		{"provider_test_failed_en", "en_US.UTF-8", func(w http.ResponseWriter, _ int) { http.Error(w, "no", http.StatusNotFound) }},
		{"provider_test_auth_el", "el_GR.UTF-8", func(w http.ResponseWriter, _ int) { http.Error(w, "no", http.StatusUnauthorized) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			srv := chatServer(t, c.reply)
			a := pump(t, config{locale: c.locale, setup: func(t *testing.T, env *app.Env) {
				addServerProvider(t, env, "Test AI", srv.URL, grading.StructuredJSONSchema)
			}})
			openProviders(t, a)
			openTest(t, a, "Test AI")
			a.PumpAndSettle()
			a.MatchesGolden(c.name)
		})
	}
}

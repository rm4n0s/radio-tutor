package app_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/glutter/services"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

const (
	adminPassword   = "parent-secret"
	studentPassword = "abcd"
)

// familyOptions makes an admin ("mum", display name "Mum") and a student
// ("mia", display name "Mia") before the window opens.
func familyOptions(t *testing.T, locale string) testutil.Options {
	return testutil.Options{Locale: locale, Before: func(env *app.Env) {
		testutil.AddProfile(t, env, "mum", "Mum", profile.RoleAdmin, adminPassword)
		testutil.AddProfile(t, env, "mia", "Mia", profile.RoleStudent, studentPassword)
	}}
}

func TestFirstRunCreatesAdmin(t *testing.T) {
	a := testutil.Pump(t, testutil.Options{})
	for _, text := range []string{"Create the parent account", "Create account"} {
		if !a.Exists(gluttertest.ByText(text)) {
			t.Fatalf("the first-run screen does not show %q", text)
		}
	}
	if a.Exists(gluttertest.ByText("Log out")) {
		t.Fatal("the app is logged in before the account exists")
	}
	a.EnterKey(app.KeyUsername, "Mum")
	a.EnterKey(app.KeyDisplayName, "Mum Ragia")
	a.EnterKey(app.KeyPassword, adminPassword)
	a.EnterKey(app.KeyPasswordAgain, adminPassword)
	a.TapKey(app.KeySubmit)
	a.WaitForText(t, "Hello, Mum Ragia")

	profiles := a.Env.Profiles.List()
	if len(profiles) != 1 || profiles[0].Username != "mum" || profiles[0].Role != profile.RoleAdmin || profiles[0].DisplayName != "Mum Ragia" {
		t.Fatalf("profiles = %+v, want the admin mum", profiles)
	}
	if _, err := a.Env.Profiles.Login("mum", adminPassword); err != nil {
		t.Errorf("the new admin cannot log in: %v", err)
	}
	if _, err := os.Stat(a.Paths.ProfilesFile()); err != nil {
		t.Errorf("profiles.json was not written: %v", err)
	}
}

func TestFirstRunErrors(t *testing.T) {
	tests := []struct {
		name                                   string
		username, display, password, again, er string
	}{
		{"passwords differ", "mum", "Mum", adminPassword, adminPassword + "x", "The two passwords are not the same."},
		{"password too short", "mum", "Mum", "short", "short", "The password needs at least 8 characters."},
		{"username invalid", "M", "Mum", adminPassword, adminPassword, "The username needs 2 to 32 characters: letters a-z, digits and _."},
		{"display name empty", "mum", "  ", adminPassword, adminPassword, "The name shown needs 1 to 40 characters."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := testutil.Pump(t, testutil.Options{})
			a.EnterKey(app.KeyUsername, tt.username)
			a.EnterKey(app.KeyDisplayName, tt.display)
			a.EnterKey(app.KeyPassword, tt.password)
			a.EnterKey(app.KeyPasswordAgain, tt.again)
			a.TapKey(app.KeySubmit)
			a.WaitForText(t, tt.er)
			if got := len(a.Env.Profiles.List()); got != 0 {
				t.Errorf("%d profiles were made", got)
			}
			if !a.Exists(gluttertest.ByText("Create the parent account")) {
				t.Error("the first-run screen is gone")
			}
		})
	}
}

func TestFirstRunUsernameTaken(t *testing.T) {
	// A profile file with only a student has no admin, so the first-run screen
	// shows and creating an admin with the student's username must be refused.
	opts := testutil.Options{Before: func(env *app.Env) {
		testutil.AddProfile(t, env, "mia", "Mia", profile.RoleStudent, studentPassword)
	}}
	a := testutil.Pump(t, opts)
	if !a.Exists(gluttertest.ByText("Create the parent account")) {
		t.Fatal("a store without an admin must show the first-run screen")
	}
	a.EnterKey(app.KeyUsername, "mia")
	a.EnterKey(app.KeyDisplayName, "Mia")
	a.EnterKey(app.KeyPassword, adminPassword)
	a.EnterKey(app.KeyPasswordAgain, adminPassword)
	a.TapKey(app.KeySubmit)
	a.WaitForText(t, "This username is already used.")
}

func TestLoginStudent(t *testing.T) {
	a := testutil.Pump(t, familyOptions(t, ""))
	for _, text := range []string{"Who is learning today?", "Mum", "Mia"} {
		if !a.Exists(gluttertest.ByText(text)) {
			t.Fatalf("the login screen does not show %q", text)
		}
	}
	a.TapKey(app.KeyProfilePrefix + "mia")
	if !a.Exists(gluttertest.ByText("Hello, Mia! Type your password.")) {
		t.Fatal("no password step for Mia")
	}

	// A wrong password.
	a.EnterKey(app.KeyPassword, "wrong")
	a.TapKey(app.KeySubmit)
	a.WaitForText(t, "That is not the right password. Try again.")
	if a.Exists(gluttertest.ByText("My cassettes")) {
		t.Fatal("a wrong password logged in")
	}

	// The right one, sent with the Enter key of the keyboard.
	a.EnterKey(app.KeyPassword, studentPassword)
	a.ShowKeyboard(gluttertest.ByKey(m.NewValueKey(app.KeyPassword)))
	a.SendKeyEvent(services.LogicalKeyboardKeyEnter)
	a.WaitForText(t, "Hello, Mia!")
	if !a.Exists(gluttertest.ByText("My cassettes")) || a.Exists(gluttertest.ByText("Parent area")) {
		t.Error("the student did not get the student home")
	}
}

func TestLoginAdminAndBack(t *testing.T) {
	a := testutil.Pump(t, familyOptions(t, ""))
	a.TapKey(app.KeyProfilePrefix + "mum")
	a.TapKey(app.KeyBack)
	if !a.Exists(gluttertest.ByText("Who is learning today?")) {
		t.Fatal("Back did not return to the names")
	}
	a.TapKey(app.KeyProfilePrefix + "mum")
	a.EnterKey(app.KeyPassword, adminPassword)
	a.TapKey(app.KeySubmit)
	a.WaitForText(t, "Hello, Mum")
	if !a.Exists(gluttertest.ByText("Parent area")) {
		t.Error("the admin did not get the parent home")
	}
}

func TestLoginThrottle(t *testing.T) {
	a := testutil.Pump(t, familyOptions(t, ""))
	a.TapKey(app.KeyProfilePrefix + "mia")
	for i := 0; i < 5; i++ {
		a.EnterKey(app.KeyPassword, "nope")
		a.TapKey(app.KeySubmit)
		a.WaitForText(t, "That is not the right password. Try again.")
	}
	// The fifth failure started the wait: even the right password is refused.
	a.EnterKey(app.KeyPassword, studentPassword)
	a.TapKey(app.KeySubmit)
	a.WaitFor(t, "the throttle message", func() bool {
		return a.Exists(gluttertest.ByTextContaining("Too many wrong tries. Wait ")) &&
			a.Exists(gluttertest.ByTextContaining(" seconds and try again."))
	})
	if a.Exists(gluttertest.ByText("My cassettes")) {
		t.Fatal("logged in during the wait")
	}
}

func TestLogoutReturnsToLogin(t *testing.T) {
	a := testutil.Pump(t, familyOptions(t, ""))
	a.TapKey(app.KeyProfilePrefix + "mia")
	a.EnterKey(app.KeyPassword, studentPassword)
	a.TapKey(app.KeySubmit)
	a.WaitForText(t, "Hello, Mia!")

	// A screen pushed on top (the cassette, the lesson...) is closed by Log out too.
	home := a.Element(gluttertest.ByType[*app.StudentHome]())
	app.Push(home, func(m.BuildContext) m.Widget {
		return m.NewScaffold(m.ScaffoldOpts{Body: m.NewText("A pushed screen")})
	})
	a.PumpAndSettle()
	if !a.Exists(gluttertest.ByText("A pushed screen")) {
		t.Fatal("the screen was not pushed")
	}
	if !app.CanPop(a.Element(gluttertest.ByText("A pushed screen"))) {
		t.Error("CanPop is false with a screen below")
	}
	app.Pop(a.Element(gluttertest.ByText("A pushed screen")))
	a.PumpAndSettle()
	if a.Exists(gluttertest.ByText("A pushed screen")) {
		t.Fatal("Pop did not close the screen")
	}
	app.Push(a.Element(gluttertest.ByType[*app.StudentHome]()), func(m.BuildContext) m.Widget {
		return m.NewScaffold(m.ScaffoldOpts{Body: m.NewText("Another pushed screen"), AppBar: m.NewAppBar(m.AppBarOpts{
			Actions: []m.Widget{m.NewTextButton(func() {}, m.NewText("x"))},
		})})
	})
	a.PumpAndSettle()
	scope := app.ScopeOf(a.Element(gluttertest.ByText("Another pushed screen")))
	scope.LogOut(a.Element(gluttertest.ByText("Another pushed screen")))
	a.PumpAndSettle()
	if a.Exists(gluttertest.ByText("Another pushed screen")) || a.Exists(gluttertest.ByText("My cassettes")) {
		t.Fatal("Log out left a screen open")
	}
	if !a.Exists(gluttertest.ByText("Who is learning today?")) {
		t.Fatal("Log out did not show the login screen")
	}
	// The password step does not keep the previous user.
	if a.HasKey(app.KeyPassword) {
		t.Error("the login screen starts on a password step")
	}
}

func TestLogoutButton(t *testing.T) {
	for _, role := range []string{"mum", "mia"} {
		t.Run(role, func(t *testing.T) {
			a := testutil.Pump(t, familyOptions(t, ""))
			a.TapKey(app.KeyProfilePrefix + role)
			password := adminPassword
			if role == "mia" {
				password = studentPassword
			}
			a.EnterKey(app.KeyPassword, password)
			a.TapKey(app.KeySubmit)
			a.WaitFor(t, "the home", func() bool { return a.HasKey(app.KeyLogout) })
			a.TapKey(app.KeyLogout)
			if !a.Exists(gluttertest.ByText("Who is learning today?")) {
				t.Fatal("Log out did not show the login screen")
			}
		})
	}
}

func TestLanguageSwitch(t *testing.T) {
	a := testutil.Pump(t, familyOptions(t, "en_US.UTF-8"))
	if !a.Exists(gluttertest.ByText("Who is learning today?")) {
		t.Fatal("not English at the start")
	}
	a.TapKey(app.KeyLanguagePrefix + "el")
	for _, text := range []string{"Ποιος μαθαίνει σήμερα;", "Mum", "Mia"} {
		if !a.Exists(gluttertest.ByText(text)) {
			t.Fatalf("the Greek login screen does not show %q", text)
		}
	}
	a.WaitFor(t, "the language to be saved", func() bool { return a.Env.Settings.Get().Language == i18n.EL })
	if data, err := os.ReadFile(a.Paths.SettingsFile()); err != nil || !strings.Contains(string(data), `"language": "el"`) {
		t.Errorf("settings.json = %q, %v", data, err)
	}

	// The Greek UI carries on after the login, and back to English.
	a.TapKey(app.KeyProfilePrefix + "mia")
	if !a.Exists(gluttertest.ByText("Mia, γράψε τον κωδικό σου.")) {
		t.Fatal("the password step is not in Greek")
	}
	a.EnterKey(app.KeyPassword, studentPassword)
	a.TapKey(app.KeySubmit)
	a.WaitForText(t, "Γεια σου, Mia!")
	a.TapKey(app.KeyLogout)
	a.TapKey(app.KeyLanguagePrefix + "en")
	if !a.Exists(gluttertest.ByText("Who is learning today?")) {
		t.Fatal("did not switch back to English")
	}
	a.WaitFor(t, "English to be saved", func() bool { return a.Env.Settings.Get().Language == i18n.EN })
}

func TestLanguageFromLocale(t *testing.T) {
	tests := []struct {
		locale string
		want   string
	}{
		{"el_GR.UTF-8", "Ποιος μαθαίνει σήμερα;"},
		{"en_GB.UTF-8", "Who is learning today?"},
		{"C", "Who is learning today?"},
	}
	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			a := testutil.Pump(t, familyOptions(t, tt.locale))
			if !a.Exists(gluttertest.ByText(tt.want)) {
				t.Errorf("the login screen does not show %q", tt.want)
			}
		})
	}
}

func TestSavedLanguageBeatsLocale(t *testing.T) {
	env, paths := testutil.NewEnv(t, testutil.Options{Locale: "en_US.UTF-8", Before: func(env *app.Env) {
		testutil.AddProfile(t, env, "mum", "Mum", profile.RoleAdmin, adminPassword)
		if err := env.Settings.SetLanguage(context.Background(), i18n.EL); err != nil {
			t.Fatal(err)
		}
	}})
	a := testutil.PumpEnv(t, env, paths, m.Size{})
	if !a.Exists(gluttertest.ByText("Ποιος μαθαίνει σήμερα;")) {
		t.Error("the saved language was not used")
	}
}

func TestNoticesShownToTheAdmin(t *testing.T) {
	env, paths := testutil.NewEnv(t, testutil.Options{})
	env.Notices = append(env.Notices, i18n.M("notice.profiles_broken", "path", "/data/profiles.json.broken-x"))
	a := testutil.PumpEnv(t, env, paths, m.Size{})
	want := "The profiles file could not be read. It was kept as /data/profiles.json.broken-x and Radio Tutor started with no profiles."
	if !a.Exists(gluttertest.ByText(want)) {
		t.Fatal("the first-run screen does not show the notice")
	}
	a.EnterKey(app.KeyUsername, "mum")
	a.EnterKey(app.KeyDisplayName, "Mum")
	a.EnterKey(app.KeyPassword, adminPassword)
	a.EnterKey(app.KeyPasswordAgain, adminPassword)
	a.TapKey(app.KeySubmit)
	a.WaitForText(t, "Hello, Mum")
	if !a.Exists(gluttertest.ByText(want)) {
		t.Fatal("the admin home does not show the notice")
	}
}

func TestAlreadyRunningWindow(t *testing.T) {
	testutil.UseLinux(t)
	tests := []struct {
		lang  i18n.Lang
		title string
		ok    string
	}{
		{i18n.EN, "Radio Tutor is already running", "OK"},
		{i18n.EL, "Το Radio Tutor εκτελείται ήδη", "Εντάξει"},
	}
	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			ts := gluttertest.New(t, gluttertest.Opts{Size: app.AlreadyRunningSize})
			ts.PumpWidget(app.NewAlreadyRunning(tt.lang))
			ts.PumpAndSettle()
			for _, text := range []string{tt.title, tt.ok} {
				if !ts.Exists(gluttertest.ByText(text)) {
					t.Fatalf("the window does not show %q", text)
				}
			}
			if ts.ExitRequests() != 0 {
				t.Fatal("the app exits before OK")
			}
			ts.MatchesGolden("alreadyrunning_" + string(tt.lang))
			ts.Tap(gluttertest.ByKey(m.NewValueKey(app.KeyAlreadyRunningOK)))
			ts.Pump(100 * time.Millisecond)
			if ts.ExitRequests() != 1 {
				t.Errorf("exit requests = %d, want 1", ts.ExitRequests())
			}
		})
	}
}

func TestGoldens(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		setup  func(t *testing.T, a *testutil.App)
		family bool
	}{
		{"firstrun_en", "en_US.UTF-8", nil, false},
		{"firstrun_el", "el_GR.UTF-8", nil, false},
		{"login_en", "en_US.UTF-8", nil, true},
		{"login_el", "el_GR.UTF-8", nil, true},
		{"login_password_en", "en_US.UTF-8", func(t *testing.T, a *testutil.App) {
			a.TapKey(app.KeyProfilePrefix + "mia")
			a.EnterKey(app.KeyPassword, "wrong")
			a.TapKey(app.KeySubmit)
			a.WaitForText(t, "That is not the right password. Try again.")
		}, true},
		{"login_password_el", "el_GR.UTF-8", func(t *testing.T, a *testutil.App) {
			a.TapKey(app.KeyProfilePrefix + "mia")
			a.EnterKey(app.KeyPassword, "wrong")
			a.TapKey(app.KeySubmit)
			a.WaitForText(t, "Αυτός δεν είναι ο σωστός κωδικός. Δοκίμασε ξανά.")
		}, true},
		{"admin_home_en", "en_US.UTF-8", func(t *testing.T, a *testutil.App) {
			a.Env.Notices = append(a.Env.Notices, i18n.M("notice.profiles_broken", "path", "/data/profiles.json.broken-20261030T101500Z"))
			logIn(t, a, "mum", adminPassword)
		}, true},
		{"student_home_el", "el_GR.UTF-8", func(t *testing.T, a *testutil.App) { logIn(t, a, "mia", studentPassword) }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := testutil.Options{Locale: tt.locale}
			if tt.family {
				opts = familyOptions(t, tt.locale)
			}
			a := testutil.Pump(t, opts)
			if tt.setup != nil {
				tt.setup(t, a)
			}
			a.MatchesGolden(tt.name)
		})
	}
}

// logIn logs in as the user from the login screen.
func logIn(t *testing.T, a *testutil.App, username, password string) {
	t.Helper()
	a.TapKey(app.KeyProfilePrefix + username)
	a.EnterKey(app.KeyPassword, password)
	a.TapKey(app.KeySubmit)
	a.WaitFor(t, "the home", func() bool { return a.HasKey(app.KeyLogout) })
}

package admin

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/archive"
	"github.com/rm4n0s/radio-tutor/internal/core/phone"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/student"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

const (
	adminPassword   = "parent-secret"
	studentPassword = "abcd"
	fractionsID     = "org.example.archive-1"
	fractionsTitle  = "Fractions, part 1"
	otherID         = "org.example.second-2"
	exShade         = "shade-the-half"
	testdata        = "../../core/archive/testdata/valid"
	tallWindow      = 1000.0
)

// fixture is a cassette on disk to install: a copy of the archive package's
// valid cassette with another id, version or title.
func fixture(t testing.TB, id, version, title string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "cassette")
	if err := os.CopyFS(dst, os.DirFS(testdata)); err != nil {
		t.Fatalf("copying the fixture: %v", err)
	}
	file := filepath.Join(dst, "cassette.json")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, r := range [][2]string{
		{`"id": "` + fractionsID + `"`, `"id": "` + id + `"`},
		{`"version": "1.0.0"`, `"version": "` + version + `"`},
		{`"title": "` + fractionsTitle + `"`, `"title": "` + title + `"`},
	} {
		if !strings.Contains(text, r[0]) {
			t.Fatalf("the fixture manifest has no %s", r[0])
		}
		text = strings.Replace(text, r[0], r[1], 1)
	}
	if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return dst
}

// install imports the cassette folder into the environment's store.
func install(t testing.TB, env *app.Env, path string) {
	t.Helper()
	res, err := env.Archive.Import(context.Background(), path, nil)
	if err != nil {
		t.Fatalf("installing %s: %v", path, err)
	}
	if res.Status != archive.ImportInstalled {
		t.Fatalf("installing %s: status %v, problems %v", path, res.Status, res.Problems)
	}
}

// twoCassettes installs the fixture cassette and a second one.
func twoCassettes(t *testing.T, env *app.Env) {
	install(t, env, fixture(t, fractionsID, "1.0.0", fractionsTitle))
	install(t, env, fixture(t, otherID, "2.1.0", "Second steps"))
}

// setProgress changes the save file of the user for the cassette.
func setProgress(t testing.TB, env *app.Env, user, id string, change func(p *progress.Progress)) {
	t.Helper()
	err := env.Progress.Update(context.Background(), user, id, func(p *progress.Progress) error {
		change(p)
		return nil
	})
	if err != nil {
		t.Fatalf("setting the progress: %v", err)
	}
}

// created is the time every profile of the tests is created.
var created = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

// fixedInterfaces are the network interfaces the settings screen lists in tests.
func fixedInterfaces() ([]phone.Interface, error) {
	return []phone.Interface{
		{Name: "lo", Up: true, Loopback: true, Addrs: []net.IP{net.IPv4(127, 0, 0, 1)}},
		{Name: "wlan0", Up: true, Addrs: []net.IP{net.IPv4(192, 168, 1, 20)}},
		{Name: "eth0", Up: true, Addrs: []net.IP{net.IPv4(10, 0, 0, 5)}},
	}, nil
}

// config is what a test sets up before the window opens.
type config struct {
	locale string
	size   m.Size
	// setup runs on the environment after the profiles exist.
	setup func(t *testing.T, env *app.Env)
	// choose answers the file dialog.
	choose app.FileChooser
	// screens are the admin screens of the later steps.
	screens Screens
	// keepNotice leaves the notice of waiting exercises open after the login;
	// otherwise pump answers it with Later, so that the tests of the other
	// screens find the home as it was.
	keepNotice bool
}

// pump opens the app with an admin ("mum") and a student ("mia"), logs mum in
// and returns the app showing the admin home.
func pump(t *testing.T, c config) *testutil.App {
	t.Helper()
	if c.size == (m.Size{}) {
		c.size = m.Size{Width: app.MinWindowWidth, Height: tallWindow}
	}
	if c.screens.Interfaces == nil {
		c.screens.Interfaces = fixedInterfaces
	}
	a := testutil.Pump(t, testutil.Options{
		Locale:     c.locale,
		Size:       c.size,
		ChooseFile: c.choose,
		Screens: app.Screens{
			Admin:   HomeBuilder(c.screens),
			Student: student.HomeBuilder(student.Screens{}),
		},
		Before: func(env *app.Env) {
			// A fixed clock, so that the creation dates of the goldens never
			// change.
			store, err := profile.Open(context.Background(), env.Paths.ProfilesFile(),
				profile.WithIterations(1), profile.WithClock(func() time.Time { return created }))
			if err != nil {
				t.Fatal(err)
			}
			env.Profiles = store
			testutil.AddProfile(t, env, "mum", "Mum", profile.RoleAdmin, adminPassword)
			testutil.AddProfile(t, env, "mia", "Mia", profile.RoleStudent, studentPassword)
			if c.setup != nil {
				c.setup(t, env)
			}
		},
	})
	a.TapKey(app.KeyProfilePrefix + "mum")
	a.EnterKey(app.KeyPassword, adminPassword)
	a.TapKey(app.KeySubmit)
	a.WaitFor(t, "the admin home", func() bool { return a.HasKey(KeyStudents) })
	// The notice of waiting exercises opens when the first count is in.
	// (A broken environment, which some tests make on purpose, has none.)
	counts, err := scanWaiting(context.Background(), a.Env, nil)
	if err == nil && counts.total() > 0 {
		waitKey(t, a, KeyReviewNotice)
		if !c.keepNotice {
			a.TapKey(KeyNoticeLater)
			a.PumpAndSettle()
			waitGone(t, a, KeyReviewNotice)
		}
	}
	return a
}

// has reports whether the text is on screen.
func has(a *testutil.App, text string) bool { return a.Exists(gluttertest.ByText(text)) }

// key finds the widget with the key.
func key(k string) gluttertest.Finder { return gluttertest.ByKey(m.NewValueKey(k)) }

// waitKey waits until a widget with the key is on screen.
func waitKey(t *testing.T, a *testutil.App, k string) {
	t.Helper()
	a.WaitFor(t, "the widget "+k, func() bool { return a.HasKey(k) })
}

// waitGone waits until no widget with the key is on screen.
func waitGone(t *testing.T, a *testutil.App, k string) {
	t.Helper()
	a.WaitFor(t, "the widget "+k+" to go", func() bool { return !a.HasKey(k) })
}

// waitText waits until the text is on screen.
func waitText(t *testing.T, a *testutil.App, text string) {
	t.Helper()
	a.WaitFor(t, "the text "+text, func() bool { return has(a, text) })
}

// goBack pops the current screen with the back button of the app bar.
func goBack(t *testing.T, a *testutil.App) {
	t.Helper()
	a.Tap(gluttertest.ByType[*m.BackButton]())
	a.PumpAndSettle()
}

// tillTrue polls cond in real time for a short while (state saved by a
// goroutine).
func tillTrue(t *testing.T, a *testutil.App, what string, cond func() bool) {
	t.Helper()
	a.WaitFor(t, what, cond)
}

// waitLimit is a generous wait.
const waitLimit = 5 * time.Second

// whileBusy waits until cond is true like WaitFor, but pumps single frames: a
// running progress indicator animates forever, so the screen never settles.
func whileBusy(t *testing.T, a *testutil.App, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitLimit)
	for {
		a.FlushPosted()
		a.Pump(20 * time.Millisecond)
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// tapBusy taps the widget with the key without waiting for the screen to settle.
func tapBusy(a *testutil.App, k string) {
	a.Tap(key(k))
	a.Pump()
}

// hideFromMia hides the cassette from mia in the settings.
func hideFromMia(t testing.TB, env *app.Env, id string) {
	t.Helper()
	err := env.Settings.Update(context.Background(), func(s *settings.Settings) {
		s.HiddenCassettes["mia"] = append(s.HiddenCassettes["mia"], id)
	})
	if err != nil {
		t.Fatal(err)
	}
}

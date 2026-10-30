package student

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/archive"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

const (
	studentPassword = "abcd"
	fractionsID     = "org.example.archive-1"
	fractionsTitle  = "Fractions, part 1"
	// The items of the fixture cassette, in order: a lesson, a worksheet of two
	// exercises, a lesson, a worksheet of one exercise.
	lesson1    = "what-is-a-fraction"
	exShade    = "shade-the-half"
	exName     = "name-the-fraction"
	lesson2    = "adding-fractions"
	exSum      = "write-the-sum"
	testdata   = "../../core/archive/testdata/valid"
	tallWindow = 1000.0
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
func install(t testing.TB, env *app.Env, path string) archive.ImportResult {
	t.Helper()
	res, err := env.Archive.Import(context.Background(), path, nil)
	if err != nil {
		t.Fatalf("installing %s: %v", path, err)
	}
	if res.Status != archive.ImportInstalled {
		t.Fatalf("installing %s: status %v, problems %v", path, res.Status, res.Problems)
	}
	return res
}

// setProgress changes the save file of mia for the cassette.
func setProgress(t testing.TB, env *app.Env, id string, change func(p *progress.Progress)) {
	t.Helper()
	err := env.Progress.Update(context.Background(), "mia", id, func(p *progress.Progress) error {
		change(p)
		return nil
	})
	if err != nil {
		t.Fatalf("setting the progress: %v", err)
	}
}

// config is what a test sets up before the window opens.
type config struct {
	locale string
	size   m.Size
	// setup runs on the environment after the profiles exist.
	setup func(t *testing.T, env *app.Env)
	// choose answers the file dialog.
	choose app.FileChooser
	// screens are the lesson and worksheet screens of the later steps.
	screens Screens
}

// pump opens the app with an admin ("mum") and a student ("mia"), logs mia in
// and returns the app showing the shelf.
func pump(t *testing.T, c config) *testutil.App {
	t.Helper()
	if c.size == (m.Size{}) {
		c.size = m.Size{Width: app.MinWindowWidth, Height: app.MinWindowHeight}
	}
	a := testutil.Pump(t, testutil.Options{
		Locale:     c.locale,
		Size:       c.size,
		ChooseFile: c.choose,
		Screens:    app.Screens{Student: HomeBuilder(c.screens)},
		Before: func(env *app.Env) {
			testutil.AddProfile(t, env, "mum", "Mum", profile.RoleAdmin, "parent-secret")
			testutil.AddProfile(t, env, "mia", "Mia", profile.RoleStudent, studentPassword)
			if c.setup != nil {
				c.setup(t, env)
			}
		},
	})
	logIn(t, a, "mia", studentPassword)
	return a
}

// logIn logs in from the login screen and waits for the shelf.
func logIn(t *testing.T, a *testutil.App, username, password string) {
	t.Helper()
	a.TapKey(app.KeyProfilePrefix + username)
	a.EnterKey(app.KeyPassword, password)
	a.TapKey(app.KeySubmit)
	a.WaitFor(t, "the shelf", func() bool { return a.HasKey(app.KeyLogout) })
}

// waitShelf waits until the cards have loaded and every cover is drawn.
func waitShelf(t *testing.T, a *testutil.App) {
	t.Helper()
	a.WaitFor(t, "the cards", func() bool {
		return !a.Exists(gluttertest.ByType[*m.CircularProgressIndicator]()) &&
			!a.Exists(gluttertest.ByIcon(m.Icons.MenuBook()))
	})
}

// has reports whether the text is on screen.
func has(a *testutil.App, text string) bool { return a.Exists(gluttertest.ByText(text)) }

// key finds the widget with the key.
func key(k string) gluttertest.Finder { return gluttertest.ByKey(m.NewValueKey(k)) }

// hasMark reports whether the widget with the key shows a state mark with the
// tooltip.
func hasMark(a *testutil.App, k, tooltip string) bool {
	return a.Exists(gluttertest.Descendant(key(k), gluttertest.ByTooltip(tooltip)))
}

// openCassette taps the card of the cassette and waits for its screen.
func openCassette(t *testing.T, a *testutil.App, id string) {
	t.Helper()
	a.TapKey(KeyCardPrefix + id)
	a.WaitFor(t, "the cassette screen", func() bool { return a.HasKey(KeyContinue) })
}

// waitLimit is a generous wait for a file to change.
const waitLimit = 5 * time.Second

// waitProgress waits until cond is true for the save file of mia.
func waitProgress(t *testing.T, a *testutil.App, id string, cond func(p *progress.Progress) bool) {
	t.Helper()
	deadline := time.Now().Add(waitLimit)
	for {
		p, err := a.Env.Progress.Load(context.Background(), "mia", id)
		if err != nil {
			t.Fatalf("loading the progress: %v", err)
		}
		if cond(p) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the save file never got the expected content: %+v", p)
		}
		a.FlushPosted()
		time.Sleep(5 * time.Millisecond)
	}
}

// hide hides the cassette from mia in the settings.
func hide(t testing.TB, env *app.Env, id string) {
	t.Helper()
	err := env.Settings.Update(context.Background(), func(s *settings.Settings) {
		s.HiddenCassettes["mia"] = append(s.HiddenCassettes["mia"], id)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// breakCassette makes the installed cassette's manifest unreadable.
func breakCassette(t testing.TB, env *app.Env, id string) {
	t.Helper()
	file := filepath.Join(env.Paths.CassettesDir(), id, "cassette.json")
	if err := os.WriteFile(file, []byte("{ not json"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// breakProgressFolder puts a file where the save files of mia would be a folder.
func breakProgressFolder(t testing.TB, env *app.Env) {
	t.Helper()
	dir := filepath.Join(env.Paths.ProgressDir(), "mia")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(env.Paths.ProgressDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// size is the window of the smallest width and the height.
func size(height float64) m.Size { return m.Size{Width: app.MinWindowWidth, Height: height} }

// plain is a screen that only shows the text.
func plain(text string) m.Widget {
	return m.NewScaffold(m.ScaffoldOpts{Body: m.NewCenter(m.NewText(text))})
}

// removeLessons deletes the lessons folder of the installed cassette, which makes
// the cassette invalid but leaves its manifest readable.
func removeLessons(t testing.TB, env *app.Env, id string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(env.Paths.CassettesDir(), id, "lessons")); err != nil {
		t.Fatal(err)
	}
}

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

// writeBrokenSave replaces the save file of mia with text that is not JSON.
func writeBrokenSave(t testing.TB, env *app.Env, id string) {
	t.Helper()
	dir := filepath.Join(env.Paths.ProgressDir(), "mia")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte("{ broken"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// removeCover deletes cover.svg of the installed cassette.
func removeCover(t testing.TB, env *app.Env, id string) {
	t.Helper()
	if err := os.Remove(filepath.Join(env.Paths.CassettesDir(), id, coverFile)); err != nil {
		t.Fatal(err)
	}
}

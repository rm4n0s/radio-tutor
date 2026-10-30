package app_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

func TestStartOpensTheCassetteAndProgressStores(t *testing.T) {
	paths := storagetest.Isolate(t)
	// An import that was interrupted left a temporary folder behind.
	leftover := filepath.Join(paths.CassettesDir(), "org.example.x-1_tmp-abcdef")
	write(t, filepath.Join(leftover, "cassette.json"), "{}")
	env, err := app.Start(context.Background(), startOptions(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	if env.Archive == nil || env.Progress == nil || env.ChooseFile == nil {
		t.Fatalf("Start left a store nil: archive %v, progress %v, chooser %v", env.Archive, env.Progress, env.ChooseFile != nil)
	}
	if _, err := os.Stat(leftover); !os.IsNotExist(err) {
		t.Errorf("the temporary folder of the interrupted import was not deleted: %v", err)
	}
	list, err := env.Archive.List(context.Background())
	if err != nil || len(list) != 0 {
		t.Errorf("List = %v, %v", list, err)
	}
}

func TestStartKeepsGoingWhenTheCassettesFolderCannotBeRecovered(t *testing.T) {
	paths := storagetest.Isolate(t)
	// The cassettes folder is a file: Recover fails, the app still starts.
	write(t, paths.CassettesDir(), "not a folder")
	env, err := app.Start(context.Background(), startOptions(nil))
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	env.Close()
}

func TestHomeShowsTheStudentScreenOfTheShell(t *testing.T) {
	a := testutil.Pump(t, testutil.Options{
		Screens: app.Screens{Student: func(username string) m.Widget {
			return m.NewScaffold(m.ScaffoldOpts{Body: m.NewText("shelf of " + username)})
		}},
		Before: func(env *app.Env) {
			testutil.AddProfile(t, env, "mum", "Mum", profile.RoleAdmin, "parent-secret")
			testutil.AddProfile(t, env, "mia", "Mia", profile.RoleStudent, "abcd")
		},
	})
	a.TapKey(app.KeyProfilePrefix + "mia")
	a.EnterKey(app.KeyPassword, "abcd")
	a.TapKey(app.KeySubmit)
	a.WaitForText(t, "shelf of mia")
}

// chooserApp pumps a widget that hands out its BuildContext.
func chooserApp(t *testing.T) (*testutil.App, m.BuildContext) {
	t.Helper()
	testutil.UseLinux(t)
	ts := gluttertest.New(t, gluttertest.Opts{Size: testutil.DefaultSize})
	var bc m.BuildContext
	ts.PumpWidget(m.NewMaterialApp(m.MaterialAppOpts{Home: m.NewBuilder(func(c m.BuildContext) m.Widget {
		bc = c
		return m.NewText("window")
	})}))
	ts.PumpAndSettle()
	return &testutil.App{Tester: ts}, bc
}

func TestDesktopFileChooser(t *testing.T) {
	question := app.FileQuestion{
		Title: "Choose", AcceptLabel: "Import", FilterName: "Cassettes", Patterns: []string{"*.cassette"}, AllFilesName: "All",
	}
	tests := []struct {
		name     string
		react    func(d *gluttertest.FakeDesktop)
		wantPath string
		wantOK   bool
		wantErr  bool
	}{
		{"a file is chosen", func(d *gluttertest.FakeDesktop) { d.AnswerFileChooser("/tmp/a b.cassette") }, "/tmp/a b.cassette", true, false},
		{"the user cancels", func(d *gluttertest.FakeDesktop) { d.CancelFileChooser() }, "", false, false},
		{"there is no file dialog", nil, "", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, bc := chooserApp(t)
			d := a.FakeDesktop()
			if tt.react == nil {
				d.DisableFileChooser()
			}
			type result struct {
				path string
				ok   bool
				err  error
			}
			done := make(chan result, 1)
			go func() {
				p, ok, err := app.DesktopFileChooser(context.Background(), bc, question)
				done <- result{p, ok, err}
			}()
			if tt.react != nil {
				a.WaitFor(t, "the dialog", func() bool { return d.PendingFileChoosers() == 1 })
				tt.react(d)
			}
			var got result
			deadline := time.After(5 * time.Second)
		wait:
			for {
				a.FlushPosted()
				select {
				case got = <-done:
					break wait
				case <-deadline:
					t.Fatal("the chooser never returned")
				case <-time.After(2 * time.Millisecond):
				}
			}
			if got.path != tt.wantPath || got.ok != tt.wantOK || (got.err != nil) != tt.wantErr {
				t.Errorf("chooser = %q, %v, %v; want %q, %v, error %v", got.path, got.ok, got.err, tt.wantPath, tt.wantOK, tt.wantErr)
			}
			if tt.wantErr && !app.FileChooserUnavailable(got.err) {
				t.Errorf("the error %v is not FileChooserUnavailable", got.err)
			}
		})
	}
}

func TestFileChooserUnavailable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"the tag", errors.New("FileChooserUnavailable", "no portal"), true},
		{"another tag", errors.New("FileChooserFailed", "x"), false},
		{"a foreign error", os.ErrNotExist, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := app.FileChooserUnavailable(tt.err); got != tt.want {
				t.Errorf("FileChooserUnavailable = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHomeShowsTheAdminScreenOfTheShell(t *testing.T) {
	a := testutil.Pump(t, testutil.Options{
		Screens: app.Screens{Admin: func() m.Widget {
			return m.NewScaffold(m.ScaffoldOpts{Body: m.NewText("admin home")})
		}},
		Before: func(env *app.Env) {
			testutil.AddProfile(t, env, "mum", "Mum", profile.RoleAdmin, "parent-secret")
		},
	})
	a.TapKey(app.KeyProfilePrefix + "mum")
	a.EnterKey(app.KeyPassword, "parent-secret")
	a.TapKey(app.KeySubmit)
	a.WaitForText(t, "admin home")
}

// TestLogOutRemovesSnackBars checks that a message shown to one user does not
// stay on the screens of the next one: it would cover the bottom of the next
// user's screen, where the buttons are.
func TestLogOutRemovesSnackBars(t *testing.T) {
	taps := 0
	at := func(alignment m.Alignment) func(m.Widget) m.Widget {
		return func(body m.Widget) m.Widget {
			return m.NewScaffold(m.ScaffoldOpts{Body: m.NewAlign(body, m.AlignOpts{Alignment: alignment})})
		}
	}
	top, bottom := at(m.AlignmentTopLeft), at(m.AlignmentBottomLeft)
	admin := func() m.Widget {
		return m.NewBuilder(func(bc m.BuildContext) m.Widget {
			return top(m.NewRow([]m.Widget{
				m.NewTextButton(func() {
					m.ScaffoldMessengerOf(bc).ShowSnackBar(m.NewSnackBar(m.NewText("The cassette was added.")))
				}, m.NewText("Add"), m.TextButtonOpts{Key: m.NewValueKey("snack")}),
				m.NewTextButton(func() { app.ScopeOf(bc).LogOut(bc) }, m.NewText("Out"), m.TextButtonOpts{Key: m.NewValueKey("out")}),
			}, m.RowOpts{MainAxisSize: m.MainAxisSizeMin}))
		})
	}
	student := func(string) m.Widget {
		return bottom(m.NewTextButton(func() { taps++ }, m.NewText("Play"), m.TextButtonOpts{Key: m.NewValueKey("play")}))
	}
	a := testutil.Pump(t, testutil.Options{
		Screens: app.Screens{Admin: admin, Student: student},
		Before: func(env *app.Env) {
			testutil.AddProfile(t, env, "mum", "Mum", profile.RoleAdmin, "parent-secret")
			testutil.AddProfile(t, env, "mia", "Mia", profile.RoleStudent, "abcd")
		},
	})
	a.TapKey(app.KeyProfilePrefix + "mum")
	a.EnterKey(app.KeyPassword, "parent-secret")
	a.TapKey(app.KeySubmit)
	a.WaitFor(t, "the admin screen", func() bool { return a.HasKey("snack") })
	a.TapKey("snack")
	a.WaitForText(t, "The cassette was added.")

	a.TapKey("out")
	a.WaitFor(t, "the login screen", func() bool { return a.HasKey(app.KeyProfilePrefix + "mia") })
	a.TapKey(app.KeyProfilePrefix + "mia")
	a.EnterKey(app.KeyPassword, "abcd")
	a.TapKey(app.KeySubmit)
	a.WaitFor(t, "the student screen", func() bool { return a.HasKey("play") })
	if a.Exists(gluttertest.ByType[*m.SnackBar]()) || a.Exists(gluttertest.ByText("The cassette was added.")) {
		t.Error("the snack bar of the parent is still on the student's screen")
	}
	a.TapKey("play")
	if taps != 1 {
		t.Errorf("the Play button was tapped %d times, want 1 (something covers it)", taps)
	}
}

package admin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

// openStudents opens the students screen.
func openStudents(t *testing.T, a *testutil.App) {
	t.Helper()
	a.TapKey(KeyStudents)
	waitKey(t, a, KeyAddStudent)
}

func TestStudentsListShowsRoleAndDate(t *testing.T) {
	a := pump(t, config{})
	openStudents(t, a)
	for _, text := range []string{"Mum", "Mia", "Parent", "Student"} {
		if !has(a, text) {
			t.Errorf("the list does not show %q", text)
		}
	}
	mia, err := a.Env.Profiles.Get("mia")
	if err != nil {
		t.Fatal(err)
	}
	if want := "@mia - added " + mia.CreatedAt.Format("2006-01-02"); !has(a, want) {
		t.Errorf("the row of Mia does not say %q", want)
	}
	if a.HasKey(KeyVisiblePrefix + "mum") {
		t.Error("a parent has a Cassettes button")
	}
	if !a.HasKey(KeyVisiblePrefix + "mia") {
		t.Error("a student has no Cassettes button")
	}
}

func TestAddStudent(t *testing.T) {
	tests := []struct {
		name     string
		role     string // key of the role choice, empty for the default
		username string
		display  string
		password string
		again    string
		want     string // text of the error, empty when it works
		created  profile.Role
	}{
		{"a student", "", "leo", "Leo", "wxyz", "wxyz", "", profile.RoleStudent},
		{"a parent", KeyRoleAdmin, "dad", "Dad", "long-secret", "long-secret", "", profile.RoleAdmin},
		{"passwords differ", "", "leo", "Leo", "wxyz", "wxy", "The two passwords are not the same.", ""},
		{"username taken", "", "mia", "Another Mia", "wxyz", "wxyz", "This username is already used.", ""},
		{"bad username", "", "Le o", "Leo", "wxyz", "wxyz", "The username needs 2 to 32 characters: letters a-z, digits and _.", ""},
		{"no name", "", "leo", "  ", "wxyz", "wxyz", "The name shown needs 1 to 40 characters.", ""},
		{"password too short for a student", "", "leo", "Leo", "abc", "abc", "The password needs at least 4 characters.", ""},
		{"password too short for a parent", KeyRoleAdmin, "dad", "Dad", "short", "short", "The password needs at least 8 characters.", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pump(t, config{})
			openStudents(t, a)
			a.TapKey(KeyAddStudent)
			waitKey(t, a, KeyDialogOK)
			if tt.role != "" {
				a.TapKey(tt.role)
			}
			a.EnterKey(app.KeyUsername, tt.username)
			a.EnterKey(app.KeyDisplayName, tt.display)
			a.EnterKey(app.KeyPassword, tt.password)
			a.EnterKey(app.KeyPasswordAgain, tt.again)
			a.TapKey(KeyDialogOK)
			if tt.want != "" {
				waitText(t, a, tt.want)
				if !a.HasKey(KeyDialogOK) {
					t.Error("the dialog closed after an error")
				}
				if got := len(a.Env.Profiles.List()); got != 2 {
					t.Errorf("%d profiles, want 2", got)
				}
				return
			}
			waitGone(t, a, KeyDialogOK)
			waitText(t, a, tt.display)
			p, err := a.Env.Profiles.Get(tt.username)
			if err != nil {
				t.Fatal(err)
			}
			if p.Role != tt.created || p.DisplayName != tt.display {
				t.Errorf("created %+v", p)
			}
			if _, err := a.Env.Profiles.Login(tt.username, tt.password); err != nil {
				t.Errorf("the new profile cannot log in: %v", err)
			}
		})
	}
}

func TestAddStudentCancel(t *testing.T) {
	a := pump(t, config{})
	openStudents(t, a)
	a.TapKey(KeyAddStudent)
	waitKey(t, a, KeyDialogCancel)
	a.TapKey(KeyDialogCancel)
	waitGone(t, a, KeyDialogOK)
	if got := len(a.Env.Profiles.List()); got != 2 {
		t.Errorf("%d profiles, want 2", got)
	}
}

func TestRenameStudent(t *testing.T) {
	tests := []struct {
		name    string
		display string
		want    string
	}{
		{"new name", "Maria", ""},
		{"empty name", "  ", "The name shown needs 1 to 40 characters."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pump(t, config{})
			openStudents(t, a)
			a.TapKey(KeyRenamePrefix + "mia")
			waitKey(t, a, app.KeyDisplayName)
			if !has(a, "Rename Mia") {
				t.Error("the dialog does not name the student")
			}
			a.EnterKey(app.KeyDisplayName, tt.display)
			a.TapKey(KeyDialogOK)
			if tt.want != "" {
				waitText(t, a, tt.want)
				return
			}
			waitGone(t, a, KeyDialogOK)
			waitText(t, a, tt.display)
			if p, _ := a.Env.Profiles.Get("mia"); p.DisplayName != tt.display || p.Username != "mia" {
				t.Errorf("profile = %+v", p)
			}
		})
	}
}

func TestResetPassword(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		password string
		again    string
		want     string
	}{
		{"works", "mia", "new1", "new1", ""},
		{"differ", "mia", "new1", "new2", "The two passwords are not the same."},
		{"too short", "mia", "n", "n", "The password needs at least 4 characters."},
		{"a parent needs 8", "mum", "short", "short", "The password needs at least 8 characters."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pump(t, config{})
			openStudents(t, a)
			a.TapKey(KeyResetPrefix + tt.user)
			waitKey(t, a, app.KeyPassword)
			a.EnterKey(app.KeyPassword, tt.password)
			a.EnterKey(app.KeyPasswordAgain, tt.again)
			a.TapKey(KeyDialogOK)
			if tt.want != "" {
				waitText(t, a, tt.want)
				return
			}
			waitGone(t, a, KeyDialogOK)
			waitText(t, a, "The password of Mia was changed.")
			if _, err := a.Env.Profiles.Login("mia", tt.password); err != nil {
				t.Errorf("the new password does not work: %v", err)
			}
			if _, err := a.Env.Profiles.Login("mia", studentPassword); err == nil {
				t.Error("the old password still works")
			}
		})
	}
}

// photos puts a photo of the user for the cassette on disk.
func photos(t *testing.T, env *app.Env, user, id string) string {
	t.Helper()
	dir := filepath.Join(env.Paths.PhotosDir(), user, id, "sub-1")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "1.jpg"), []byte("jpg"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestDeleteStudent(t *testing.T) {
	var miaPhotos, leoPhotos string
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		twoCassettes(t, env)
		testutil.AddProfile(t, env, "leo", "Leo", profile.RoleStudent, studentPassword)
		for _, user := range []string{"mia", "leo"} {
			setProgress(t, env, user, fractionsID, func(p *progress.Progress) {
				p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StatePassed, Attempts: 1}
			})
		}
		miaPhotos = photos(t, env, "mia", fractionsID)
		leoPhotos = photos(t, env, "leo", fractionsID)
		err := env.Settings.Update(context.Background(), func(s *settings.Settings) {
			s.HiddenCassettes["mia"] = []string{otherID}
			s.HiddenCassettes["leo"] = []string{otherID}
		})
		if err != nil {
			t.Fatal(err)
		}
	}})
	openStudents(t, a)
	a.TapKey(KeyDeletePrefix + "mia")
	waitKey(t, a, KeyDialogOK)
	if !has(a, "Delete Mia?") || !has(a, "The profile, the progress and the photos of Mia will be deleted. This cannot be undone.") {
		t.Error("the confirmation does not name the student")
	}
	// Cancelling deletes nothing.
	a.TapKey(KeyDialogCancel)
	waitGone(t, a, KeyDialogOK)
	if _, err := a.Env.Profiles.Get("mia"); err != nil {
		t.Fatalf("Mia was deleted by Cancel: %v", err)
	}

	a.TapKey(KeyDeletePrefix + "mia")
	waitKey(t, a, KeyDialogOK)
	a.TapKey(KeyDialogOK)
	waitText(t, a, "Mia was deleted.")
	if _, err := a.Env.Profiles.Get("mia"); err == nil {
		t.Error("the profile of Mia is still there")
	}
	if exists(filepath.Join(a.Env.Paths.ProgressDir(), "mia")) || exists(filepath.Join(a.Env.Paths.PhotosDir(), "mia")) || exists(miaPhotos) {
		t.Error("the progress or the photos of Mia are still there")
	}
	if !exists(leoPhotos) || !exists(filepath.Join(a.Env.Paths.ProgressDir(), "leo")) {
		t.Error("the data of Leo was deleted too")
	}
	hidden := a.Env.Settings.Get().HiddenCassettes
	if _, ok := hidden["mia"]; ok || len(hidden["leo"]) != 1 {
		t.Errorf("hidden cassettes = %v", hidden)
	}
	if has(a, "Mia") {
		t.Error("the list still shows Mia")
	}
}

func TestDeleteYourselfIsNotOffered(t *testing.T) {
	a := pump(t, config{})
	openStudents(t, a)
	a.TapKey(KeyDeletePrefix + "mum")
	if a.HasKey(KeyDialogOK) {
		t.Error("a dialog to delete the logged-in account opened")
	}
	if _, err := a.Env.Profiles.Get("mum"); err != nil {
		t.Errorf("mum was deleted: %v", err)
	}
}

func TestDeleteAnotherParent(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		testutil.AddProfile(t, env, "dad", "Dad", profile.RoleAdmin, adminPassword)
	}})
	openStudents(t, a)
	a.TapKey(KeyDeletePrefix + "dad")
	waitKey(t, a, KeyDialogOK)
	a.TapKey(KeyDialogOK)
	waitText(t, a, "Dad was deleted.")
}

func TestDeleteStudentRefusesTheLastAdmin(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		twoCassettes(t, env)
		setProgress(t, env, "mum", fractionsID, func(p *progress.Progress) {
			p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StatePassed, Attempts: 1}
		})
		photos(t, env, "mum", fractionsID)
	}})
	err := deleteStudent(context.Background(), a.Env, "mum")
	wantRoute(t, err, "Store.Delete.LastAdmin")
	if _, err := a.Env.Profiles.Get("mum"); err != nil {
		t.Error("the last admin was deleted")
	}
	if !exists(filepath.Join(a.Env.Paths.ProgressDir(), "mum")) || !exists(filepath.Join(a.Env.Paths.PhotosDir(), "mum")) {
		t.Error("the data of the last admin was deleted although the profile stayed")
	}
	if got := deleteMessage(err).Key; got != "admin.students.last_admin" {
		t.Errorf("message = %s", got)
	}
}

func TestDeleteStudentFailures(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name  string
		ctx   context.Context
		setup func(t *testing.T, env *app.Env)
		route string
	}{
		{"cancelled", cancelled, nil, "deleteStudent.AdminDeleteCancelled"},
		{"photos cannot be removed", context.Background(), func(t *testing.T, env *app.Env) {
			dir := photos(t, env, "mia", fractionsID)
			// A folder without write permission keeps RemoveAll from deleting
			// what is inside it.
			if err := os.Chmod(filepath.Dir(dir), 0o500); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(filepath.Dir(dir), 0o700) })
		}, "deleteStudent->removePhotosOf.AdminPhotosDeleteFailed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				storagetest.SkipWithoutUnixModes(t) // the setup makes a folder read-only
			}
			env, _ := testutil.NewEnv(t, testutil.Options{})
			testutil.AddProfile(t, env, "mum", "Mum", profile.RoleAdmin, adminPassword)
			testutil.AddProfile(t, env, "mia", "Mia", profile.RoleStudent, studentPassword)
			if tt.setup != nil {
				tt.setup(t, env)
			}
			wantRoute(t, deleteStudent(tt.ctx, env, "mia"), tt.route)
		})
	}
}

func TestRemovePhotosOfRefusesUnsafeNames(t *testing.T) {
	env, _ := testutil.NewEnv(t, testutil.Options{})
	for _, name := range []string{"", ".", "..", "../mia", "a/b"} {
		wantRoute(t, removePhotosOf(env, name), "removePhotosOf.AdminPhotosUsernameInvalid")
	}
}

// wantRoute fails unless err is an *errors.Error whose route contains route.
func wantRoute(t *testing.T, err error, route string) {
	t.Helper()
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T: %v", err, err)
	}
	if !appErr.HasRoute(route) {
		t.Errorf("unexpected failure path: %s (want %s)", appErr.Route(), route)
	}
}

func TestHiddenCassettesOfAStudent(t *testing.T) {
	a := pump(t, config{setup: twoCassettes})
	openStudents(t, a)
	a.TapKey(KeyVisiblePrefix + "mia")
	waitKey(t, a, KeyShowPrefix+fractionsID)
	if !has(a, "Cassettes for Mia") || !has(a, fractionsTitle) || !has(a, "Second steps") {
		t.Error("the dialog does not list the cassettes")
	}
	hidden := func() []string { return a.Env.Settings.Get().HiddenCassettes["mia"] }
	if len(hidden()) != 0 {
		t.Fatalf("hidden at first: %v", hidden())
	}

	// Un-ticking hides the cassette from the student, at once and in the file.
	a.TapKey(KeyShowPrefix + otherID)
	tillTrue(t, a, "the cassette to be hidden", func() bool { return len(hidden()) == 1 })
	if hidden()[0] != otherID {
		t.Errorf("hidden = %v", hidden())
	}
	// Ticking shows it again.
	a.TapKey(KeyShowPrefix + otherID)
	tillTrue(t, a, "the cassette to be shown", func() bool { return len(hidden()) == 0 })
	if _, ok := a.Env.Settings.Get().HiddenCassettes["mia"]; ok {
		t.Error("an empty list was kept in the settings")
	}
	a.TapKey(KeyDialogOK)
	waitGone(t, a, KeyDialogOK)
}

func TestHiddenCassettesStartFromTheSettings(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		twoCassettes(t, env)
		err := env.Settings.Update(context.Background(), func(s *settings.Settings) {
			s.HiddenCassettes["mia"] = []string{otherID, "org.example.gone-1"}
		})
		if err != nil {
			t.Fatal(err)
		}
	}})
	openStudents(t, a)
	a.TapKey(KeyVisiblePrefix + "mia")
	waitKey(t, a, KeyShowPrefix+fractionsID)
	// Show the hidden one: the entry of a removed cassette stays untouched.
	a.TapKey(KeyShowPrefix + otherID)
	tillTrue(t, a, "the cassette to be shown", func() bool {
		return len(a.Env.Settings.Get().HiddenCassettes["mia"]) == 1
	})
	if got := a.Env.Settings.Get().HiddenCassettes["mia"][0]; got != "org.example.gone-1" {
		t.Errorf("hidden = %v", got)
	}
}

func TestHiddenCassettesWithNothingInstalled(t *testing.T) {
	a := pump(t, config{})
	openStudents(t, a)
	a.TapKey(KeyVisiblePrefix + "mia")
	waitText(t, a, "No cassettes are installed.")
}

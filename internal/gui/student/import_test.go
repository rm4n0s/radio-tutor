package student

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/rm4n0s/errors"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/archive"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

// answer is a file chooser that always picks the path, and counts the asks.
func answer(path string, asked *atomic.Int32) app.FileChooser {
	return func(_ context.Context, _ m.BuildContext, q app.FileQuestion) (string, bool, error) {
		if asked != nil {
			asked.Add(1)
		}
		if q.Title != "Choose a cassette file" || q.FilterName != "Cassettes (*.cassette)" || q.AllFilesName != "All files" || len(q.Patterns) == 0 {
			return "", false, errors.New("TestBadQuestion", "the file question is wrong", "question", q.Title)
		}
		return path, true, nil
	}
}

func TestImportInstallsACassette(t *testing.T) {
	var asked atomic.Int32
	path := fixture(t, fractionsID, "1.0.0", fractionsTitle)
	a := pump(t, config{choose: answer(path, &asked)})
	a.WaitForText(t, "There are no cassettes yet. Ask a parent to add one.")
	tapBusy(a, KeyImport)
	whileBusy(t, a, "the message", func() bool { return has(a, `The cassette "Fractions, part 1" was added.`) })
	waitShelf(t, a)
	if !a.HasKey(KeyCardPrefix + fractionsID) {
		t.Error("the new cassette is not on the shelf")
	}
	if has(a, "There are no cassettes yet. Ask a parent to add one.") {
		t.Error("the empty message is still shown")
	}
	if a.HasKey(KeyImportCancel) {
		t.Error("the import indicator is still shown")
	}
	if asked.Load() != 1 {
		t.Errorf("the file dialog was opened %d times", asked.Load())
	}
	if _, mf, err := a.Env.Archive.Get(context.Background(), fractionsID); err != nil || mf.Title != fractionsTitle {
		t.Errorf("the store has %v, %v", mf.Title, err)
	}
}

func TestImportShowsTheProblemsOfAnInvalidCassette(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.cassette")
	if err := os.WriteFile(bad, []byte("this is not a zip file"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := archiveImport(t, bad)
	if err != nil || res.Status != archive.ImportInvalid || len(res.Problems) == 0 {
		t.Fatalf("reference import = %+v, %v", res, err)
	}
	a := pump(t, config{choose: answer(bad, nil)})
	a.WaitForText(t, "There are no cassettes yet. Ask a parent to add one.")
	tapBusy(a, KeyImport)
	whileBusy(t, a, "the problems dialog", func() bool { return a.HasKey(KeyDialogOK) })
	for _, text := range []string{"This file is not a good cassette", "Nothing was added. These are the problems:", res.Problems[0].String()} {
		if !has(a, text) {
			t.Errorf("the dialog does not show %q", text)
		}
	}
	a.TapKey(KeyDialogOK)
	if a.HasKey(KeyDialogOK) {
		t.Error("the dialog did not close")
	}
	entries, err := os.ReadDir(a.Env.Paths.CassettesDir())
	if err == nil && len(entries) != 0 {
		t.Errorf("an invalid cassette left %d entries in the store", len(entries))
	}
}

// archiveImport imports the file into a store of its own to learn what Import
// says about it.
func archiveImport(t *testing.T, path string) (archive.ImportResult, error) {
	t.Helper()
	env, _ := testutil.NewEnv(t, testutil.Options{})
	return env.Archive.Import(context.Background(), path, nil)
}

func TestImportUpgradeAsksFirst(t *testing.T) {
	tests := []struct {
		name        string
		button      string
		message     string
		wantVersion string
	}{
		{"confirmed", KeyDialogYes, `The cassette "Fractions, new edition" was updated to version 1.1.0.`, "1.1.0"},
		{"declined", KeyDialogNo, "The cassette was not changed.", "1.0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newer := fixture(t, fractionsID, "1.1.0", "Fractions, new edition")
			a := pump(t, config{
				choose: answer(newer, nil),
				setup: install1(func(p *progress.Progress) {
					p.Lessons[lesson1] = progress.LessonProgress{CompletedAt: completedAt}
				}),
			})
			waitShelf(t, a)
			tapBusy(a, KeyImport)
			whileBusy(t, a, "the upgrade dialog", func() bool { return a.HasKey(KeyDialogYes) })
			for _, text := range []string{"Update the cassette?", "Version 1.0.0 is installed. Replace it with version 1.1.0? The progress is kept."} {
				if !has(a, text) {
					t.Errorf("the dialog does not show %q", text)
				}
			}
			if !a.HasKey(KeyImportCancel) {
				t.Error("the import is not shown as running while the dialog is open")
			}
			// Nothing changes until the answer.
			if _, mf, _ := a.Env.Archive.Get(context.Background(), fractionsID); mf.Version.String() != "1.0.0" {
				t.Fatalf("the cassette changed to %s before the answer", mf.Version)
			}
			tapBusy(a, tt.button)
			whileBusy(t, a, "the message", func() bool { return has(a, tt.message) })
			a.WaitFor(t, "the import to end", func() bool { return a.HasKey(KeyImport) })
			_, mf, err := a.Env.Archive.Get(context.Background(), fractionsID)
			if err != nil || mf.Version.String() != tt.wantVersion {
				t.Errorf("version = %s, %v; want %s", mf.Version, err, tt.wantVersion)
			}
			// The progress is kept.
			p, err := a.Env.Progress.Load(context.Background(), "mia", fractionsID)
			if err != nil || !p.Lessons[lesson1].Completed() {
				t.Errorf("the progress was lost: %+v, %v", p, err)
			}
			if tt.wantVersion == "1.1.0" {
				waitShelf(t, a)
				a.WaitForText(t, "Fractions, new edition")
			}
		})
	}
}

func TestImportSameOrOlderVersion(t *testing.T) {
	tests := []struct {
		name    string
		offer   string
		message string
	}{
		{"same version", "1.0.0", "This cassette is already installed."},
		{"older version", "0.9.0", "A newer version (1.0.0) is installed."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fixture(t, fractionsID, tt.offer, fractionsTitle)
			a := pump(t, config{choose: answer(path, nil), setup: install1(nil)})
			waitShelf(t, a)
			tapBusy(a, KeyImport)
			whileBusy(t, a, "the message", func() bool { return has(a, tt.message) })
			if a.HasKey(KeyDialogYes) {
				t.Error("a dialog asked about a version that is not newer")
			}
		})
	}
}

func TestImportCancelledAndFailed(t *testing.T) {
	tests := []struct {
		name    string
		choose  app.FileChooser
		message string
	}{
		{"the user cancels the file dialog", func(context.Context, m.BuildContext, app.FileQuestion) (string, bool, error) {
			return "", false, nil
		}, ""},
		{"a file that is not there", func(context.Context, m.BuildContext, app.FileQuestion) (string, bool, error) {
			return filepath.Join(os.TempDir(), "radio-tutor-no-such-file.cassette"), true, nil
		}, "The cassette could not be added. Ask a parent."},
		{"no file dialog on the desktop", func(context.Context, m.BuildContext, app.FileQuestion) (string, bool, error) {
			return "", false, errors.New("FileChooserUnavailable", "no portal")
		}, "This computer has no file dialog, so a file cannot be chosen."},
		{"the dialog fails", func(context.Context, m.BuildContext, app.FileQuestion) (string, bool, error) {
			return "", false, errors.New("FileChooserFailed", "the bus is gone")
		}, "The cassette could not be added. Ask a parent."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pump(t, config{choose: tt.choose})
			a.WaitForText(t, "There are no cassettes yet. Ask a parent to add one.")
			tapBusy(a, KeyImport)
			if tt.message != "" {
				whileBusy(t, a, "the message", func() bool { return has(a, tt.message) })
			}
			whileBusy(t, a, "the import to end", func() bool { return a.HasKey(KeyImport) })
			if tt.message == "" && has(a, "The cassette could not be added. Ask a parent.") {
				t.Error("a cancel was reported as a failure")
			}
		})
	}
}

func TestImportCanBeCancelled(t *testing.T) {
	started := make(chan struct{})
	choose := func(ctx context.Context, _ m.BuildContext, _ app.FileQuestion) (string, bool, error) {
		close(started)
		<-ctx.Done()
		return "", false, ctx.Err()
	}
	a := pump(t, config{choose: choose})
	a.WaitForText(t, "There are no cassettes yet. Ask a parent to add one.")
	tapBusy(a, KeyImport)
	<-started
	whileBusy(t, a, "the progress indicator", func() bool { return a.HasKey(KeyImportCancel) })
	if !has(a, "Adding the cassette...") || a.HasKey(KeyImport) {
		t.Error("the shelf does not show the running import")
	}
	tapBusy(a, KeyImportCancel)
	a.WaitFor(t, "the Import button again", func() bool { return a.HasKey(KeyImport) })
	if has(a, "The cassette could not be added. Ask a parent.") {
		t.Error("a cancel was reported as a failure")
	}
}

func TestImportFailureMessages(t *testing.T) {
	env, _ := testutil.NewEnv(t, testutil.Options{})
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name    string
		ctx     context.Context
		admin   bool
		err     error
		wantKey string
		wantArg []string
	}{
		{"admin sees the tag", context.Background(), true, errors.New("SomeFailure", "it broke"), "student.import.failed_detail", []string{"message", "it broke", "tag", "SomeFailure"}},
		{"admin sees a foreign error", context.Background(), true, os.ErrNotExist, "student.import.failed_detail", []string{"message", os.ErrNotExist.Error(), "tag", "Unknown"}},
		{"child is told to ask", context.Background(), false, errors.New("SomeFailure", "it broke"), "student.import.failed", nil},
		{"no dialog", context.Background(), false, errors.New("FileChooserUnavailable", "no portal"), "student.import.no_dialog", nil},
		{"cancelled is silent", cancelled, true, context.Canceled, "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := importFailure(tt.ctx, env, tt.admin, tt.err)
			if tt.wantKey == "" {
				if out.message != nil {
					t.Fatalf("message = %+v, want none", out.message)
				}
				return
			}
			if out.message == nil || out.message.Key != tt.wantKey || !slices.Equal(out.message.Args, tt.wantArg) {
				t.Errorf("message = %+v, want %s %v", out.message, tt.wantKey, tt.wantArg)
			}
		})
	}
}

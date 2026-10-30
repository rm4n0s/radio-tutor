package admin

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/problem"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// The functions of this file are the work of the progress and review screens:
// they read and write the stores and the photos folder, never touch a widget,
// and are called on goroutines.

// reviewData is a student's save file for one cassette together with the
// cassette document, as the screens show them.
type reviewData struct {
	// ID is the cassette id.
	ID string
	// Title is the cassette's title, or its id when the cassette cannot be read.
	Title string
	// Cassette is the installed cassette; nil when it is not installed or is
	// unusable. Without it the screens are read-only.
	Cassette *cassette.Cassette
	// NotInstalled says why Cassette is nil: the cassette is not installed (the
	// other reason is that it is damaged).
	NotInstalled bool
	// Progress is the save file.
	Progress *progress.Progress
	// States are the item states by id (progress.States); nil without Cassette.
	States map[string]progress.ItemState
}

// openCassette loads the installed cassette. It fails with the tags of
// archive.Store.Get, AdminCassetteInvalid (the documents have errors) and the
// tags of cassette.Load.
func openCassette(ctx context.Context, env *app.Env, id string) (*cassette.Cassette, error) {
	fsys, _, err := env.Archive.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	c, problems, err := cassette.Load(ctx, fsys)
	if err != nil {
		return nil, err
	}
	if c == nil || problem.HasErrors(problems) {
		return nil, errors.New("AdminCassetteInvalid", "the installed cassette has errors; remove it and import it again", "cassette", id, "problems", problems)
	}
	return c, nil
}

// loadReview reads the save file of the student for the cassette and the
// cassette itself. A cassette that is not installed or cannot be used gives a
// reviewData without Cassette. A broken save file was set aside by the store and
// the fresh one is shown. It fails with the tags of progress.Store.Load, and
// with the cassette's error when ctx is done.
func loadReview(ctx context.Context, env *app.Env, username, id string) (*reviewData, error) {
	d := &reviewData{ID: id, Title: id}
	c, err := openCassette(ctx, env, id)
	switch {
	case err == nil:
		d.Cassette, d.Title = c, c.Manifest.Title
	case ctx.Err() != nil:
		return nil, err
	default:
		d.NotInstalled = hasTag(err, "StoreNotInstalled")
		if !d.NotInstalled {
			env.Log.Warn("an installed cassette cannot be used by the progress screen", "cassette", id, "err", err)
		}
	}
	p, err := env.Progress.Load(ctx, username, id)
	if p == nil {
		return nil, err
	}
	if err != nil {
		env.Log.Error("a save file was broken and set aside", "username", username, "cassette", id, "err", err)
	}
	d.Progress = p
	if c != nil {
		d.States = progress.States(c, p)
	}
	return d, nil
}

// hasTag reports whether err is an error of the errors package with the tag.
func hasTag(err error, tag string) bool {
	appErr, ok := errors.FromError(err)
	return ok && appErr.Tag == tag
}

// savedAnyway turns the note of the progress store that a broken save file was
// set aside (and a fresh one saved) into success.
func savedAnyway(err error) error {
	if hasTag(err, "ProgressFileBroken") {
		return nil
	}
	return err
}

// waitingCounts are the numbers of exercises in needs_review, by username and
// cassette id.
type waitingCounts map[string]map[string]int

// total is the sum of all the numbers.
func (w waitingCounts) total() int {
	n := 0
	for _, byCassette := range w {
		for _, c := range byCassette {
			n += c
		}
	}
	return n
}

// ofStudent is the sum of one student's numbers.
func (w waitingCounts) ofStudent(username string) int {
	n := 0
	for _, c := range w[username] {
		n += c
	}
	return n
}

// walkWaiting visits, for every profile and every installed cassette that can
// be read, the save file and the item states, in the order of the profiles, then
// of the cassette ids. The state is the one the unlock rule computes
// (progress.States), the same the progress screen shows, so the count and the
// review queue never differ from what the admin finds there. Cassettes that are
// not installed are not visited: nothing can be decided about them. Only the
// listed usernames are looked at; nil means every profile.
func walkWaiting(ctx context.Context, env *app.Env, usernames []string,
	visit func(p profile.Profile, c *cassette.Cassette, prog *progress.Progress, states map[string]progress.ItemState)) error {
	installed, err := env.Archive.List(ctx)
	if err != nil {
		return err
	}
	loaded := map[string]*cassette.Cassette{}
	for _, in := range installed {
		if in.Broken {
			continue
		}
		c, err := openCassette(ctx, env, in.ID)
		if err != nil {
			if ctx.Err() != nil {
				return err
			}
			env.Log.Warn("a cassette cannot be counted for review", "cassette", in.ID, "err", err)
			continue
		}
		loaded[in.ID] = c
	}
	ids := sortedKeys(loaded)
	for _, p := range env.Profiles.List() {
		if usernames != nil && !slices.Contains(usernames, p.Username) {
			continue
		}
		for _, id := range ids {
			c := loaded[id]
			// A broken save file is set aside by Load, which returns a fresh
			// Progress with the error; the screens that show the progress tell
			// the admin, this walk goes on.
			prog, err := env.Progress.Load(ctx, p.Username, id)
			if prog == nil {
				return err
			}
			visit(p, c, prog, progress.States(c, prog))
		}
	}
	return nil
}

// scanWaiting counts, for every profile and every installed cassette that can
// be read, the exercises that wait for a parent (SPEC §12.3); see walkWaiting.
func scanWaiting(ctx context.Context, env *app.Env, usernames []string) (waitingCounts, error) {
	out := waitingCounts{}
	err := walkWaiting(ctx, env, usernames, func(p profile.Profile, c *cassette.Cassette, _ *progress.Progress, states map[string]progress.ItemState) {
		n := 0
		for itemID, state := range states {
			if state == progress.StateNeedsReview && c.Exercises[itemID] != nil {
				n++
			}
		}
		if n > 0 {
			if out[p.Username] == nil {
				out[p.Username] = map[string]int{}
			}
			out[p.Username][c.Manifest.ID] = n
		}
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// cassetteEntry is one row of a student's cassette list.
type cassetteEntry struct {
	// ID is the cassette id.
	ID string
	// Title is the cassette's title, or its id when it is not installed.
	Title string
	// Installed is true for a cassette that can be read; the others are
	// read-only.
	Installed bool
	// Waiting is how many exercises wait for review.
	Waiting int
}

// listEntries lists the cassettes of a student's progress page: every installed
// cassette, then the ones that only have a save file, by id.
func listEntries(ctx context.Context, env *app.Env, username string, waiting map[string]int) ([]cassetteEntry, error) {
	installed, err := env.Archive.List(ctx)
	if err != nil {
		return nil, err
	}
	saves, err := listSaveIDs(env, username)
	if err != nil {
		return nil, err
	}
	var out []cassetteEntry
	seen := map[string]bool{}
	for _, in := range installed {
		seen[in.ID] = true
		e := cassetteEntry{ID: in.ID, Title: in.ID, Waiting: waiting[in.ID]}
		if !in.Broken {
			e.Title, e.Installed = in.Manifest.Title, true
		}
		out = append(out, e)
	}
	for _, id := range saves {
		if !seen[id] {
			out = append(out, cassetteEntry{ID: id, Title: id})
		}
	}
	return out, nil
}

// listSaveIDs returns, sorted, the ids of the cassettes the student has a save
// file for. Fails with the tags AdminSavesUsernameInvalid and AdminSavesListFailed.
func listSaveIDs(env *app.Env, username string) ([]string, error) {
	if !safeElement(username) {
		return nil, errors.New("AdminSavesUsernameInvalid", "the username cannot name a progress folder", "username", username)
	}
	dir := filepath.Join(env.Paths.ProgressDir(), username)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, errors.NewErr("AdminSavesListFailed", err).SetMetadata("path", dir)
	}
	var ids []string
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if ok && !e.IsDir() && cassette.ValidCassetteID(id) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids, nil
}

// applyOverride performs a parent's action on the student's save file (SPEC
// §7.7). Fails with the tags of progress.ApplyOverride and progress.Store.Update.
func applyOverride(ctx context.Context, env *app.Env, c *cassette.Cassette, username string, o progress.Override) error {
	return savedAnyway(env.Progress.Update(ctx, username, c.Manifest.ID, func(p *progress.Progress) error {
		return progress.ApplyOverride(c, p, o)
	}))
}

// setSettings changes the switches of the student's save file at once.
func setSettings(ctx context.Context, env *app.Env, username, id string, change func(*progress.Settings)) error {
	return savedAnyway(env.Progress.Update(ctx, username, id, func(p *progress.Progress) error {
		change(&p.Settings)
		return nil
	}))
}

// resetAll is "Reset all progress" (SPEC §7.7). With deletePhotos the photos of
// the student for the cassette are deleted first and the submissions are dropped
// from the save file; otherwise they stay for review. Fails with the tags of
// removeCassettePhotos and progress.Store.Update.
func resetAll(ctx context.Context, env *app.Env, username, id, by string, now time.Time, deletePhotos bool) error {
	if deletePhotos {
		if err := removeCassettePhotos(env, username, id); err != nil {
			return err
		}
	}
	return savedAnyway(env.Progress.Update(ctx, username, id, func(p *progress.Progress) error {
		progress.ResetAll(p, by, "", now, deletePhotos)
		return nil
	}))
}

// photoFolder is photos/<username>/<cassette-id>/<submission-id>, or the folder
// above it when submissionID is empty. Fails with the tag AdminPhotosNameInvalid.
func photoFolder(env *app.Env, username, id, submissionID string) (string, error) {
	if !safeElement(username) || !cassette.ValidCassetteID(id) || (submissionID != "" && !safeElement(submissionID)) {
		return "", errors.New("AdminPhotosNameInvalid", "a name cannot be a photos folder",
			"username", username, "cassette", id, "submission", submissionID)
	}
	dir := filepath.Join(env.Paths.PhotosDir(), username, id)
	if submissionID != "" {
		dir = filepath.Join(dir, submissionID)
	}
	return dir, nil
}

// removeCassettePhotos deletes photos/<username>/<cassette-id>. Fails with the
// tags AdminPhotosNameInvalid and AdminCassettePhotosDeleteFailed.
func removeCassettePhotos(env *app.Env, username, id string) error {
	dir, err := photoFolder(env, username, id, "")
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return errors.NewErr("AdminCassettePhotosDeleteFailed", err).SetMetadata("path", dir)
	}
	return nil
}

// deleteSubmissionPhotos deletes the photos of one submission and clears the
// photo list of its record, which stays (SPEC §12.3). The files go first, so
// that a failure never leaves photos that nothing points to. Fails with the
// tags AdminPhotosNameInvalid, AdminSubmissionPhotosDeleteFailed,
// AdminSubmissionUnknown and the tags of progress.Store.Update.
func deleteSubmissionPhotos(ctx context.Context, env *app.Env, username, id, submissionID string) error {
	dir, err := photoFolder(env, username, id, submissionID)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return errors.NewErr("AdminSubmissionPhotosDeleteFailed", err).SetMetadata("path", dir)
	}
	return savedAnyway(env.Progress.Update(ctx, username, id, func(p *progress.Progress) error {
		sub := p.Submission(submissionID)
		if sub == nil {
			return errors.New("AdminSubmissionUnknown", "the save file has no such submission", "submission", submissionID)
		}
		sub.Photos = []string{}
		return nil
	}))
}

// deleteStudentPhotos deletes all the photos of the student and clears the photo
// lists of the submissions in all their save files (SPEC §11.7). Fails with the
// tags AdminPhotosUsernameInvalid, AdminPhotosDeleteFailed, AdminSavesListFailed
// and the tags of progress.Store.Update.
func deleteStudentPhotos(ctx context.Context, env *app.Env, username string) error {
	if err := removePhotosOf(env, username); err != nil {
		return err
	}
	ids, err := listSaveIDs(env, username)
	if err != nil {
		return err
	}
	for _, id := range ids {
		err := savedAnyway(env.Progress.Update(ctx, username, id, func(p *progress.Progress) error {
			for i := range p.Submissions {
				p.Submissions[i].Photos = []string{}
			}
			return nil
		}))
		if err != nil {
			return err
		}
	}
	return nil
}

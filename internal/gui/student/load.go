package student

import (
	"context"
	"io/fs"
	"slices"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/problem"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// coverFile is the cover of a cassette inside its folder (CASSETTE-FORMAT §1).
const coverFile = "cover.svg"

// card is what the shelf shows of one cassette.
type card struct {
	// ID is the cassette id.
	ID string
	// Title and Subject come from the manifest.
	Title, Subject string
	// Cover is the bytes of cover.svg; nil when it cannot be read.
	Cover []byte
	// Done is the number of passed exercises plus completed lessons, out of
	// Total, the number of items (SPEC §13).
	Done, Total int
}

// countDone counts the items of the manifest that the student has finished:
// completed lessons and passed exercises. Entries of ids the manifest does not
// have, or has with another type, do not count.
func countDone(items []cassette.Item, p *progress.Progress) (done, total int) {
	for _, it := range items {
		switch it.Type {
		case cassette.ItemLesson:
			if p.Lessons[it.ID].Completed() {
				done++
			}
		case cassette.ItemExercise:
			if p.Exercises[it.ID].State == progress.StatePassed {
				done++
			}
		}
	}
	return done, len(items)
}

// loadShelf reads the cassettes the student sees (SPEC §6.3, §13): the installed
// ones that are not broken and not hidden from the student, with their covers
// and progress. A cassette that disappears or breaks between the listing and
// the read is left out. A save file that was broken is set aside by the
// progress store and the cassette starts again at 0.
func loadShelf(ctx context.Context, env *app.Env, username string) ([]card, error) {
	installed, err := env.Archive.List(ctx)
	if err != nil {
		return nil, err
	}
	hidden := env.Settings.Get().HiddenCassettes[username]
	var cards []card
	for _, in := range installed {
		if in.Broken || slices.Contains(hidden, in.ID) {
			continue
		}
		fsys, mf, err := env.Archive.Get(ctx, in.ID)
		if err != nil {
			if ctx.Err() != nil {
				return nil, err
			}
			env.Log.Warn("a cassette could not be read and is left out of the shelf", "cassette", in.ID, "err", err)
			continue
		}
		p, err := env.Progress.Load(ctx, username, in.ID)
		if p == nil {
			return nil, err
		}
		if err != nil {
			env.Log.Error("a save file was broken and set aside", "username", username, "cassette", in.ID, "err", err)
		}
		c := card{ID: in.ID, Title: mf.Title, Subject: mf.Subject}
		c.Done, c.Total = countDone(mf.Items, p)
		c.Cover = readCover(env, fsys, in.ID)
		cards = append(cards, c)
	}
	return cards, nil
}

// readCover reads cover.svg; a cassette whose cover cannot be read is still
// shown, with a plain picture.
func readCover(env *app.Env, fsys fs.FS, id string) []byte {
	data, err := fs.ReadFile(fsys, coverFile)
	if err != nil {
		env.Log.Warn("a cover could not be read", "cassette", id, "err", err)
		return nil
	}
	return data
}

// view is an opened cassette: its documents, the student's progress and what
// the unlock rule makes of them.
type view struct {
	// Cassette is the loaded cassette.
	Cassette *cassette.Cassette
	// FS is the cassette's folder (for the lessons and the worksheets).
	FS fs.FS
	// Cover is the bytes of cover.svg; nil when it cannot be read.
	Cover []byte
	// Progress is the student's save file, merged with this version.
	Progress *progress.Progress
	// States are the item states by id (progress.States).
	States map[string]progress.ItemState
	// Continue is the item the Continue button opens.
	Continue cassette.Item
	// Done and Total are the shelf's numbers for the cassette.
	Done, Total int
}

// loadCassette opens the installed cassette for the student: it loads the
// cassette, reads the save file, merges it with the installed version when the
// file was last merged with an older version (SPEC §7.5; the merge is saved), and
// computes the states and the Continue target. It fails with the tag
// StudentCassetteInvalid when the documents of the cassette have errors, and
// otherwise with the tags of the archive and progress stores.
func loadCassette(ctx context.Context, env *app.Env, username, id string) (*view, error) {
	fsys, _, err := env.Archive.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	c, problems, err := cassette.Load(ctx, fsys)
	if err != nil {
		return nil, err
	}
	if c == nil || problem.HasErrors(problems) {
		return nil, errors.New("StudentCassetteInvalid", "the installed cassette has errors; remove it and import it again", "cassette", id, "problems", problems)
	}
	p, err := env.Progress.Load(ctx, username, id)
	if p == nil {
		return nil, err
	}
	if err != nil {
		env.Log.Error("a save file was broken and set aside", "username", username, "cassette", id, "err", err)
	}
	// Migrate only when the file is older than the installed version, so a
	// save file that is newer (an older cassette was put back) is never orphaned.
	if p.CassetteVersion.Less(c.Manifest.Version) {
		err := env.Progress.Update(ctx, username, id, func(saved *progress.Progress) error {
			p = saved
			if !saved.CassetteVersion.Less(c.Manifest.Version) {
				return nil // another writer merged it meanwhile
			}
			_, err := progress.Migrate(c, saved)
			return err
		})
		if err != nil && !isBrokenFile(err) {
			return nil, err
		}
	}
	v := &view{Cassette: c, FS: fsys, Progress: p, States: progress.States(c, p)}
	v.Done, v.Total = countDone(c.Manifest.Items, p)
	v.Continue, _ = progress.Continue(c, p)
	if data, err := fs.ReadFile(fsys, coverFile); err == nil {
		v.Cover = data
	}
	return v, nil
}

// isBrokenFile reports whether err is the progress store's note that a save
// file was set aside; the store has already saved a fresh one then.
func isBrokenFile(err error) bool {
	appErr, ok := errors.FromError(err)
	return ok && appErr.Tag == "ProgressFileBroken"
}

// worksheetState is the state shown for a worksheet row: locked when every
// exercise is locked, then needs_review when one exercise is, then passed when
// all are, otherwise available.
func worksheetState(w cassette.Worksheet, states map[string]progress.ItemState) progress.ItemState {
	locked, passed := 0, 0
	review := false
	for _, id := range w.ExerciseIDs {
		switch states[id] {
		case progress.StateLocked:
			locked++
		case progress.StatePassed:
			passed++
		case progress.StateNeedsReview:
			review = true
		}
	}
	switch {
	case locked == len(w.ExerciseIDs):
		return progress.StateLocked
	case review:
		return progress.StateNeedsReview
	case passed == len(w.ExerciseIDs):
		return progress.StatePassed
	}
	return progress.StateAvailable
}

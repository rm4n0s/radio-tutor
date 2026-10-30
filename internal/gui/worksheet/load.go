package worksheet

import (
	"context"
	"io/fs"

	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/student"
)

// exerciseView is what a card shows of one exercise.
type exerciseView struct {
	ID, Label, Title, Prompt string
	State                    progress.ItemState
	// AttemptsLeft counts when Limited.
	AttemptsLeft int
	Limited      bool
	// Hints are the hints the child may see.
	Hints []string
	// Picture is prompt.svg; nil when the exercise has none.
	Picture []byte
	// HasAudio is true when the exercise has prompt.ogg.
	HasAudio bool
}

// sheet is the worksheet as the student sees it.
type sheet struct {
	Exercises []exerciseView
}

// promptFile is the path of a file of an exercise in the cassette's folder.
func promptFile(id, name string) string { return "exercises/" + id + "/" + name }

// loadSheet reads the save file of the student and the prompt files of the
// worksheet's exercises. A save file that was broken is set aside by the
// progress store and the worksheet starts again.
func loadSheet(ctx context.Context, env *app.Env, req student.WorksheetRequest) (*sheet, error) {
	c := req.Cassette
	p, err := env.Progress.Load(ctx, req.Username, c.Manifest.ID)
	if p == nil {
		return nil, err
	}
	if err != nil {
		env.Log.Error("a save file was broken and set aside", "username", req.Username, "cassette", c.Manifest.ID, "err", err)
	}
	states := progress.States(c, p)
	var out sheet
	for _, id := range req.Worksheet.ExerciseIDs {
		ex := c.Exercises[id]
		if ex == nil {
			continue
		}
		v := exerciseView{
			ID: id, Label: ex.Label, Title: ex.Title, Prompt: ex.Prompt,
			State: states[id], Hints: progress.VisibleHints(c, p, id),
		}
		v.AttemptsLeft, v.Limited = progress.AttemptsLeft(c, p, id)
		if data, err := fs.ReadFile(req.FS, promptFile(id, "prompt.svg")); err == nil {
			v.Picture = data
		}
		if info, err := fs.Stat(req.FS, promptFile(id, "prompt.ogg")); err == nil && !info.IsDir() {
			v.HasAudio = true
		}
		out.Exercises = append(out.Exercises, v)
	}
	return &out, nil
}

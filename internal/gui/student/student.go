// Package student is the student area of Radio Tutor (SPEC §13): the shelf of
// cassettes with the import, and the cassette screen with its items, states,
// locks and the Continue button. The lesson player (step 6.4) and the worksheet
// (step 6.5) plug in through Screens.
//
// The screens are widgets on Glutter. They read and write through the stores of
// app.Env on goroutines (State.Go) and hand the results back with SetState.
package student

import (
	"io/fs"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
)

// Keys of the widgets that tests and the later steps look for.
const (
	// KeyImport is the key of the Import a cassette button of the shelf.
	KeyImport = "import"
	// KeyImportCancel is the key of the Cancel button shown while an import runs.
	KeyImportCancel = "import-cancel"
	// KeyCardPrefix starts the key of a cassette card of the shelf; the
	// cassette id follows.
	KeyCardPrefix = "cassette-"
	// KeyProgressPrefix starts the key of the progress bar of a card; the
	// cassette id follows.
	KeyProgressPrefix = "progress-"
	// KeyContinue is the key of the Continue button of the cassette screen.
	KeyContinue = "continue"
	// KeyItemPrefix starts the key of the row of a lesson (the lesson id
	// follows) or of a worksheet (its index follows, see WorksheetKey).
	KeyItemPrefix = "item-"
	// KeyStateSuffix is added to the key of a row to make the key of its state
	// mark (the lock, the check or the flag at the end of the row), which has
	// the state's name as its tooltip. A row that is simply available has none.
	KeyStateSuffix = "-state"
	// KeyExercisePrefix starts the key of the label chip of an exercise in its
	// worksheet row; the exercise id follows.
	KeyExercisePrefix = "exercise-"
	// KeyDialogYes is the key of the confirming button of the upgrade dialog.
	KeyDialogYes = "dialog-yes"
	// KeyDialogNo is the key of the declining button of the upgrade dialog.
	KeyDialogNo = "dialog-no"
	// KeyDialogOK is the key of the OK button of the problems dialog.
	KeyDialogOK = "dialog-ok"
)

// WorksheetKey is the key of the row of the worksheet with the index.
func WorksheetKey(index int) string { return KeyItemPrefix + "worksheet-" + itoa(index) }

// LessonRequest is what the lesson player (step 6.4) gets when the student
// opens a lesson.
type LessonRequest struct {
	// Username is the student, who owns the progress.
	Username string
	// Cassette is the loaded cassette.
	Cassette *cassette.Cassette
	// FS is the cassette's folder: scene.svg and narration.ogg of the lesson.
	FS fs.FS
	// LessonID is the lesson to play.
	LessonID string
	// OpenNext closes the lesson screen and opens the item that follows the
	// lesson in the cassette (the cassette screen reads the progress again
	// first, so the item is unlocked by what the lesson just saved). It is nil
	// when the lesson is the last item. The context is the lesson screen's.
	OpenNext func(bc m.BuildContext)
}

// WorksheetRequest is what the worksheet screen (step 6.5) gets when the
// student opens a worksheet.
type WorksheetRequest struct {
	// Username is the student, who owns the progress.
	Username string
	// Cassette is the loaded cassette.
	Cassette *cassette.Cassette
	// FS is the cassette's folder: the prompt pictures and audio.
	FS fs.FS
	// Worksheet is the worksheet to show.
	Worksheet cassette.Worksheet
	// OpenNext closes the worksheet screen and opens the item that follows the
	// worksheet in the cassette, after its last exercise (the cassette screen
	// reads the progress again first, so the item is unlocked by what the
	// worksheet just saved). It is nil when the worksheet ends the cassette. The
	// context is the worksheet screen's.
	OpenNext func(bc m.BuildContext)
}

// Screens are the screens the student area opens. A nil function gives a
// placeholder that says the screen comes soon; steps 6.4 and 6.5 provide the
// real ones. The function builds the widget of the route; the student area
// pushes it, and the screen goes back with app.Pop. When it pops, the cassette
// screen reads the progress again.
type Screens struct {
	// Lesson builds the lesson player.
	Lesson func(LessonRequest) m.Widget
	// Worksheet builds the worksheet screen.
	Worksheet func(WorksheetRequest) m.Widget
}

// HomeBuilder returns the function the shell calls for the home of a student
// (app.Screens.Student) and that the admin's Play as me pushes: it builds the
// shelf of the username with the screens.
func HomeBuilder(screens Screens) func(username string) m.Widget {
	return func(username string) m.Widget { return NewShelf(username, screens) }
}

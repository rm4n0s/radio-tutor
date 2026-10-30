package app

import (
	"context"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/glutter/desktop"
	m "github.com/rm4n0s/glutter/material"
)

// Screens are the screens that live in other GUI packages. Those packages
// import this one (for the Scope, the navigation and the theme), so this one
// cannot import them: package desktop fills the struct and the shell calls the
// functions. A nil function leaves the placeholder of the shell.
type Screens struct {
	// Student builds the home of a student, the shelf (SPEC §13), for the
	// username. The shell shows it to a logged-in student, and the admin's
	// Play as me pushes it as a route.
	Student func(username string) m.Widget
}

// FileQuestion describes the file dialog to open: its title, the label of the
// accept button and the names of the two filters.
type FileQuestion struct {
	// Title is the title of the dialog.
	Title string
	// AcceptLabel is the text of the accept button.
	AcceptLabel string
	// FilterName is the name of the filter for the files that match Patterns.
	FilterName string
	// Patterns are the globs of the first filter, such as "*.cassette".
	Patterns []string
	// AllFilesName is the name of the second filter, which shows every file.
	AllFilesName string
}

// FileChooser asks the user for one file and returns its path. ok is false when
// the user cancelled. It waits for the user, so it is called on a goroutine, not
// on the UI thread, with the context of the screen (the dialog closes when ctx
// is done). Tests replace it with a function that answers at once.
type FileChooser func(ctx context.Context, bc m.BuildContext, q FileQuestion) (path string, ok bool, err error)

// DesktopFileChooser is the FileChooser that opens the file dialog of the
// desktop (Glutter's desktop.OpenFiles, the XDG file chooser portal). A desktop
// without the portal fails with the tag FileChooserUnavailable of Glutter.
func DesktopFileChooser(ctx context.Context, bc m.BuildContext, q FileQuestion) (string, bool, error) {
	res, err := desktop.OpenFiles(ctx, bc, desktop.OpenFilesOpts{
		Title:       q.Title,
		AcceptLabel: q.AcceptLabel,
		Filters: []desktop.FileFilter{
			{Name: q.FilterName, Patterns: q.Patterns},
			{Name: q.AllFilesName, Patterns: []string{"*"}},
		},
	})
	if err != nil {
		return "", false, err
	}
	if res.Cancelled || len(res.Paths) == 0 {
		return "", false, nil
	}
	return res.Paths[0], true, nil
}

// FileChooserUnavailable reports whether err says that the desktop has no file
// dialog.
func FileChooserUnavailable(err error) bool {
	appErr, ok := errors.FromError(err)
	return ok && appErr.Tag == "FileChooserUnavailable"
}

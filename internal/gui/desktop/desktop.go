// Package desktop wires the screens of the GUI packages into the application
// shell and runs it. The screen packages (student, later admin and player)
// import package app, so app cannot import them; this package imports all of
// them. cmd/radio-tutor calls Run.
package desktop

import (
	"context"

	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/student"
)

// Screens returns the screens of the real application. The steps that add
// screens (the admin area, the lesson player, the worksheet) add them here.
func Screens() app.Screens {
	return app.Screens{
		Student: student.HomeBuilder(student.Screens{}),
	}
}

// Run starts the application with the real screens and blocks until its window
// is closed (app.RunWith).
func Run(ctx context.Context) error {
	return app.RunWith(ctx, app.StartOptions{Screens: Screens()})
}

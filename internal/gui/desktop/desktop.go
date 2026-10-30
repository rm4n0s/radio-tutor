// Package desktop wires the screens of the GUI packages into the application
// shell and runs it. The screen packages (student, later admin and player)
// import package app, so app cannot import them; this package imports all of
// them. cmd/radio-tutor calls Run.
package desktop

import (
	"context"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/gui/admin"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/player"
	"github.com/rm4n0s/radio-tutor/internal/gui/student"
	"github.com/rm4n0s/radio-tutor/internal/gui/svgcheck"
	"github.com/rm4n0s/radio-tutor/internal/gui/worksheet"
)

// Screens returns the screens of the real application. The steps that add
// screens (the admin area, the lesson player, the worksheet) add them here.
func Screens() app.Screens {
	return app.Screens{
		Student: student.HomeBuilder(student.Screens{
			Lesson:    player.Builder(),
			Worksheet: worksheet.Builder(),
		}),
		Admin: admin.HomeBuilder(admin.Screens{
			Progress: func() m.Widget { return admin.NewProgress() },
		}),
	}
}

// Run starts the application with the real screens and blocks until its window
// is closed (app.RunWith). Imported cassettes are render-checked with
// svgcheck.Checker.
func Run(ctx context.Context) error {
	return app.RunWith(ctx, app.StartOptions{Screens: Screens(), Render: svgcheck.Checker{}})
}

package main

import (
	"context"

	"github.com/rm4n0s/radio-tutor/internal/core/menuentry"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

const setupDesktopUsage = "usage: radio-tutor -setup-desktop [-remove]\n"

// menuEntry describes Radio Tutor's entry in the application menu (SPEC §14).
func menuEntry() menuentry.Entry {
	return menuentry.Entry{
		ID:   app.ID,
		Name: app.Title,
		// The name is the same in Greek: it is a name, not a phrase.
		Names:      map[string]string{"el": app.Title},
		Comment:    "Lessons and exercises for children",
		Comments:   map[string]string{"el": "Μαθήματα και ασκήσεις για παιδιά"},
		Categories: []string{"Education"},
		IconPNG:    app.IconPNG(),
		GUIProgram: "radio-tutor.exe",
	}
}

// setupDeps is what setupDesktop needs from its surroundings, so that tests can
// point it at throw-away folders.
type setupDeps struct {
	stdout, stderr *printer
	env            menuentry.Env
}

// setupDesktop runs `radio-tutor -setup-desktop [-remove]` (SPEC §14); args are
// what follows -setup-desktop on the command line, and -remove may also be
// written --remove. It adds the running program to the application menu of the
// current user, or with -remove deletes what it added, prints the files it wrote or deleted and
// returns the exit code. It does not open the window, the log or the data
// folders.
func setupDesktop(ctx context.Context, args []string, d setupDeps) int {
	remove := false
	switch {
	case len(args) == 0:
	case len(args) == 1 && (args[0] == "-remove" || args[0] == "--remove"):
		remove = true
	default:
		d.stderr.printf("%s", setupDesktopUsage)
		return exitUsage
	}

	if remove {
		removed, err := menuentry.Remove(ctx, menuEntry(), d.env)
		if err != nil {
			d.stderr.printf("radio-tutor -setup-desktop: %v\n", err)
			return exitError
		}
		if len(removed) == 0 {
			d.stdout.printf("Radio Tutor was not in the application menu: nothing to remove\n")
			return exitOK
		}
		d.stdout.printf("Removed Radio Tutor from the application menu:\n")
		for _, path := range removed {
			d.stdout.printf("  %s\n", path)
		}
		return exitOK
	}

	written, err := menuentry.Install(ctx, menuEntry(), d.env)
	if err != nil {
		d.stderr.printf("radio-tutor -setup-desktop: %v\n", err)
		return exitError
	}
	d.stdout.printf("Added Radio Tutor to the application menu:\n")
	for _, path := range written {
		d.stdout.printf("  %s\n", path)
	}
	return exitOK
}

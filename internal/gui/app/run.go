package app

import (
	"context"
	"log/slog"
	"os"

	"github.com/rm4n0s/errors"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
)

// ID is the application ID of Radio Tutor: the app_id of its windows on Wayland, the
// class of its windows on X11, the AppUserModelID on Windows, and the name of its
// icon and desktop entry (radio-tutor setup-desktop, SPEC §14).
const ID = "dev.radiotutor.app"

// Run starts the application and blocks until its window is closed or ctx is
// done (SIGTERM and SIGINT cancel it, see QuitOnDone): it calls
// Start, opens the window with the Root and releases the lock and the log at the
// end. When another instance holds the lock it shows a small window that says
// "Radio Tutor is already running" (so a launcher without a terminal is not
// silent) and returns the storage error AlreadyRunning, which the command turns
// into the message on stderr and exit code 2 (SPEC §4).
//
// Run shows the placeholder screens of this package; package desktop calls
// RunWith with the real ones.
func Run(ctx context.Context) error { return RunWith(ctx, StartOptions{}) }

// RunWith is Run with the options of Start (the screens, the SVG checker).
func RunWith(ctx context.Context, opts StartOptions) error {
	env, err := Start(ctx, opts)
	if err != nil {
		if appErr, ok := errors.FromError(err); ok && appErr.Tag == "AlreadyRunning" {
			// Failing to show the window (no display, a terminal run) must not
			// hide the AlreadyRunning error, so the window's own error is dropped.
			_ = m.RunApp(NewQuitOnDone(ctx, NewAlreadyRunning(i18n.FromLocale(processLocale()))), windowOpts(Title, AlreadyRunningSize, AlreadyRunningSize, nil))
		}
		return err
	}
	defer env.Close()
	minSize := m.Size{Width: MinWindowWidth, Height: MinWindowHeight}
	return m.RunApp(NewQuitOnDone(ctx, NewRoot(env)), windowOpts(Title, minSize, minSize, env))
}

// windowOpts are the RunApp options of a window.
func windowOpts(title string, size, minSize m.Size, env *Env) m.RunAppOpts {
	opts := m.RunAppOpts{
		AppID:  ID,
		Window: m.WindowConfig{Title: title, Size: size, MinSize: minSize},
	}
	logger := slog.Default()
	if env != nil {
		opts.Logger = env.Log
		logger = env.Log
	}
	// The embedded icon is checked by a test; a window without an icon still works.
	icon, err := windowIcon()
	if err != nil {
		logger.Error("window icon", "err", err)
	} else {
		opts.Window.Icon = icon
	}
	return opts
}

// processLocale is the locale of the process: LANG, or what the operating system
// reports when LANG is empty.
func processLocale() string {
	if locale := os.Getenv("LANG"); locale != "" {
		return locale
	}
	return systemLocale()
}

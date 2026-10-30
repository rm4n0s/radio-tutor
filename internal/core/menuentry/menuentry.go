// Package menuentry adds Radio Tutor to the application menu of the desktop and
// removes it again (SPEC §14, "radio-tutor -setup-desktop"). On Linux it writes the
// icon pictures under $XDG_DATA_HOME/icons/hicolor and a Desktop Entry file under
// $XDG_DATA_HOME/applications; on Windows it makes a shortcut in the per-user
// Start menu. The code that differs between the systems lives in the files ending
// in _linux.go and _windows.go (SPEC §5). The package knows nothing of the
// application's name or icon: the caller passes them in an Entry.
package menuentry

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/rm4n0s/errors"
)

// Entry describes the menu entry to make.
type Entry struct {
	// ID is the application ID (the Wayland app_id and the X11 WM_CLASS of the
	// window, the AppUserModelID of Windows). On Linux it names the icon and the
	// .desktop file, so it must be a plain file name.
	ID string
	// Name is the name shown in the menu.
	Name string
	// Names are the names in other languages, by language code ("el").
	Names map[string]string
	// Comment is the one-line description.
	Comment string
	// Comments are the descriptions in other languages, by language code.
	Comments map[string]string
	// Categories are the menu categories of the Desktop Entry (Linux).
	Categories []string
	// IconPNG is a square PNG, at least as large as the largest icon size.
	IconPNG []byte
	// GUIProgram is the file name of the program the Start-menu shortcut starts
	// (Windows): it must be in the folder of the running program.
	GUIProgram string
}

// Env is what Install and Remove need from the system, so that tests can point
// them at throw-away folders.
type Env struct {
	// Executable returns the path of the running program (os.Executable).
	Executable func() (string, error)
	// Platform holds what only one operating system needs.
	Platform PlatformEnv
}

// DefaultEnv returns the real Env: the running program and the folders of the
// current user.
func DefaultEnv() Env {
	return Env{Executable: os.Executable, Platform: defaultPlatformEnv()}
}

// Install adds the entry to the menu of the current user and returns the paths of
// the files it wrote, in the order it wrote them. Running it again replaces them.
// The entry points at the running program, with symbolic links resolved; a
// program that was started with "go run" or "go test" is refused (it is deleted
// when the command ends), with the tag ExecutableTemporary. Other tags:
// EntryInvalid, ExecutableUnknown, ExecutableResolveFailed, and those of the
// operating system's part (the files of Linux, the shortcut of Windows).
func Install(ctx context.Context, e Entry, env Env) ([]string, error) {
	if err := e.validate(); err != nil {
		return nil, err
	}
	exe, err := runningProgram(env)
	if err != nil {
		return nil, err
	}
	return install(ctx, e, exe, env.Platform)
}

// Remove deletes exactly the files Install writes; a file that is not there is
// fine. It returns the paths it deleted. It fails with the tag EntryInvalid and
// the tags of the operating system's part.
func Remove(ctx context.Context, e Entry, env Env) ([]string, error) {
	if err := e.validate(); err != nil {
		return nil, err
	}
	return remove(ctx, e, env.Platform)
}

// validate checks the fields that become file names.
func (e Entry) validate() error {
	if e.ID == "" || e.ID == "." || e.ID == ".." || strings.ContainsAny(e.ID, `/\`+"\x00") || strings.TrimSpace(e.ID) != e.ID {
		return errors.New("EntryInvalid", "the application ID must be a plain file name", "id", e.ID)
	}
	if e.Name == "" || strings.ContainsAny(e.Name, "\r\n\x00") {
		return errors.New("EntryInvalid", "the application name must be one line of text", "name", e.Name)
	}
	return nil
}

// runningProgram returns the absolute path of the running program with symbolic
// links resolved, and refuses a temporary build.
func runningProgram(env Env) (string, error) {
	exe, err := env.Executable()
	if err != nil {
		return "", errors.NewErr("ExecutableUnknown", err)
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return "", errors.NewErr("ExecutableResolveFailed", err).SetMetadata("path", exe)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", errors.NewErr("ExecutableResolveFailed", err).SetMetadata("path", abs)
	}
	if isTemporaryBuild(resolved, os.Getenv("GOCACHE")) {
		return "", errors.New("ExecutableTemporary",
			"this is a temporary program made by \"go run\" or \"go test\"; build it or run \"go install\" first, then run the installed program",
			"path", resolved)
	}
	return resolved, nil
}

// isTemporaryBuild reports whether path is inside a folder where the go command
// builds programs that it deletes afterwards: a folder named go-build... (the
// temporary folder of "go run" and "go test", and the build cache) or the folder
// in the GOCACHE variable.
func isTemporaryBuild(path, gocache string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if strings.HasPrefix(part, "go-build") {
			return true
		}
	}
	if gocache != "" && filepath.IsAbs(gocache) {
		rel, err := filepath.Rel(filepath.Clean(gocache), path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

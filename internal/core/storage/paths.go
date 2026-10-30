// Package storage resolves Radio Tutor's folders (the XDG variables on Linux,
// %LOCALAPPDATA% on Windows) and reads and writes its files the way SPEC §4
// requires: JSON documents are written atomically, a JSON file that cannot be
// parsed is quarantined, and an exclusive lock keeps a single instance running.
// The code that differs between the operating systems lives in the files
// ending in _linux.go and _windows.go (SPEC §5).
package storage

import (
	"os"
	"path/filepath"
)

const (
	// DirMode is the mode of every folder Radio Tutor creates.
	DirMode = 0o700
	// FileMode is the mode of every file Radio Tutor creates.
	FileMode = 0o600

	// appFolder is the folder Radio Tutor uses inside each base folder (each XDG
	// base folder on Linux, %LOCALAPPDATA% on Windows).
	appFolder = "radio-tutor"
)

// Paths holds the three folders of SPEC §4: the settings folder, the data
// folder and the state folder. On Linux they are $XDG_CONFIG_HOME/radio-tutor,
// $XDG_DATA_HOME/radio-tutor and $XDG_STATE_HOME/radio-tutor; on Windows they
// are the config, data and state folders of %LOCALAPPDATA%\radio-tutor.
// Resolve only computes the paths; it creates nothing.
type Paths struct {
	// Config is the folder of settings.json and providers.json.
	Config string
	// Data is the folder of the profiles, cassettes, progress and photos.
	Data string
	// State is the folder of the log and the lock file.
	State string
}

// Resolve computes the Paths from the process environment (see ResolveFrom for
// the rules of each operating system). It creates nothing.
func Resolve() (Paths, error) {
	return ResolveFrom(os.Getenv)
}

// SettingsFile is the app settings file (SPEC §12.4).
func (p Paths) SettingsFile() string { return filepath.Join(p.Config, "settings.json") }

// ProvidersFile is the AI providers file, which holds API keys (SPEC §11.6).
func (p Paths) ProvidersFile() string { return filepath.Join(p.Config, "providers.json") }

// ProfilesFile is the profiles file (SPEC §3.1).
func (p Paths) ProfilesFile() string { return filepath.Join(p.Data, "profiles.json") }

// CassettesDir is the folder of the installed cassettes, one folder per
// cassette id (SPEC §6.2).
func (p Paths) CassettesDir() string { return filepath.Join(p.Data, "cassettes") }

// ProgressDir is the folder of the save files: progress/<username>/<cassette-id>.json
// (SPEC §7.4).
func (p Paths) ProgressDir() string { return filepath.Join(p.Data, "progress") }

// PhotosDir is the folder of the submitted photos:
// photos/<username>/<cassette-id>/<submission-id>/<n>.jpg.
func (p Paths) PhotosDir() string { return filepath.Join(p.Data, "photos") }

// LogFile is the log file (SPEC §15).
func (p Paths) LogFile() string { return filepath.Join(p.State, "radio-tutor.log") }

// LockFile is the single-instance lock file.
func (p Paths) LockFile() string { return filepath.Join(p.State, "lock") }

// Package storage resolves Radio Tutor's folders from the XDG variables and
// reads and writes its files the way SPEC §4 requires: JSON documents are
// written atomically, a JSON file that cannot be parsed is quarantined, and an
// exclusive lock keeps a single instance running.
package storage

import (
	"os"
	"path/filepath"

	"github.com/rm4n0s/errors"
)

const (
	// DirMode is the mode of every folder Radio Tutor creates.
	DirMode = 0o700
	// FileMode is the mode of every file Radio Tutor creates.
	FileMode = 0o600

	// appFolder is the folder Radio Tutor uses inside each XDG base folder.
	appFolder = "radio-tutor"
)

// Paths holds the three folders of SPEC §4: the settings folder
// ($XDG_CONFIG_HOME/radio-tutor), the data folder ($XDG_DATA_HOME/radio-tutor)
// and the state folder ($XDG_STATE_HOME/radio-tutor). Resolve only computes the
// paths; it creates nothing.
type Paths struct {
	// Config is the folder of settings.json and providers.json.
	Config string
	// Data is the folder of the profiles, cassettes, progress and photos.
	Data string
	// State is the folder of the log and the lock file.
	State string
}

// Resolve computes the Paths from the process environment: XDG_CONFIG_HOME
// (default ~/.config), XDG_DATA_HOME (default ~/.local/share) and
// XDG_STATE_HOME (default ~/.local/state), each followed by "radio-tutor". As
// the XDG specification says, a variable that is empty or holds a relative path
// is ignored.
func Resolve() (Paths, error) {
	return ResolveFrom(os.Getenv)
}

// ResolveFrom is Resolve with the environment given as a lookup function. It
// fails with the tag HomeUnknown when a default is needed and HOME is not an
// absolute path.
func ResolveFrom(getenv func(string) string) (Paths, error) {
	home := getenv("HOME")
	dir := func(variable, fallback string) (string, error) {
		if v := getenv(variable); filepath.IsAbs(v) {
			return filepath.Join(v, appFolder), nil
		}
		if !filepath.IsAbs(home) {
			return "", errors.New("HomeUnknown", "cannot find the home folder: HOME is not an absolute path",
				"variable", variable, "home", home)
		}
		return filepath.Join(home, fallback, appFolder), nil
	}

	config, err := dir("XDG_CONFIG_HOME", ".config")
	if err != nil {
		return Paths{}, err
	}
	data, err := dir("XDG_DATA_HOME", filepath.Join(".local", "share"))
	if err != nil {
		return Paths{}, err
	}
	state, err := dir("XDG_STATE_HOME", filepath.Join(".local", "state"))
	if err != nil {
		return Paths{}, err
	}
	return Paths{Config: config, Data: data, State: state}, nil
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

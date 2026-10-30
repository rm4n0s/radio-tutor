//go:build !windows

package storage

import (
	"path/filepath"

	"github.com/rm4n0s/errors"
)

// ResolveFrom is Resolve with the environment given as a lookup function. On
// Linux the folders are XDG_CONFIG_HOME (default ~/.config), XDG_DATA_HOME
// (default ~/.local/share) and XDG_STATE_HOME (default ~/.local/state), each
// followed by "radio-tutor". As the XDG specification says, a variable that is
// empty or holds a relative path is ignored. It fails with the tag HomeUnknown
// when a default is needed and HOME is not an absolute path.
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

//go:build windows

package storage

import (
	"path/filepath"

	"github.com/rm4n0s/errors"
)

// ResolveFrom is Resolve with the environment given as a lookup function. On
// Windows everything lives under %LOCALAPPDATA%\radio-tutor (SPEC §4): the
// settings and providers in "config", the profiles, cassettes, progress and
// photos in "data", the log and the lock in "state". The folders are local, not
// roaming: providers.json holds API keys and the photos show a child's work,
// and neither belongs on a domain server or in OneDrive. When LOCALAPPDATA is
// empty or relative, USERPROFILE\AppData\Local is used. It fails with the tag
// LocalAppDataUnknown when neither is an absolute path.
func ResolveFrom(getenv func(string) string) (Paths, error) {
	base := getenv("LOCALAPPDATA")
	if !filepath.IsAbs(base) {
		profile := getenv("USERPROFILE")
		if !filepath.IsAbs(profile) {
			return Paths{}, errors.New("LocalAppDataUnknown",
				"cannot find the local application data folder: LOCALAPPDATA and USERPROFILE are not absolute paths",
				"LOCALAPPDATA", base, "USERPROFILE", profile)
		}
		base = filepath.Join(profile, "AppData", "Local")
	}
	root := filepath.Join(base, appFolder)
	return Paths{
		Config: filepath.Join(root, "config"),
		Data:   filepath.Join(root, "data"),
		State:  filepath.Join(root, "state"),
	}, nil
}

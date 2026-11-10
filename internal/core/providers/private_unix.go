//go:build !windows

package providers

import (
	"io/fs"

	"github.com/rm4n0s/errors"
)

// checkPrivate refuses a providers file that group or others can access. It
// fails with the tag ProvidersFileTooOpen.
func checkPrivate(path string, info fs.FileInfo) error {
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return errors.New("ProvidersFileTooOpen",
			"providers.json holds API keys but other users can read it; make it private with: chmod 600 "+path,
			"path", path, "mode", perm.String())
	}
	return nil
}

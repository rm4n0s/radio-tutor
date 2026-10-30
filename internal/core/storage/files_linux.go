//go:build !windows

package storage

import "os"

// renameFile moves oldPath over newPath, replacing it.
func renameFile(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

// syncDirectory flushes the folder's entries to disk, so that a rename in it
// survives a crash.
func syncDirectory(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	if err := d.Sync(); err != nil {
		_ = d.Close() // the sync error is the one to report
		return err
	}
	return d.Close()
}

// protectFile makes the file private to its owner. On Linux the file already has
// mode 0600 (FileMode), which is that, so there is nothing to do.
func protectFile(string) error { return nil }

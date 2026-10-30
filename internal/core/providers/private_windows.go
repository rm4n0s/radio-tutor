//go:build windows

package providers

import "io/fs"

// checkPrivate accepts the file: Windows has no permission bits to look at (the
// mode it reports is the same for every file), and the file is written with a
// DACL for the current user only (storage.WritePrivateJSONAtomic).
func checkPrivate(string, fs.FileInfo) error { return nil }

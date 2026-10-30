package storage

import "strings"

// ReservedName reports whether name, one element of a path (a folder or file
// name), is one Windows reserves for a device: CON, PRN, AUX, NUL, COM1 to COM9
// and LPT1 to LPT9, in any letter case, with or without an extension (Windows
// treats "con.txt" like "con"), and with trailing spaces. Such a name cannot be
// a file or folder on Windows, so ids and usernames that become folder names
// must not be one on any system (SPEC D25), or a profile or a cassette made on
// Linux could not be copied to Windows.
func ReservedName(name string) bool {
	base := name
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	base = strings.ToUpper(strings.TrimRight(base, " "))
	switch base {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) {
		return base[3] >= '1' && base[3] <= '9'
	}
	return false
}

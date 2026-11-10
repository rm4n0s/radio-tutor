//go:build !windows

package app

// systemLocale returns the locale the operating system itself reports, for when
// the LANG variable is empty. Linux has nothing but the environment, so it is
// empty.
func systemLocale() string { return "" }

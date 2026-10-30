//go:build windows

package app

import (
	"syscall"
	"unsafe"
)

// localeNameMaxLength is LOCALE_NAME_MAX_LENGTH of winnls.h, in UTF-16 units.
const localeNameMaxLength = 85

var procGetUserDefaultLocaleName = syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")

// systemLocale returns the name of the user's locale ("el-GR", "en-US"), for
// when the LANG variable is empty, which it always is on Windows unless a
// developer sets it. It returns "" when the call fails (i18n.FromLocale then
// chooses English).
func systemLocale() string {
	var buf [localeNameMaxLength]uint16
	n, _, _ := procGetUserDefaultLocaleName.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

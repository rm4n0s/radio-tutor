//go:build windows

package phone

import (
	"github.com/rm4n0s/errors"
	"golang.org/x/sys/windows"
)

// isAddrInUse reports whether err from a listen says that the address is taken.
// Winsock reports WSAEADDRINUSE (10048), which Go's syscall.EADDRINUSE does not
// match on Windows.
func isAddrInUse(err error) bool {
	return errors.Is(err, windows.WSAEADDRINUSE)
}

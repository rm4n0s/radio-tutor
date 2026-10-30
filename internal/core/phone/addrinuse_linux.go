//go:build !windows

package phone

import (
	"syscall"

	"github.com/rm4n0s/errors"
)

// isAddrInUse reports whether err from a listen says that the address is taken
// (EADDRINUSE).
func isAddrInUse(err error) bool { return errors.Is(err, syscall.EADDRINUSE) }

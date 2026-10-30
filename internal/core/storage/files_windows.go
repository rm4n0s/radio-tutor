//go:build windows

package storage

import (
	"os"
	"time"

	"github.com/rm4n0s/errors"
	"golang.org/x/sys/windows"
)

// fileAllAccess is FILE_ALL_ACCESS (winnt.h): every right a file has.
const fileAllAccess = 0x1F01FF

// renameRetries and renameRetryWait bound how long renameFile waits for a
// reader to let go of the destination.
const (
	renameRetries   = 20
	renameRetryWait = 10 * time.Millisecond
)

// renameFile moves oldPath over newPath, replacing it. On Windows a file that
// another process has open (a backup tool, a virus scanner, a reader of the
// moment) cannot be replaced: MoveFileEx fails with a sharing violation or
// "access denied" until the other side closes it. The rename is retried for a
// short while, about 200 ms, before it gives up.
func renameFile(oldPath, newPath string) error {
	var err error
	for i := 0; i < renameRetries; i++ {
		if err = os.Rename(oldPath, newPath); err == nil {
			return nil
		}
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return err
		}
		time.Sleep(renameRetryWait)
	}
	return err
}

// syncDirectory does nothing on Windows: a folder cannot be flushed (opening it
// for FlushFileBuffers is refused) and NTFS journals its metadata, so a rename
// that returned is not lost by a crash.
func syncDirectory(string) error { return nil }

// protectFile gives the file a protected DACL that grants full access to the
// current user only: no inherited entry survives, so neither other users of the
// PC nor the Users group see it (SYSTEM and administrators can still take
// ownership, as on every Windows file). The error comes from the Win32 call.
func protectFile(path string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: fileAllAccess,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.NO_INHERITANCE,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
		},
	}}, nil)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}

//go:build windows

package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ownerOnly reads the security descriptor of the file back and reports what
// the test needs to know about its DACL.
func ownerOnly(t *testing.T, path string) (protected bool, sids []*windows.SID, rights []uint32) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatalf("GetNamedSecurityInfo: %v", err)
	}
	control, _, err := sd.Control()
	if err != nil {
		t.Fatalf("Control: %v", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatalf("DACL: %v", err)
	}
	sids, rights = entriesOf(t, dacl)
	return control&windows.SE_DACL_PROTECTED != 0, sids, rights
}

// entriesOf lists the SIDs and the rights of the allow entries of a DACL.
func entriesOf(t *testing.T, dacl *windows.ACL) (sids []*windows.SID, rights []uint32) {
	t.Helper()
	if dacl == nil {
		t.Fatal("NULL DACL, which gives everyone access")
	}
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			t.Fatalf("GetAce(%d): %v", i, err)
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			t.Errorf("ACE %d has type %d, want an allow entry", i, ace.Header.AceType)
		}
		sids = append(sids, (*windows.SID)(unsafe.Pointer(&ace.SidStart)))
		rights = append(rights, uint32(ace.Mask))
	}
	return sids, rights
}

func currentSID(t *testing.T) *windows.SID {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	return user.User.Sid
}

// underWine reports whether the test runs in Wine. Wine does not keep the DACL
// of a file: it turns the descriptor into Unix mode bits and builds one back from
// them (SYSTEM, the owner, Everyone), so a protected DACL cannot be read back
// there. The tests that read a file's descriptor are for a real Windows (and
// SPEC §16 and docs/MANUAL-CHECKS.md say so); under Wine the list of entries
// that protectFile hands to Windows is checked instead.
func underWine() bool {
	return windows.NewLazySystemDLL("ntdll.dll").NewProc("wine_get_version").Find() == nil
}

// The access list that protectFile sets has one entry: the current user, with
// full access.
func TestOwnerOnlyACL(t *testing.T) {
	acl, err := ownerOnlyACL()
	if err != nil {
		t.Fatalf("ownerOnlyACL: %v", err)
	}
	sids, rights := entriesOf(t, acl)
	if len(sids) != 1 {
		t.Fatalf("the list has %d entries, want 1 (the current user)", len(sids))
	}
	if !sids[0].Equals(currentSID(t)) {
		t.Errorf("the entry is for %s, want the current user %s", sids[0], currentSID(t))
	}
	if rights[0] != fileAllAccess {
		t.Errorf("the entry grants %#x, want full access %#x", rights[0], fileAllAccess)
	}
}

// On Windows the DACL of a private file, read back from the file, is protected
// and has that one entry.
func TestProtectFileGrantsOnlyTheUser(t *testing.T) {
	if underWine() {
		t.Skip("Wine keeps no DACL: it rebuilds one from the Unix mode bits (docs/MANUAL-CHECKS.md W.1 checks it on Windows)")
	}
	path := filepath.Join(t.TempDir(), "secret.json")
	if err := os.WriteFile(path, []byte("{}"), FileMode); err != nil {
		t.Fatal(err)
	}
	if err := protectFile(path); err != nil {
		t.Fatalf("protectFile: %v", err)
	}
	protected, sids, rights := ownerOnly(t, path)
	if !protected {
		t.Error("the DACL is not protected: it still inherits entries from the folder")
	}
	if len(sids) != 1 {
		t.Fatalf("the DACL has %d entries, want 1 (the current user)", len(sids))
	}
	if !sids[0].Equals(currentSID(t)) {
		t.Errorf("the entry is for %s, want the current user %s", sids[0], currentSID(t))
	}
	if rights[0] != fileAllAccess {
		t.Errorf("the entry grants %#x, want full access %#x", rights[0], fileAllAccess)
	}
}

// protectFile succeeds on a file and leaves it usable by its owner, also under
// Wine.
func TestProtectFileKeepsTheFileUsable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.json")
	if err := os.WriteFile(path, []byte("{}"), FileMode); err != nil {
		t.Fatal(err)
	}
	if err := protectFile(path); err != nil {
		t.Fatalf("protectFile: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"a":1}`), FileMode); err != nil {
		t.Errorf("the owner cannot write the protected file: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != `{"a":1}` {
		t.Errorf("read back %q, %v", data, err)
	}
}

func TestWritePrivateJSONAtomicProtectsTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "providers.json")
	for i := 0; i < 2; i++ { // the second write replaces the first
		if err := WritePrivateJSONAtomic(context.Background(), path, doc{Format: 1, Name: "key"}); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		if underWine() {
			continue // see underWine
		}
		protected, sids, _ := ownerOnly(t, path)
		if !protected || len(sids) != 1 || !sids[0].Equals(currentSID(t)) {
			t.Errorf("write %d: protected=%v, %d entries", i, protected, len(sids))
		}
	}
	var got doc
	if err := ReadJSON(path, &got); err != nil || got.Name != "key" {
		t.Errorf("ReadJSON: %+v, %v", got, err)
	}
}

// A plain WriteJSONAtomic does not protect the file: it takes the folder's ACL,
// so the test above proves something.
func TestWriteJSONAtomicKeepsTheFoldersACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.json")
	if err := WriteJSONAtomic(context.Background(), path, doc{Format: 1}); err != nil {
		t.Fatal(err)
	}
	if underWine() {
		t.Skip("Wine keeps no DACL")
	}
	if protected, _, _ := ownerOnly(t, path); protected {
		t.Error("a file written with WriteJSONAtomic has a protected DACL")
	}
}

func TestProtectFileFailure(t *testing.T) {
	err := protectFile(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("protectFile of a missing file succeeded")
	}
}

// The rename of an atomic write is retried while the old file is open elsewhere
// without sharing, and gives up with the error when the other side never lets go.
func TestRenameFileRetriesWhileTheTargetIsOpen(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("old"), FileMode); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("new"), FileMode); err != nil {
		t.Fatal(err)
	}
	// Open the target with no share mode, as a scanner may: the rename fails
	// until the handle is closed.
	name, err := windows.UTF16PtrFromString(target)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- renameFile(src, target) }()
	// Let the first attempts fail, then release the handle.
	windows.SleepEx(50, false)
	if err := windows.CloseHandle(h); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("renameFile after the handle closed: %v", err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "new" {
		t.Errorf("target = %q, %v", data, err)
	}
}

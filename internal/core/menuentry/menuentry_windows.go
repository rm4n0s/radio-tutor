//go:build windows

package menuentry

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/rm4n0s/errors"
	"golang.org/x/sys/windows"
)

// PlatformEnv is what the Windows part needs from the system.
type PlatformEnv struct {
	// ProgramsDir returns the folder of the per-user Start menu that holds
	// the program shortcuts (FOLDERID_Programs).
	ProgramsDir func() (string, error)
}

func defaultPlatformEnv() PlatformEnv { return PlatformEnv{ProgramsDir: programsDir} }

// programsDir asks the shell for the Programs folder of the user's Start menu
// (SHGetKnownFolderPath), as %APPDATA%\Microsoft\Windows\Start Menu\Programs
// can be moved or redirected. It fails with the tag ProgramsDirUnknown.
func programsDir() (string, error) {
	return programsDirFrom(func() (string, error) {
		return windows.KnownFolderPath(windows.FOLDERID_Programs, windows.KF_FLAG_DEFAULT)
	})
}

// programsDirFrom is programsDir with the shell's answer given as a function.
func programsDirFrom(known func() (string, error)) (string, error) {
	dir, err := known()
	if err != nil {
		return "", errors.NewErr("ProgramsDirUnknown", err)
	}
	return dir, nil
}

// shortcutPath is the .lnk file of the entry.
func shortcutPath(e Entry, p PlatformEnv) (string, error) {
	if strings.ContainsAny(e.Name, `<>:"/\|?*`) {
		return "", errors.New("EntryInvalid", "the application name cannot be the name of a shortcut file: it has a character Windows does not allow in file names", "name", e.Name)
	}
	dir, err := p.ProgramsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, e.Name+".lnk"), nil
}

// install makes the Start-menu shortcut. It starts the e.GUIProgram that is in
// the folder of exe (the running program is the console build, which would open
// a console window), with that folder as its working folder and its own icon.
// The shortcut gets the AppUserModelID e.ID, which Glutter gives the windows of
// the application, so that the taskbar groups them under the shortcut.
func install(ctx context.Context, e Entry, exe string, p PlatformEnv) ([]string, error) {
	if e.GUIProgram == "" || e.GUIProgram != filepath.Base(e.GUIProgram) {
		return nil, errors.New("EntryInvalid", "the program to start must be a file name", "program", e.GUIProgram)
	}
	link, err := shortcutPath(e, p)
	if err != nil {
		return nil, err
	}
	folder := filepath.Dir(exe)
	target := filepath.Join(folder, e.GUIProgram)
	if st, err := os.Stat(target); err != nil || st.IsDir() {
		return nil, errors.New("GUIProgramMissing",
			"the program the shortcut starts is not next to this one", "path", target)
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.NewErr("InstallCancelled", err)
	}
	dir := filepath.Dir(link)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, errors.NewErr("ProgramsDirCreateFailed", err).SetMetadata("path", dir)
	}
	// The shortcut is saved under a temporary name and renamed over the old one,
	// so that a failure leaves the previous shortcut as it was.
	tmp := link + ".tmp"
	if err := saveShortcut(tmp, shortcut{target: target, folder: folder, description: e.Comment, appID: e.ID}); err != nil {
		_ = os.Remove(tmp) // best effort: the half-written file is garbage
		return nil, err
	}
	if err := os.Rename(tmp, link); err != nil {
		_ = os.Remove(tmp) // best effort, as above
		return nil, errors.NewErr("ShortcutReplaceFailed", err).SetMetadata("path", link)
	}
	return []string{link}, nil
}

// remove deletes the shortcut; a missing one is fine.
func remove(_ context.Context, e Entry, p PlatformEnv) ([]string, error) {
	link, err := shortcutPath(e, p)
	if err != nil {
		return nil, err
	}
	err = os.Remove(link)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.NewErr("EntryFileRemoveFailed", err).SetMetadata("path", link)
	}
	return []string{link}, nil
}

// shortcut is what a Start-menu shortcut holds.
type shortcut struct {
	target      string // the program
	folder      string // its working folder
	description string // the tooltip
	appID       string // the AppUserModelID
}

// The COM identifiers of the shell's link object and its interfaces, and the
// property that holds the AppUserModelID (propkey.h, shobjidl.h).
var (
	clsidShellLink   = windows.GUID{Data1: 0x00021401, Data2: 0, Data3: 0, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidShellLinkW    = windows.GUID{Data1: 0x000214F9, Data2: 0, Data3: 0, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidPersistFile   = windows.GUID{Data1: 0x0000010B, Data2: 0, Data3: 0, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidPropertyStore = windows.GUID{Data1: 0x886D8EEB, Data2: 0x8CF2, Data3: 0x4446, Data4: [8]byte{0x8D, 0x02, 0xCD, 0xBA, 0x1D, 0xBD, 0xCF, 0x99}}
	pkeyAppUserModel = propertyKey{fmtid: windows.GUID{Data1: 0x9F4C2855, Data2: 0x9F79, Data3: 0x4B39, Data4: [8]byte{0xA8, 0xD0, 0xE1, 0xD4, 0x2D, 0xE1, 0xD5, 0xF3}}, pid: 5}
)

// Positions of the methods in the tables of the interfaces (each table starts
// with the three methods of IUnknown).
const (
	vtQueryInterface = 0
	vtRelease        = 2

	vtLinkSetDescription      = 7
	vtLinkSetWorkingDirectory = 9
	vtLinkSetIconLocation     = 17
	vtLinkSetPath             = 20

	vtFileSave = 6

	vtStoreSetValue = 6
	vtStoreCommit   = 7
)

const (
	vtLPWSTR = 31 // VT_LPWSTR
	// coinitApartmentThreaded is COINIT_APARTMENTTHREADED.
	coinitApartmentThreaded = 0x2
	// sFalse is S_FALSE: CoInitializeEx says so when the thread was initialized already.
	sFalse = 1
	// rpcEChangedMode is RPC_E_CHANGED_MODE: the thread uses another apartment type already.
	rpcEChangedMode = 0x80010106
)

// propertyKey is a PROPERTYKEY.
type propertyKey struct {
	fmtid windows.GUID
	pid   uint32
}

// propVariant is a PROPVARIANT that holds a string pointer (VT_LPWSTR): the type,
// three reserved words, the pointer and the padding that makes it as large as the
// union of the real structure (16 bytes on 32-bit systems, 24 on 64-bit).
type propVariant struct {
	vt  uint16
	_   [3]uint16
	str *uint16
	_   uintptr
}

// object is a COM object: its first word points to the table of its methods.
type object struct {
	vtbl *[32]uintptr
}

// call calls the method at position index of the object's table with the given
// arguments and returns the HRESULT.
//
//go:uintptrescapes
func (o *object) call(index int, args ...uintptr) error {
	all := make([]uintptr, 0, len(args)+1)
	all = append(all, uintptr(unsafe.Pointer(o)))
	all = append(all, args...)
	r, _, _ := syscall.SyscallN(o.vtbl[index], all...)
	if int32(r) < 0 {
		return syscall.Errno(uint32(r))
	}
	return nil
}

// release calls IUnknown::Release.
func (o *object) release() {
	_, _, _ = syscall.SyscallN(o.vtbl[vtRelease], uintptr(unsafe.Pointer(o)))
}

// queryInterface asks the object for another of its interfaces.
func (o *object) queryInterface(iid *windows.GUID) (*object, error) {
	var out *object
	if err := o.call(vtQueryInterface, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out))); err != nil {
		return nil, err
	}
	return out, nil
}

var procCoCreateInstance = windows.NewLazySystemDLL("ole32.dll").NewProc("CoCreateInstance")

// createShellLink creates the shell's link object (CLSID_ShellLink) with the
// IShellLinkW interface.
func createShellLink() (*object, error) {
	var out *object
	r, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidShellLink)), 0, windows.CLSCTX_INPROC_SERVER,
		uintptr(unsafe.Pointer(&iidShellLinkW)), uintptr(unsafe.Pointer(&out)))
	if int32(r) < 0 {
		return nil, syscall.Errno(uint32(r))
	}
	return out, nil
}

// initCOM prepares the calling thread for COM and returns the function that
// undoes it. A thread that is prepared already, in the same or in another mode,
// is used as it is.
func initCOM() (func(), error) {
	err := windows.CoInitializeEx(0, coinitApartmentThreaded)
	switch {
	case err == nil || err == syscall.Errno(sFalse):
		return windows.CoUninitialize, nil
	case err == syscall.Errno(rpcEChangedMode):
		return func() {}, nil
	}
	return nil, err
}

// saveShortcut writes the shortcut file at path. It fails with the tag
// ShortcutFailed, whose "step" metadata says which call of the shell failed.
func saveShortcut(path string, s shortcut) error {
	if step, err := writeShortcut(path, s); err != nil {
		return errors.NewErr("ShortcutFailed", err).SetMetadata("step", step, "path", path)
	}
	return nil
}

// writeShortcut makes the shell's link object (IShellLinkW), fills it and saves it
// with IPersistFile. On a failure it returns the name of the step that failed.
func writeShortcut(path string, s shortcut) (step string, err error) {
	// COM objects belong to the thread that made them.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	uninit, err := initCOM()
	if err != nil {
		return "CoInitializeEx", err
	}
	defer uninit()

	link, err := createShellLink()
	if err != nil {
		return "CoCreateInstance", err
	}
	defer link.release()

	for _, step := range []struct {
		name  string
		index int
		value string
		extra []uintptr
	}{
		{"SetPath", vtLinkSetPath, s.target, nil},
		{"SetWorkingDirectory", vtLinkSetWorkingDirectory, s.folder, nil},
		{"SetDescription", vtLinkSetDescription, s.description, nil},
		// The icon is the first one in the program (the icon given to glutter-windows).
		{"SetIconLocation", vtLinkSetIconLocation, s.target, []uintptr{0}},
	} {
		p, err := windows.UTF16PtrFromString(step.value)
		if err != nil {
			return step.name, err
		}
		args := append([]uintptr{uintptr(unsafe.Pointer(p))}, step.extra...)
		if err := link.call(step.index, args...); err != nil {
			return step.name, err
		}
	}

	if s.appID != "" {
		if err := setAppUserModelID(link, s.appID); err != nil {
			return "SetAppUserModelID", err
		}
	}

	file, err := link.queryInterface(&iidPersistFile)
	if err != nil {
		return "QueryInterface IPersistFile", err
	}
	defer file.release()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "Save", err
	}
	const remember = 1 // fRemember: the file becomes the current one of the object
	if err := file.call(vtFileSave, uintptr(unsafe.Pointer(p)), remember); err != nil {
		return "Save", err
	}
	return "", nil
}

// setAppUserModelID sets System.AppUserModel.ID of the link through its
// IPropertyStore.
func setAppUserModelID(link *object, id string) error {
	store, err := link.queryInterface(&iidPropertyStore)
	if err != nil {
		return err
	}
	defer store.release()
	str, err := windows.UTF16PtrFromString(id)
	if err != nil {
		return err
	}
	value := propVariant{vt: vtLPWSTR, str: str}
	// SetValue copies the string, so value need not outlive the call.
	if err := store.call(vtStoreSetValue, uintptr(unsafe.Pointer(&pkeyAppUserModel)), uintptr(unsafe.Pointer(&value))); err != nil {
		return err
	}
	return store.call(vtStoreCommit)
}

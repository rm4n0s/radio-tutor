//go:build windows

package main

import (
	"debug/pe"
	"io"
	"os"
	"syscall"
	"unsafe"
)

// subsystemWindowsGUI is IMAGE_SUBSYSTEM_WINDOWS_GUI, the subsystem of a program
// linked with -H=windowsgui.
const subsystemWindowsGUI = 2

// isGUIProgram reports whether this process is the GUI build of the program, read
// from the subsystem field in the header of its own .exe file (a program cannot be
// told apart by its console: a GUI program started from a terminal has none). When
// the header cannot be read the program is taken for the console build, so that the
// commands keep working.
func isGUIProgram() bool {
	path, err := os.Executable()
	if err != nil {
		return false
	}
	f, err := pe.Open(path)
	if err != nil {
		return false
	}
	subsystem, ok := subsystemOf(f)
	cerr := f.Close()
	return ok && cerr == nil && subsystem == subsystemWindowsGUI
}

// subsystemOf returns the subsystem of a PE file, for both the 32-bit and the
// 64-bit header.
func subsystemOf(f *pe.File) (uint16, bool) {
	switch h := f.OptionalHeader.(type) {
	case *pe.OptionalHeader64:
		return h.Subsystem, true
	case *pe.OptionalHeader32:
		return h.Subsystem, true
	}
	return 0, false
}

// guiRefusalDeps returns the real means of telling the user that the GUI program
// takes no command: its standard error, the console of the parent process, and a
// message box.
func guiRefusalDeps() guiRefusal {
	return guiRefusal{stderr: os.Stderr, console: attachParentConsole, box: showMessageBox}
}

// attachParentConsole attaches this process to the console of its parent (ATTACH_PARENT_PROCESS)
// and opens its output (CONOUT$). It fails when the parent has no console.
func attachParentConsole() (io.WriteCloser, error) {
	const attachParentProcess = ^uintptr(0) // (DWORD)-1
	r, _, callErr := syscall.NewLazyDLL("kernel32.dll").NewProc("AttachConsole").Call(attachParentProcess)
	// ERROR_ACCESS_DENIED means that the process has a console already: use it.
	if r == 0 && callErr != syscall.Errno(5) {
		return nil, callErr
	}
	return os.OpenFile("CONOUT$", os.O_WRONLY, 0)
}

// showMessageBox shows text in a message box titled "Radio Tutor" (MB_OK | MB_ICONINFORMATION).
func showMessageBox(text string) {
	t, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	title, err := syscall.UTF16PtrFromString("Radio Tutor")
	if err != nil {
		return
	}
	const mbOKInformation = 0x40
	// The call blocks until the box is closed; the results are of no use.
	_, _, _ = syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(title)), mbOKInformation)
}

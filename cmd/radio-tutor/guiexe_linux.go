//go:build !windows

package main

// isGUIProgram reports whether this process is the GUI build of the program (a
// "windows subsystem" .exe). Only Windows has such a build; Linux has one program.
func isGUIProgram() bool { return false }

// guiRefusalDeps returns the means of telling the user that the GUI program takes
// no command; unused on Linux.
func guiRefusalDeps() guiRefusal { return guiRefusal{} }

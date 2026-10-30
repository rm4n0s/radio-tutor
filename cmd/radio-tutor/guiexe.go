package main

import "io"

// guiCommandMessage is what the GUI build of Radio Tutor on Windows (radio-tutor.exe,
// a program of the "windows" subsystem) says when it is given a command: only the
// console build has working standard input and output in every terminal, so the
// commands belong to radio-tutor-cli.exe (SPEC §4, scripts/build-windows.sh).
const guiCommandMessage = "radio-tutor.exe opens the application window and takes no command.\n" +
	"For the commands (validate, pack, reset-password, setup-desktop, version) use radio-tutor-cli.exe,\n" +
	"which is next to it.\n"

// guiExit is the exit code of a GUI program given a command (a usage mistake).
const guiExit = exitUsage

// guiRefusal is how the GUI program tells the user that it takes no command: the
// ways to reach the user are tried in this order, and the first that works ends it.
type guiRefusal struct {
	// stderr is the standard error of the process; a failing write means that the
	// program has none (a GUI program started from a terminal gets no handles).
	stderr io.Writer
	// console attaches to the console of the parent process and returns its output,
	// or fails when there is none (a double click in Explorer).
	console func() (io.WriteCloser, error)
	// box shows the text in a message box.
	box func(text string)
}

// refuseGUICommand tells the user that the GUI program takes no command and returns
// the exit code. The message goes to the standard error; when the process has none
// it goes to the console of the parent; and when there is no console either, it is
// shown in a message box, so that a launcher without a terminal is not silent.
func refuseGUICommand(d guiRefusal) int {
	if d.stderr != nil {
		if _, err := io.WriteString(d.stderr, guiCommandMessage); err == nil {
			return guiExit
		}
	}
	if d.console != nil {
		if w, err := d.console(); err == nil {
			_, werr := io.WriteString(w, guiCommandMessage)
			cerr := w.Close()
			if werr == nil && cerr == nil {
				return guiExit
			}
		}
	}
	if d.box != nil {
		d.box(guiCommandMessage)
	}
	return guiExit
}

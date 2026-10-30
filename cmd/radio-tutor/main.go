// Command radio-tutor is the Radio Tutor desktop application and its command
// line tools. With no arguments it opens the application window; see run for
// the subcommands.
package main

import (
	"context"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/rm4n0s/radio-tutor/internal/gui/desktop"
)

func main() {
	// os.Interrupt is Ctrl+C on both systems; SIGTERM is the termination signal on
	// Linux and, in Go, what Windows sends when a console is closed or the user logs
	// off or shuts down (SPEC §4).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	info, ok := debug.ReadBuildInfo()
	version := versionOf(info, ok)
	startGUI := func(ctx context.Context) error { return desktop.Run(ctx, version) }
	code := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, startGUI)
	stop()
	os.Exit(code)
}

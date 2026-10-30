// Command radio-tutor is the Radio Tutor desktop application and its command
// line tools. With no arguments it opens the application window; see run for
// the subcommands.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rm4n0s/radio-tutor/internal/gui/desktop"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, desktop.Run)
	stop()
	os.Exit(code)
}

package app

import (
	"context"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/glutter/ui"
)

// QuitOnDone shows its child and ends the application when the context it was
// made with is done (SIGTERM and SIGINT cancel the context of the command), as
// if the window had been closed: Run returns, so the deferred cleanups run
// (Env.Close stops the audio and the phone server, flushes the files, releases
// the lock).
//
// Glutter: RunApp has no way to end the run from outside the widget tree, and
// it does not look at signals, so the app ignored SIGTERM. This widget reaches
// the binding from inside the tree and posts ExitApplication to the UI thread.
// Glutter should provide RunAppOpts.Context (or return when the process gets
// SIGTERM or SIGINT).
type QuitOnDone struct {
	m.StatefulWidget
	ctx   context.Context
	child m.Widget
}

// NewQuitOnDone returns the widget that shows child and exits the application
// when ctx is done.
func NewQuitOnDone(ctx context.Context, child m.Widget, opts ...m.WidgetOpts) *QuitOnDone {
	return &QuitOnDone{StatefulWidget: m.NewStatefulWidgetBase(opts...), ctx: ctx, child: child}
}

// CreateState creates the state.
func (q *QuitOnDone) CreateState() m.State { return &quitOnDoneState{} }

type quitOnDoneState struct {
	m.StateOf[*QuitOnDone]
}

func (s *quitOnDoneState) InitState() {
	binding := m.WidgetsBindingOf(s.Context())
	done := s.Widget().ctx.Done()
	s.Go(func(life context.Context) {
		select {
		case <-done:
			// A required exit does not ask the observers: a signal ends the app.
			binding.Post(func() { binding.ExitApplication(ui.AppExitTypeRequired) })
		case <-life.Done():
		}
	})
}

func (s *quitOnDoneState) Build(m.BuildContext) m.Widget { return s.Widget().child }

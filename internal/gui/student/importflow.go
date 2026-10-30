package student

import (
	"context"

	"github.com/rm4n0s/errors"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/archive"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/problem"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// maxProblemsShown is how many problems the dialog lists; the rest is counted.
const maxProblemsShown = 40

// outcome is what an import ends with, to be shown on the UI thread: a short
// message, or the list of problems of an invalid cassette.
type outcome struct {
	message  *i18n.Message
	problems []problem.Problem
	reload   bool
}

// startImport asks for a file and imports it on a goroutine (SPEC §6.2). The
// shelf shows a progress indicator with Cancel meanwhile.
func (s *shelfState) startImport() {
	if s.importing {
		return
	}
	scope := app.ScopeOf(s.Context())
	ctx, cancel := context.WithCancel(s.Lifetime())
	s.SetState(func() { s.importing, s.cancel = true, cancel })
	bc := s.Context()
	s.Go(func(context.Context) {
		defer cancel()
		out := s.runImport(ctx, bc, scope)
		s.SetState(func() {
			s.importing, s.cancel = false, nil
			s.finishImport(out)
		})
	})
}

// runImport is the body of the import goroutine: it never touches a widget.
func (s *shelfState) runImport(ctx context.Context, bc m.BuildContext, scope *app.Scope) outcome {
	env := scope.Env()
	path, ok, err := env.ChooseFile(ctx, bc, app.FileQuestion{
		Title:        scope.T("student.import.title"),
		AcceptLabel:  scope.T("student.import.accept"),
		FilterName:   scope.T("student.import.filter"),
		Patterns:     []string{"*.cassette", "*.CASSETTE"},
		AllFilesName: scope.T("student.import.all_files"),
	})
	if err != nil {
		return importFailure(ctx, env, scope.Session().Role == profile.RoleAdmin, err)
	}
	if !ok {
		return outcome{}
	}
	res, err := env.Archive.Import(ctx, path, func(old, upgrade cassette.Version) bool {
		return s.confirmUpgrade(ctx, old, upgrade)
	})
	if err != nil {
		return importFailure(ctx, env, scope.Session().Role == profile.RoleAdmin, err)
	}
	switch res.Status {
	case archive.ImportInvalid:
		return outcome{problems: res.Problems}
	case archive.ImportInstalled:
		return outcome{message: msg("student.import.added", "title", titleOf(ctx, env, res.ID)), reload: true}
	case archive.ImportUpgraded:
		return outcome{message: msg("student.import.updated", "title", titleOf(ctx, env, res.ID), "version", res.Version.String()), reload: true}
	case archive.ImportAlreadyInstalled:
		return outcome{message: msg("student.import.already")}
	case archive.ImportNewerInstalled:
		return outcome{message: msg("student.import.newer", "version", res.Previous.String())}
	case archive.ImportDeclined:
		return outcome{message: msg("student.import.declined")}
	}
	return outcome{}
}

func msg(key string, args ...string) *i18n.Message {
	mm := i18n.M(key, args...)
	return &mm
}

// titleOf is the title of the installed cassette, or its id.
func titleOf(ctx context.Context, env *app.Env, id string) string {
	if _, mf, err := env.Archive.Get(ctx, id); err == nil {
		return mf.Title
	}
	return id
}

// failure turns an error of the import into a message. A cancel is silent, a
// missing file dialog has its own text, and anything else is "ask a parent"
// for a child and the details for an admin.
func importFailure(ctx context.Context, env *app.Env, admin bool, err error) outcome {
	if ctx.Err() != nil {
		return outcome{}
	}
	env.Log.Error("importing a cassette failed", "err", err)
	if app.FileChooserUnavailable(err) {
		return outcome{message: msg("student.import.no_dialog")}
	}
	if admin {
		if appErr, ok := errors.FromError(err); ok {
			return outcome{message: msg("student.import.failed_detail", "message", appErr.Message, "tag", appErr.Tag)}
		}
		return outcome{message: msg("student.import.failed_detail", "message", err.Error(), "tag", "Unknown")}
	}
	return outcome{message: msg("student.import.failed")}
}

// confirmUpgrade asks the user whether to replace the installed version; it is
// the confirmUpgrade function of archive.Store.Import, so it runs on the import
// goroutine and blocks it until the user answers (or the import is cancelled).
// The dialog is opened on the UI thread.
func (s *shelfState) confirmUpgrade(ctx context.Context, old, upgrade cassette.Version) bool {
	answer := make(chan bool, 1)
	s.SetState(func() {
		scope := app.ScopeOf(s.Context())
		ch := m.ShowDialog[bool](s.Context(), func(bc m.BuildContext) m.Widget {
			return upgradeDialog(scope, old, upgrade)
		})
		// Only a channel is read here; the dialog's result goes to the import.
		go func() {
			select {
			case v := <-ch:
				answer <- v
			case <-ctx.Done():
				answer <- false
			}
		}()
	})
	select {
	case v := <-answer:
		return v
	case <-ctx.Done():
		return false
	}
}

// upgradeDialog names both versions and asks (SPEC §6.2).
func upgradeDialog(scope *app.Scope, old, upgrade cassette.Version) m.Widget {
	return m.NewAlertDialog(m.AlertDialogOpts{
		Title:   m.NewText(scope.T("student.import.upgrade_title")),
		Content: m.NewText(scope.T("student.import.upgrade_body", "old", old.String(), "new", upgrade.String())),
		Actions: []m.Widget{
			dialogButton(KeyDialogNo, scope.T("student.import.upgrade_no"), false, false),
			dialogButton(KeyDialogYes, scope.T("student.import.upgrade_yes"), true, true),
		},
	})
}

// dialogButton is a button of a dialog that closes it with the answer.
func dialogButton(key, label string, answer, filled bool) m.Widget {
	return m.NewBuilder(func(bc m.BuildContext) m.Widget {
		press := func() { m.NavigatorPop(bc, answer) }
		if filled {
			return m.NewFilledButton(press, m.NewText(label), m.FilledButtonOpts{Key: m.NewValueKey(key)})
		}
		return m.NewTextButton(press, m.NewText(label), m.TextButtonOpts{Key: m.NewValueKey(key)})
	})
}

// finishImport shows the outcome; it runs on the UI thread.
func (s *shelfState) finishImport(out outcome) {
	scope := app.ScopeOf(s.Context())
	switch {
	case out.message != nil:
		showMessage(s.Context(), scope.Catalog().Msg(*out.message))
	case len(out.problems) > 0:
		m.ShowDialog[bool](s.Context(), func(m.BuildContext) m.Widget { return problemsDialog(scope, out.problems) })
	}
	if out.reload {
		s.reload()
	}
}

// problemsDialog lists the problems of an invalid cassette, one "path: message"
// per line (SPEC §6.2).
func problemsDialog(scope *app.Scope, problems []problem.Problem) m.Widget {
	lines := []m.Widget{m.NewText(scope.T("student.import.invalid_intro")), gap(app.Gap)}
	for i, p := range problems {
		if i == maxProblemsShown {
			lines = append(lines, m.NewText(scope.T("student.import.more", "n", itoa(len(problems)-maxProblemsShown))))
			break
		}
		lines = append(lines, m.NewText(p.String()), gap(6))
	}
	return m.NewAlertDialog(m.AlertDialogOpts{
		Title:      m.NewText(scope.T("student.import.invalid_title")),
		Content:    m.NewColumn(lines, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart, MainAxisSize: m.MainAxisSizeMin}),
		Scrollable: true,
		Actions:    []m.Widget{dialogButton(KeyDialogOK, scope.T("student.import.ok"), true, true)},
	})
}

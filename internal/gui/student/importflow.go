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

// ImportHost is what the import flow needs from the state of the screen that
// offers it: every State of a screen has these methods (Glutter's StateOf).
type ImportHost interface {
	// Context is the build context of the screen.
	Context() m.BuildContext
	// Lifetime is done when the screen is disposed.
	Lifetime() context.Context
	// Go runs fn on a goroutine tied to the screen's lifetime.
	Go(fn func(ctx context.Context))
	// SetState changes the screen's state on the UI thread and rebuilds it.
	SetState(fn func())
}

// Importer is the import of a cassette (SPEC §6.2) for a screen: the Import
// button that turns into a progress indicator with Cancel, the file dialog, the
// import on a goroutine, the upgrade confirmation and the messages. The shelf
// of a student and the cassette list of the admin both use it. Everything
// except the goroutine runs on the UI thread.
type Importer struct {
	host      ImportHost
	installed func()
	importing bool
	cancel    context.CancelFunc
}

// NewImporter returns the import flow of the screen whose state is host.
// installed is called on the UI thread after a cassette was installed or
// upgraded, so the screen can read its list again (it may be nil).
func NewImporter(host ImportHost, installed func()) *Importer {
	return &Importer{host: host, installed: installed}
}

// Importing reports whether an import is running.
func (i *Importer) Importing() bool { return i.importing }

// Cancel stops the running import and closes the file dialog.
func (i *Importer) Cancel() {
	if i.cancel != nil {
		i.cancel()
	}
}

// Start asks for a file and imports it on a goroutine (SPEC §6.2). The screen
// shows Area meanwhile.
func (i *Importer) Start() {
	if i.importing {
		return
	}
	scope := app.ScopeOf(i.host.Context())
	ctx, cancel := context.WithCancel(i.host.Lifetime())
	i.host.SetState(func() { i.importing, i.cancel = true, cancel })
	bc := i.host.Context()
	i.host.Go(func(context.Context) {
		defer cancel()
		out := i.run(ctx, bc, scope)
		i.host.SetState(func() {
			i.importing, i.cancel = false, nil
			i.finish(out)
		})
	})
}

// run is the body of the import goroutine: it never touches a widget.
func (i *Importer) run(ctx context.Context, bc m.BuildContext, scope *app.Scope) outcome {
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
		return i.confirmUpgrade(ctx, old, upgrade)
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
func (i *Importer) confirmUpgrade(ctx context.Context, old, upgrade cassette.Version) bool {
	answer := make(chan bool, 1)
	i.host.SetState(func() {
		scope := app.ScopeOf(i.host.Context())
		ch := m.ShowDialog[bool](i.host.Context(), func(bc m.BuildContext) m.Widget {
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

// finish shows the outcome; it runs on the UI thread.
func (i *Importer) finish(out outcome) {
	scope := app.ScopeOf(i.host.Context())
	switch {
	case out.message != nil:
		showMessage(i.host.Context(), scope.Catalog().Msg(*out.message))
	case len(out.problems) > 0:
		m.ShowDialog[bool](i.host.Context(), func(m.BuildContext) m.Widget { return problemsDialog(scope, out.problems) })
	}
	if out.reload && i.installed != nil {
		i.installed()
	}
}

// Area is the Import button, or the progress indicator with Cancel while an
// import runs.
func (i *Importer) Area(bc m.BuildContext, scope *app.Scope) m.Widget {
	if !i.importing {
		return m.NewAlign(m.NewFilledButtonIcon(i.Start, m.NewText(scope.T("student.import")), m.FilledButtonIconOpts{
			Icon: m.NewIcon(m.Icons.FileOpen()),
			Key:  m.NewValueKey(KeyImport),
		}), m.AlignOpts{Alignment: m.AlignmentCenterLeft})
	}
	return m.NewRow([]m.Widget{
		m.NewSizedBox(m.NewCircularProgressIndicator(), m.SizedBoxOpts{Width: m.Some(36.0), Height: m.Some(36.0)}),
		hgap(app.Gap),
		m.NewExpanded(m.NewText(scope.T("student.import.working"), m.TextOpts{Style: m.ThemeOf(bc).TextTheme.BodyLarge})),
		hgap(app.Gap),
		m.NewOutlinedButton(i.Cancel, m.NewText(scope.T("student.import.cancel")), m.OutlinedButtonOpts{Key: m.NewValueKey(KeyImportCancel)}),
	}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter})
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

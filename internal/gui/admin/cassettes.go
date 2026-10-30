package admin

import (
	"context"
	"sync/atomic"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/archive"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/student"
)

// Cassettes is the screen of the installed cassettes (SPEC §6.2): the list with
// versions, the broken ones (which can only be removed), the import (the flow of
// the student package) and Remove with the option to delete the progress and
// photos.
type Cassettes struct{ m.StatefulWidget }

// NewCassettes returns the cassettes screen.
func NewCassettes(opts ...m.WidgetOpts) *Cassettes {
	return &Cassettes{StatefulWidget: m.NewStatefulWidgetBase(opts...)}
}

// CreateState creates the state.
func (w *Cassettes) CreateState() m.State { return &cassettesState{} }

type cassettesState struct {
	m.StateOf[*Cassettes]
	started    bool
	generation atomic.Int64
	list       []archive.Installed
	loaded     bool
	loadFailed bool
	importer   *student.Importer
}

// InitState creates the import flow.
func (s *cassettesState) InitState() { s.importer = student.NewImporter(s, s.reload) }

// DidChangeDependencies starts the first load (the Scope cannot be read in
// InitState).
func (s *cassettesState) DidChangeDependencies() {
	if s.started {
		return
	}
	s.started = true
	s.reload()
}

// reload reads the list on a goroutine.
func (s *cassettesState) reload() {
	env := app.ScopeOf(s.Context()).Env()
	s.Go(func(ctx context.Context) {
		gen := s.generation.Add(1)
		list, err := env.Archive.List(ctx)
		if err != nil && ctx.Err() == nil {
			env.Log.Error("listing the cassettes failed", "err", err)
		}
		s.SetState(func() {
			if s.generation.Load() == gen {
				s.list, s.loaded, s.loadFailed = list, true, err != nil
			}
		})
	})
}

func (s *cassettesState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	children := []m.Widget{s.importer.Area(bc, scope), gap(app.Gap)}
	switch {
	case !s.loaded:
		children = append(children, m.NewCenter(m.NewCircularProgressIndicator()))
	case s.loadFailed:
		children = append(children, errorText(bc, scope.T("admin.cassettes.load_failed")))
	case len(s.list) == 0:
		children = append(children, body(bc, scope.T("admin.cassettes.empty")))
	default:
		for _, in := range s.list {
			children = append(children, s.row(bc, scope, in), gap(12))
		}
	}
	return m.NewScaffold(m.ScaffoldOpts{
		AppBar: appBar(scope, bc, scope.T("admin.cassettes")),
		Body:   pageBody(pageWidth, children),
	})
}

// row is the card of one installed cassette.
func (s *cassettesState) row(bc m.BuildContext, scope *app.Scope, in archive.Installed) m.Widget {
	icon, title := m.Icons.MenuBook(), in.Manifest.Title
	lines := []m.Widget{bold(bc, title), gap(2), muted(bc, in.ID)}
	var trailing []m.Widget
	if in.Broken {
		icon, title = m.Icons.Warning(), in.ID
		lines = []m.Widget{
			bold(bc, title), gap(2),
			errorLine(bc, scope.T("admin.cassettes.broken")),
		}
		if len(in.Problems) > 0 {
			lines = append(lines, gap(2), muted(bc, in.Problems[0].String()))
		}
	} else {
		lines = append(lines, gap(2), muted(bc, in.Manifest.Subject))
		trailing = append(trailing, badge(scope.T("admin.cassettes.version", "version", in.Manifest.Version.String()), m.Icons.Sell()), hgap(8))
	}
	trailing = append(trailing, iconAction(KeyRemovePrefix+in.ID, m.Icons.Delete(), scope.T("admin.cassettes.remove"), func() { s.remove(scope, in.ID, title) }))
	return m.NewCard(m.NewPadding(m.NewEdgeInsetsAll(16), m.NewRow([]m.Widget{
		m.NewIcon(icon, m.IconOpts{Size: m.Some(32.0)}),
		hgap(app.Gap),
		m.NewExpanded(m.NewColumn(lines, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart, Key: m.NewValueKey(KeyCassettePrefix + in.ID)})),
		hgap(8),
		m.NewRow(trailing, m.RowOpts{MainAxisSize: m.MainAxisSizeMin}),
	}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter})), m.CardOpts{Margin: m.NewEdgeInsetsAll(0)})
}

// errorLine is a line of text in the error color (without the error key).
func errorLine(bc m.BuildContext, text string) m.Widget {
	theme := m.ThemeOf(bc)
	return m.NewText(text, m.TextOpts{Style: theme.TextTheme.BodyMedium.CopyWith(m.TextStyleCopy{Color: m.Some(theme.ColorScheme.Error())})})
}

// removeAnswer is what the Remove dialog answers.
type removeAnswer struct {
	// confirmed is true when the admin chose Remove.
	confirmed bool
	// deleteData is the checkbox: delete the progress and photos too.
	deleteData bool
}

// remove asks for confirmation, with the unchecked option to delete the
// progress and photos of every student, then removes the cassette on a
// goroutine (SPEC §6.2).
func (s *cassettesState) remove(scope *app.Scope, id, title string) {
	env := scope.Env()
	build := func(m.BuildContext) m.Widget { return newRemoveDialog(title) }
	showDialog(s, build, func(a removeAnswer) {
		if !a.confirmed {
			return
		}
		s.Go(func(ctx context.Context) {
			err := env.Archive.Remove(ctx, id)
			if err == nil && a.deleteData {
				err = removeCassetteData(ctx, env, id)
			}
			if err != nil && ctx.Err() == nil {
				env.Log.Error("removing a cassette failed", "id", id, "err", err)
			}
			s.SetState(func() {
				switch {
				case err == nil:
					showMessage(s.Context(), scope.T("admin.cassettes.removed", "title", title))
				case ctx.Err() == nil:
					showMessage(s.Context(), scope.Catalog().Msg(errMessage(err)))
				}
				s.reload()
			})
		})
	})
}

// removeDialog is the confirmation of Remove with the checkbox that also
// deletes the progress and the photos; it is unchecked at first.
type removeDialog struct {
	m.StatefulWidget
	title string
}

func newRemoveDialog(title string) *removeDialog {
	return &removeDialog{StatefulWidget: m.NewStatefulWidgetBase(), title: title}
}

// CreateState creates the state.
func (w *removeDialog) CreateState() m.State { return &removeDialogState{} }

type removeDialogState struct {
	m.StateOf[*removeDialog]
	deleteData bool
}

func (s *removeDialogState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	return m.NewAlertDialog(m.AlertDialogOpts{
		Title: m.NewText(scope.T("admin.cassettes.remove_title", "title", s.Widget().title)),
		Content: m.NewSizedBox(m.NewColumn([]m.Widget{
			body(bc, scope.T("admin.cassettes.remove_body")),
			gap(app.Gap),
			m.NewCheckboxListTile(m.Some(s.deleteData), func(v m.Opt[bool]) {
				s.SetState(func() { s.deleteData = v.OrElse(false) })
			}, m.CheckboxListTileOpts{
				Key:             m.NewValueKey(KeyDeleteData),
				Title:           m.NewText(scope.T("admin.cassettes.delete_data")),
				ControlAffinity: m.ListTileControlAffinityLeading,
			}),
		}, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStretch, MainAxisSize: m.MainAxisSizeMin}), m.SizedBoxOpts{Width: m.Some(460.0)}),
		Scrollable: true,
		Actions: []m.Widget{
			dialogButton(KeyDialogCancel, scope.T("admin.cancel"), removeAnswer{}, false),
			m.NewBuilder(func(bc m.BuildContext) m.Widget {
				return m.NewFilledButton(func() { m.NavigatorPop(bc, removeAnswer{confirmed: true, deleteData: s.deleteData}) },
					m.NewText(scope.T("admin.cassettes.remove")), m.FilledButtonOpts{Key: m.NewValueKey(KeyDialogOK)})
			}),
		},
	})
}

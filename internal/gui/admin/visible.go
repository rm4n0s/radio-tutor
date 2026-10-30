package admin

import (
	"context"
	"slices"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/archive"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// visibleDialog lists the installed cassettes with a checkbox each: a checked
// cassette is on the student's shelf, an unchecked one is hidden from them
// (SPEC §6.3). Every change is saved at once in settings.json.
type visibleDialog struct {
	m.StatefulWidget
	student profile.Profile
}

func newVisibleDialog(p profile.Profile) *visibleDialog {
	return &visibleDialog{StatefulWidget: m.NewStatefulWidgetBase(), student: p}
}

// CreateState creates the state.
func (w *visibleDialog) CreateState() m.State { return &visibleState{} }

type visibleState struct {
	m.StateOf[*visibleDialog]
	started   bool
	loaded    bool
	installed []archive.Installed
	hidden    map[string]bool
}

// DidChangeDependencies starts the list of cassettes (the Scope cannot be read
// in InitState).
func (s *visibleState) DidChangeDependencies() {
	if s.started {
		return
	}
	s.started = true
	env, username := app.ScopeOf(s.Context()).Env(), s.Widget().student.Username
	s.hidden = map[string]bool{}
	for _, id := range env.Settings.Get().HiddenCassettes[username] {
		s.hidden[id] = true
	}
	s.Go(func(ctx context.Context) {
		list, err := env.Archive.List(ctx)
		if err != nil && ctx.Err() == nil {
			env.Log.Error("listing the cassettes failed", "err", err)
		}
		s.SetState(func() { s.installed, s.loaded = usable(list), true })
	})
}

// usable leaves out the broken cassettes, which no student sees.
func usable(list []archive.Installed) []archive.Installed {
	var out []archive.Installed
	for _, in := range list {
		if !in.Broken {
			out = append(out, in)
		}
	}
	return out
}

// toggle hides or shows the cassette and saves the setting in the background.
func (s *visibleState) toggle(id string, visible bool) {
	env, username := app.ScopeOf(s.Context()).Env(), s.Widget().student.Username
	s.SetState(func() {
		if visible {
			delete(s.hidden, id)
		} else {
			s.hidden[id] = true
		}
	})
	s.Go(func(ctx context.Context) {
		err := env.Settings.Update(ctx, func(st *settings.Settings) {
			ids := slices.DeleteFunc(st.HiddenCassettes[username], func(x string) bool { return x == id })
			if !visible {
				ids = append(ids, id)
			}
			if len(ids) == 0 {
				delete(st.HiddenCassettes, username)
				return
			}
			st.HiddenCassettes[username] = ids
		})
		if err == nil || ctx.Err() != nil {
			return
		}
		env.Log.Error("saving the hidden cassettes failed", "username", username, "err", err)
		msg := errMessage(err)
		s.SetState(func() {
			// The file did not change: show it as it is.
			if visible {
				s.hidden[id] = true
			} else {
				delete(s.hidden, id)
			}
			showMessage(s.Context(), app.ScopeOf(s.Context()).Catalog().Msg(msg))
		})
	})
}

func (s *visibleState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	student := s.Widget().student
	var content m.Widget
	switch {
	case !s.loaded:
		content = m.NewCenter(m.NewCircularProgressIndicator())
	case len(s.installed) == 0:
		content = body(bc, scope.T("admin.visible.none"))
	default:
		var rows []m.Widget
		for _, in := range s.installed {
			id := in.ID
			rows = append(rows, m.NewCheckboxListTile(m.Some(!s.hidden[id]), func(v m.Opt[bool]) {
				s.toggle(id, v.OrElse(false))
			}, m.CheckboxListTileOpts{
				Key:             m.NewValueKey(KeyShowPrefix + id),
				Title:           m.NewText(in.Manifest.Title),
				Subtitle:        m.NewText(id),
				ControlAffinity: m.ListTileControlAffinityLeading,
			}))
		}
		content = m.NewColumn(rows, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStretch, MainAxisSize: m.MainAxisSizeMin})
	}
	return m.NewAlertDialog(m.AlertDialogOpts{
		Title: m.NewText(scope.T("admin.visible.title", "name", student.DisplayName)),
		Content: m.NewSizedBox(m.NewColumn([]m.Widget{
			body(bc, scope.T("admin.visible.intro")), gap(app.Gap), content,
		}, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStretch, MainAxisSize: m.MainAxisSizeMin}), m.SizedBoxOpts{Width: m.Some(460.0)}),
		Scrollable: true,
		Actions:    []m.Widget{dialogButton(KeyDialogOK, scope.T("admin.done"), true, true)},
	})
}

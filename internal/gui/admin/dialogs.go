package admin

import (
	"context"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// dialogHost is what showDialog needs from the state of a screen: every State
// has these methods.
type dialogHost interface {
	Context() m.BuildContext
	Go(fn func(ctx context.Context))
	SetState(fn func())
}

// showDialog opens the dialog and calls then, on the UI thread inside SetState,
// with its answer when it closes. A dialog that is dismissed gives the zero
// value.
func showDialog[T any](host dialogHost, build m.WidgetBuilder, then func(T)) {
	ch := m.ShowDialog[T](host.Context(), build)
	host.Go(func(ctx context.Context) {
		select {
		case v := <-ch:
			host.SetState(func() { then(v) })
		case <-ctx.Done():
		}
	})
}

// fieldSpec describes one text field of a form dialog.
type fieldSpec struct {
	key, label, helper, initial string
	secret                      bool
}

// formValues are what the user typed in a form dialog.
type formValues struct {
	texts []string
	role  profile.Role
}

// formSpec describes a form dialog: its texts, its fields and what it does.
type formSpec struct {
	title, intro, submitLabel string
	fields                    []fieldSpec
	// matchLast says that the last two fields (a password and its repeat) must
	// be the same.
	matchLast bool
	// roles adds the choice between a student and a parent account.
	roles bool
	// submit does the work on a goroutine. A returned error is shown in the
	// dialog and the dialog stays open; success closes it with true.
	submit func(ctx context.Context, v formValues) error
}

// formDialog is a dialog with text fields, an optional role choice, and Cancel
// and submit buttons.
type formDialog struct {
	m.StatefulWidget
	spec formSpec
}

func newFormDialog(spec formSpec) *formDialog {
	return &formDialog{StatefulWidget: m.NewStatefulWidgetBase(), spec: spec}
}

// CreateState creates the state.
func (w *formDialog) CreateState() m.State { return &formDialogState{} }

type formDialogState struct {
	m.StateOf[*formDialog]
	controllers []*m.TextEditingController
	role        profile.Role
	busy        bool
	err         *i18n.Message
}

func (s *formDialogState) InitState() {
	s.role = profile.RoleStudent
	for _, f := range s.Widget().spec.fields {
		c := m.NewTextEditingController()
		c.SetText(f.initial)
		s.controllers = append(s.controllers, c)
	}
}

func (s *formDialogState) Dispose() {
	for _, c := range s.controllers {
		c.Dispose()
	}
}

// submit runs the work on a goroutine (hashing a password takes a third of a
// second) and closes the dialog when it worked.
func (s *formDialogState) submit() {
	if s.busy {
		return
	}
	spec := s.Widget().spec
	v := formValues{role: s.role}
	for _, c := range s.controllers {
		v.texts = append(v.texts, c.Text())
	}
	if n := len(v.texts); spec.matchLast && n >= 2 && v.texts[n-1] != v.texts[n-2] {
		s.SetState(func() { s.err = &i18n.Message{Key: "err.passwords_differ"} })
		return
	}
	log := app.ScopeOf(s.Context()).Env().Log
	s.SetState(func() { s.busy, s.err = true, nil })
	s.Go(func(ctx context.Context) {
		err := spec.submit(ctx, v)
		s.SetState(func() {
			s.busy = false
			if err != nil {
				log.Error("an admin action failed", "action", spec.title, "err", err)
				msg := errMessage(err)
				s.err = &msg
				return
			}
			m.NavigatorPop(s.Context(), true)
		})
	})
}

func (s *formDialogState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	spec := s.Widget().spec
	children := []m.Widget{}
	if spec.intro != "" {
		children = append(children, body(bc, spec.intro), gap(app.Gap))
	}
	for i, f := range spec.fields {
		var onSubmitted func(string)
		if i == len(spec.fields)-1 {
			onSubmitted = func(string) { s.submit() }
		}
		children = append(children, textField(f.key, s.controllers[i], f.label, f.helper, f.secret, onSubmitted), gap(app.Gap))
	}
	if spec.roles {
		children = append(children, s.roleChoice(scope), gap(app.Gap))
	}
	if s.err != nil {
		children = append(children, errorText(bc, scope.Catalog().Msg(*s.err)), gap(app.Gap))
	}
	var submit m.Widget = m.NewFilledButton(s.submit, m.NewText(spec.submitLabel), m.FilledButtonOpts{Key: m.NewValueKey(KeyDialogOK)})
	if s.busy {
		submit = m.NewSizedBox(m.NewCircularProgressIndicator(), m.SizedBoxOpts{Width: m.Some(app.ButtonHeight), Height: m.Some(app.ButtonHeight)})
	}
	return m.NewAlertDialog(m.AlertDialogOpts{
		Title:      m.NewText(spec.title),
		Content:    m.NewSizedBox(m.NewColumn(children, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStretch, MainAxisSize: m.MainAxisSizeMin}), m.SizedBoxOpts{Width: m.Some(460.0)}),
		Scrollable: true,
		Actions: []m.Widget{
			dialogButton(KeyDialogCancel, scope.T("admin.cancel"), false, false),
			submit,
		},
	})
}

// roleChoice is the choice between a student and a parent account.
func (s *formDialogState) roleChoice(scope *app.Scope) m.Widget {
	chip := func(key string, role profile.Role, text string) m.Widget {
		return m.NewChoiceChip(m.NewText(text), s.role == role, m.ChoiceChipOpts{
			Key: m.NewValueKey(key),
			OnSelected: func(bool) {
				s.SetState(func() { s.role = role })
			},
		})
	}
	return m.NewWrap([]m.Widget{
		m.NewText(scope.T("admin.students.role")),
		chip(KeyRoleStudent, profile.RoleStudent, scope.T("admin.role.student")),
		chip(KeyRoleAdmin, profile.RoleAdmin, scope.T("admin.role.admin")),
	}, m.WrapOpts{Spacing: 12, RunSpacing: 8, CrossAxisAlignment: m.WrapCrossAlignmentCenter})
}

// confirmDialog asks a yes/no question and answers true for the confirming
// button, which has KeyDialogOK.
func confirmDialog(scope *app.Scope, title, text, confirm string) m.Widget {
	return m.NewAlertDialog(m.AlertDialogOpts{
		Title:   m.NewText(title),
		Content: m.NewText(text),
		Actions: []m.Widget{
			dialogButton(KeyDialogCancel, scope.T("admin.cancel"), false, false),
			dialogButton(KeyDialogOK, confirm, true, true),
		},
	})
}

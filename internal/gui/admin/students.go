package admin

import (
	"context"

	"github.com/rm4n0s/errors"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// Students is the screen of the profiles (SPEC §12.2): the list with role and
// creation date, Add, Rename, Reset password, Delete, and the cassettes each
// student can see.
type Students struct{ m.StatefulWidget }

// NewStudents returns the students screen.
func NewStudents(opts ...m.WidgetOpts) *Students {
	return &Students{StatefulWidget: m.NewStatefulWidgetBase(opts...)}
}

// CreateState creates the state.
func (w *Students) CreateState() m.State { return &studentsState{} }

type studentsState struct{ m.StateOf[*Students] }

// refresh shows the profiles again.
func (s *studentsState) refresh() { s.SetState(func() {}) }

func (s *studentsState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	children := []m.Widget{
		m.NewAlign(m.NewFilledButtonIcon(func() { s.add(scope) }, m.NewText(scope.T("admin.students.add")), m.FilledButtonIconOpts{
			Icon: m.NewIcon(m.Icons.Add()),
			Key:  m.NewValueKey(KeyAddStudent),
		}), m.AlignOpts{Alignment: m.AlignmentCenterLeft}),
		gap(app.Gap),
	}
	for _, p := range scope.Env().Profiles.List() {
		children = append(children, s.row(bc, scope, p), gap(12))
	}
	return m.NewScaffold(m.ScaffoldOpts{
		AppBar: appBar(scope, bc, scope.T("admin.students")),
		Body:   pageBody(pageWidth, children),
	})
}

// row is the card of one profile.
func (s *studentsState) row(bc m.BuildContext, scope *app.Scope, p profile.Profile) m.Widget {
	roleBadge := badge(scope.T("admin.role."+string(p.Role)), roleIcon(p.Role))
	actions := []m.Widget{
		iconAction(KeyRenamePrefix+p.Username, m.Icons.Edit(), scope.T("admin.students.rename"), func() { s.rename(scope, p) }),
		iconAction(KeyResetPrefix+p.Username, m.Icons.Key(), scope.T("admin.students.reset"), func() { s.resetPassword(scope, p) }),
	}
	if p.Role == profile.RoleStudent {
		actions = append(actions, iconAction(KeyVisiblePrefix+p.Username, m.Icons.Visibility(), scope.T("admin.students.cassettes"), func() { s.cassettes(p) }))
	}
	deleteTip := scope.T("admin.students.delete")
	deleteAction := func() { s.delete(scope, p) }
	if p.Username == scope.Session().Username {
		deleteTip, deleteAction = scope.T("admin.students.delete_self"), nil
	}
	actions = append(actions, m.NewIconButton(deleteAction, m.NewIcon(m.Icons.Delete()), m.IconButtonOpts{
		Key: m.NewValueKey(KeyDeletePrefix + p.Username), Tooltip: deleteTip,
	}))
	return m.NewCard(m.NewPadding(m.NewEdgeInsetsAll(16), m.NewRow([]m.Widget{
		m.NewExpanded(m.NewColumn([]m.Widget{
			m.NewText(p.DisplayName, m.TextOpts{
				Key:   m.NewValueKey(KeyStudentPrefix + p.Username),
				Style: m.ThemeOf(bc).TextTheme.TitleLarge.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600}),
			}),
			gap(2),
			muted(bc, scope.T("admin.students.created", "username", p.Username, "date", p.CreatedAt.Format("2006-01-02"))),
		}, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart})),
		hgap(8),
		roleBadge,
		hgap(8),
		m.NewRow(actions, m.RowOpts{MainAxisSize: m.MainAxisSizeMin}),
	}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter})), m.CardOpts{Margin: m.NewEdgeInsetsAll(0)})
}

// roleIcon is the icon of the role's badge.
func roleIcon(r profile.Role) m.IconData {
	if r == profile.RoleAdmin {
		return m.Icons.AdminPanelSettings()
	}
	return m.Icons.Person()
}

// add opens the Add dialog (SPEC §3.1): username, name, role and the password
// twice.
func (s *studentsState) add(scope *app.Scope) {
	env := scope.Env()
	spec := formSpec{
		title:       scope.T("admin.students.add_title"),
		intro:       scope.T("admin.students.add_intro"),
		submitLabel: scope.T("admin.students.add_submit"),
		roles:       true,
		matchLast:   true,
		fields: []fieldSpec{
			{key: app.KeyUsername, label: scope.T("field.username"), helper: scope.T("field.username_hint")},
			{key: app.KeyDisplayName, label: scope.T("field.display_name")},
			{key: app.KeyPassword, label: scope.T("field.password"), helper: scope.T("admin.students.password_rule"), secret: true},
			{key: app.KeyPasswordAgain, label: scope.T("field.password_again"), secret: true},
		},
		submit: func(ctx context.Context, v formValues) error {
			_, err := env.Profiles.Create(ctx, v.texts[0], v.texts[1], v.role, v.texts[2])
			return err
		},
	}
	showDialog(s, func(m.BuildContext) m.Widget { return newFormDialog(spec) }, func(bool) { s.refresh() })
}

// rename opens the Rename dialog; only the name shown changes.
func (s *studentsState) rename(scope *app.Scope, p profile.Profile) {
	env := scope.Env()
	spec := formSpec{
		title:       scope.T("admin.students.rename_title", "name", p.DisplayName),
		submitLabel: scope.T("admin.students.rename"),
		fields:      []fieldSpec{{key: app.KeyDisplayName, label: scope.T("field.display_name"), initial: p.DisplayName}},
		submit: func(ctx context.Context, v formValues) error {
			return env.Profiles.Rename(ctx, p.Username, v.texts[0])
		},
	}
	showDialog(s, func(m.BuildContext) m.Widget { return newFormDialog(spec) }, func(bool) { s.refresh() })
}

// resetPassword opens the Reset password dialog.
func (s *studentsState) resetPassword(scope *app.Scope, p profile.Profile) {
	env := scope.Env()
	spec := formSpec{
		title:       scope.T("admin.students.reset_title", "name", p.DisplayName),
		submitLabel: scope.T("admin.students.reset_submit"),
		matchLast:   true,
		fields: []fieldSpec{
			{key: app.KeyPassword, label: scope.T("admin.students.new_password"), helper: scope.T("field.password_min_hint", "min", itoa(p.Role.MinPasswordLength())), secret: true},
			{key: app.KeyPasswordAgain, label: scope.T("field.password_again"), secret: true},
		},
		submit: func(ctx context.Context, v formValues) error {
			return env.Profiles.SetPassword(ctx, p.Username, v.texts[0])
		},
	}
	showDialog(s, func(m.BuildContext) m.Widget { return newFormDialog(spec) }, func(done bool) {
		if done {
			showMessage(s.Context(), scope.T("admin.students.reset_done", "name", p.DisplayName))
		}
	})
}

// delete asks for confirmation naming the student, then deletes the profile,
// the progress and the photos on a goroutine (SPEC §3.1).
func (s *studentsState) delete(scope *app.Scope, p profile.Profile) {
	env := scope.Env()
	build := func(m.BuildContext) m.Widget {
		return confirmDialog(scope, scope.T("admin.students.delete_title", "name", p.DisplayName),
			scope.T("admin.students.delete_body", "name", p.DisplayName), scope.T("admin.students.delete"))
	}
	showDialog(s, build, func(yes bool) {
		if !yes {
			return
		}
		s.Go(func(ctx context.Context) {
			err := deleteStudent(ctx, env, p.Username)
			if err != nil && ctx.Err() == nil {
				env.Log.Error("deleting a profile failed", "username", p.Username, "err", err)
			}
			s.SetState(func() {
				switch {
				case err == nil:
					showMessage(s.Context(), scope.T("admin.students.deleted", "name", p.DisplayName))
				case ctx.Err() != nil:
				default:
					showMessage(s.Context(), scope.Catalog().Msg(deleteMessage(err)))
				}
			})
		})
	})
}

// deleteMessage is the message for a failed delete: the last admin has its own
// text.
func deleteMessage(err error) i18n.Message {
	if appErr, ok := errors.FromError(err); ok && appErr.Tag == "LastAdmin" {
		return i18n.M("admin.students.last_admin")
	}
	return errMessage(err)
}

// cassettes opens the dialog with the cassettes the student can see.
func (s *studentsState) cassettes(p profile.Profile) {
	showDialog(s, func(m.BuildContext) m.Widget { return newVisibleDialog(p) }, func(bool) {})
}

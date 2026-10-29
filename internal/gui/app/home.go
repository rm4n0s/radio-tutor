package app

import (
	m "github.com/rm4n0s/glutter/material"
)

// logoutButton is the Log out button of the app bars.
func logoutButton(scope *Scope, bc m.BuildContext) m.Widget {
	return m.NewPadding(m.NewEdgeInsetsSymmetric(0, 16), m.NewFilledButtonTonalIcon(
		func() { scope.LogOut(bc) },
		m.NewText(scope.T("home.logout")),
		m.FilledButtonIconOpts{Icon: m.NewIcon(m.Icons.Logout()), Key: m.NewValueKey(KeyLogout), Style: smallFilledButtonStyle()},
	))
}

// AdminHome is the placeholder home of an admin; step 6.2 fills it.
type AdminHome struct{ m.StatelessWidget }

// NewAdminHome returns the admin home.
func NewAdminHome(opts ...m.WidgetOpts) *AdminHome {
	return &AdminHome{StatelessWidget: m.NewStatelessWidgetBase(opts...)}
}

// Build builds the admin home: the notices from startup, a greeting and a
// placeholder.
func (w *AdminHome) Build(bc m.BuildContext) m.Widget {
	scope := ScopeOf(bc)
	return m.NewScaffold(m.ScaffoldOpts{
		AppBar: m.NewAppBar(m.AppBarOpts{
			Title:   m.NewText(scope.T("home.admin.title")),
			Actions: []m.Widget{logoutButton(scope, bc)},
		}),
		Body: centered([]m.Widget{
			NewNotices(),
			headline(bc, scope.T("home.admin.hello", "name", scope.Session().DisplayName)),
			gap(Gap),
			body(bc, scope.T("home.admin.placeholder")),
		}),
	})
}

// StudentHome is the placeholder home of a student; step 6.3 fills it with the
// shelf.
type StudentHome struct{ m.StatelessWidget }

// NewStudentHome returns the student home.
func NewStudentHome(opts ...m.WidgetOpts) *StudentHome {
	return &StudentHome{StatelessWidget: m.NewStatelessWidgetBase(opts...)}
}

// Build builds the student home: a greeting and a placeholder for the shelf.
func (w *StudentHome) Build(bc m.BuildContext) m.Widget {
	scope := ScopeOf(bc)
	return m.NewScaffold(m.ScaffoldOpts{
		AppBar: m.NewAppBar(m.AppBarOpts{
			Title:   m.NewText(scope.T("home.student.title")),
			Actions: []m.Widget{logoutButton(scope, bc)},
		}),
		Body: centered([]m.Widget{
			headline(bc, scope.T("home.student.hello", "name", scope.Session().DisplayName)),
			gap(Gap),
			body(bc, scope.T("home.student.shelf")),
		}),
	})
}

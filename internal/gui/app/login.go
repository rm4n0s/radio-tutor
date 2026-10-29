package app

import (
	"context"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
)

// Login is the login screen (SPEC §3.2): a big button per profile with its
// display name; tapping one asks for its password.
type Login struct{ m.StatefulWidget }

// NewLogin returns the login screen.
func NewLogin(opts ...m.WidgetOpts) *Login {
	return &Login{StatefulWidget: m.NewStatefulWidgetBase(opts...)}
}

// CreateState creates the state.
func (w *Login) CreateState() m.State { return &loginState{} }

type loginState struct {
	m.StateOf[*Login]
	password *m.TextEditingController
	selected *profile.Profile
	busy     bool
	err      *i18n.Message
}

func (s *loginState) InitState() { s.password = m.NewTextEditingController() }

func (s *loginState) Dispose() { s.password.Dispose() }

// choose shows the password step for p.
func (s *loginState) choose(p profile.Profile) {
	s.password.SetText("")
	s.SetState(func() { s.selected, s.err = &p, nil })
}

// back returns to the list of names.
func (s *loginState) back() {
	if s.busy {
		return
	}
	s.password.SetText("")
	s.SetState(func() { s.selected, s.err = nil, nil })
}

// submit checks the password on a goroutine (the hash takes a third of a second).
func (s *loginState) submit() {
	if s.busy || s.selected == nil {
		return
	}
	scope := ScopeOf(s.Context())
	p, password := *s.selected, s.password.Text()
	s.SetState(func() { s.busy, s.err = true, nil })
	store, log := scope.Env().Profiles, scope.Env().Log
	s.Go(func(context.Context) {
		got, err := store.Login(p.Username, password)
		if err != nil {
			msg := errorMessage(err, p.Role == profile.RoleAdmin)
			if msg.Key != "login.wrong" && msg.Key != "login.throttled" {
				log.Error("login failed", "username", p.Username, "err", err)
			}
			s.SetState(func() {
				s.busy, s.err = false, &msg
				s.password.SetText("")
			})
			return
		}
		scope.LogIn(got)
	})
}

func (s *loginState) Build(bc m.BuildContext) m.Widget {
	scope := ScopeOf(bc)
	children := []m.Widget{
		NewLanguageSwitch(),
		gap(Gap),
	}
	if s.selected == nil {
		children = append(children, s.nameList(bc, scope)...)
	} else {
		children = append(children, s.passwordStep(bc, scope)...)
	}
	return m.NewScaffold(m.ScaffoldOpts{Body: centered(children)})
}

// nameList is the headline and a button for each profile.
func (s *loginState) nameList(bc m.BuildContext, scope *Scope) []m.Widget {
	var buttons []m.Widget
	for _, p := range scope.Env().Profiles.List() {
		buttons = append(buttons, m.NewFilledButtonTonalIcon(func() { s.choose(p) }, m.NewText(p.DisplayName), m.FilledButtonIconOpts{
			Icon: m.NewIcon(m.Icons.Person()),
			Key:  m.NewValueKey(KeyProfilePrefix + p.Username),
		}))
	}
	return []m.Widget{
		headline(bc, scope.T("login.title")),
		gap(Gap * 2),
		m.NewWrap(buttons, m.WrapOpts{Alignment: m.WrapAlignmentCenter, Spacing: Gap, RunSpacing: Gap}),
	}
}

// passwordStep greets the chosen profile and asks for the password.
func (s *loginState) passwordStep(bc m.BuildContext, scope *Scope) []m.Widget {
	children := []m.Widget{
		headline(bc, scope.T("login.password", "name", s.selected.DisplayName)),
		gap(Gap * 2),
		textField(KeyPassword, s.password, scope.T("field.password"), "", true, func(string) { s.submit() }),
		gap(Gap),
	}
	if s.err != nil {
		children = append(children, errorText(bc, scope.Catalog().Msg(*s.err)), gap(Gap))
	}
	if s.busy {
		return append(children, busyIndicator())
	}
	return append(children, m.NewRow([]m.Widget{
		m.NewOutlinedButton(s.back, m.NewText(scope.T("login.back")), m.OutlinedButtonOpts{Key: m.NewValueKey(KeyBack)}),
		hgap(Gap),
		m.NewFilledButton(s.submit, m.NewText(scope.T("login.enter")), m.FilledButtonOpts{Key: m.NewValueKey(KeySubmit)}),
	}, m.RowOpts{MainAxisAlignment: m.MainAxisAlignmentCenter}))
}

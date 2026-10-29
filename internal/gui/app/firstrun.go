package app

import (
	"context"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
)

// FirstRun is the "Create the parent account" screen, the only screen while no
// admin exists (SPEC §3.1). It makes the first admin and logs them in.
type FirstRun struct{ m.StatefulWidget }

// NewFirstRun returns the first-run screen.
func NewFirstRun(opts ...m.WidgetOpts) *FirstRun {
	return &FirstRun{StatefulWidget: m.NewStatefulWidgetBase(opts...)}
}

// CreateState creates the state.
func (w *FirstRun) CreateState() m.State { return &firstRunState{} }

type firstRunState struct {
	m.StateOf[*FirstRun]
	username, display, password, again *m.TextEditingController
	busy                               bool
	err                                *i18n.Message
}

func (s *firstRunState) InitState() {
	s.username = m.NewTextEditingController()
	s.display = m.NewTextEditingController()
	s.password = m.NewTextEditingController()
	s.again = m.NewTextEditingController()
}

func (s *firstRunState) Dispose() {
	s.username.Dispose()
	s.display.Dispose()
	s.password.Dispose()
	s.again.Dispose()
}

// submit creates the admin on a goroutine (hashing the password takes a third of
// a second) and logs them in.
func (s *firstRunState) submit() {
	if s.busy {
		return
	}
	scope := ScopeOf(s.Context())
	username, display, password := s.username.Text(), s.display.Text(), s.password.Text()
	if password != s.again.Text() {
		s.SetState(func() { s.err = &i18n.Message{Key: "err.passwords_differ"} })
		return
	}
	s.SetState(func() { s.busy, s.err = true, nil })
	store, log := scope.Env().Profiles, scope.Env().Log
	s.Go(func(ctx context.Context) {
		p, err := store.Create(ctx, username, display, profile.RoleAdmin, password)
		if err != nil {
			msg := errorMessage(err, true)
			log.Error("creating the first admin failed", "err", err)
			s.SetState(func() { s.busy, s.err = false, &msg })
			return
		}
		scope.LogIn(p)
	})
}

func (s *firstRunState) Build(bc m.BuildContext) m.Widget {
	scope := ScopeOf(bc)
	submit := func(string) { s.submit() }
	min := profile.RoleAdmin.MinPasswordLength()
	children := []m.Widget{
		NewLanguageSwitch(),
		gap(Gap),
		headline(bc, scope.T("firstrun.title")),
		gap(Gap),
		body(bc, scope.T("firstrun.intro")),
		gap(Gap * 1.5),
		m.NewRow([]m.Widget{
			m.NewExpanded(textField(KeyUsername, s.username, scope.T("field.username"), scope.T("field.username_hint"), false, nil)),
			hgap(Gap),
			m.NewExpanded(textField(KeyDisplayName, s.display, scope.T("field.display_name"), "", false, nil)),
		}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart}),
		gap(Gap),
		m.NewRow([]m.Widget{
			m.NewExpanded(textField(KeyPassword, s.password, scope.T("field.password"), scope.T("field.password_min_hint", "min", itoa(min)), true, nil)),
			hgap(Gap),
			m.NewExpanded(textField(KeyPasswordAgain, s.again, scope.T("field.password_again"), "", true, submit)),
		}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart}),
		gap(Gap),
	}
	if s.err != nil {
		children = append(children, errorText(bc, scope.Catalog().Msg(*s.err)), gap(Gap))
	}
	if s.busy {
		children = append(children, busyIndicator())
	} else {
		children = append(children, m.NewFilledButton(s.submit, m.NewText(scope.T("firstrun.create")), m.FilledButtonOpts{Key: m.NewValueKey(KeySubmit)}))
	}
	children = append(children, gap(Gap))
	children = append(children, NewNotices())
	return m.NewScaffold(m.ScaffoldOpts{Body: centered(children)})
}

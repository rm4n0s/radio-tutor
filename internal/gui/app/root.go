// Package app is the application shell of Radio Tutor: startup, the root
// widget, the theme, the navigation, the translations in use, the first-run and
// login screens and the placeholder homes.
package app

import (
	"context"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
)

// Title is the title of the application window.
const Title = "Radio Tutor"

// The smallest size of the application window, in logical pixels.
const (
	MinWindowWidth  = 800
	MinWindowHeight = 600
)

// Root is the root widget of the application: the Scope with the Material app
// and its navigator inside. It holds the language and the logged-in user.
type Root struct {
	m.StatefulWidget
	env *Env
}

// NewRoot returns the root widget of the application for the environment.
func NewRoot(env *Env, opts ...m.WidgetOpts) *Root {
	return &Root{StatefulWidget: m.NewStatefulWidgetBase(opts...), env: env}
}

// Env is the environment the root was made with.
func (r *Root) Env() *Env { return r.env }

// CreateState creates the state.
func (r *Root) CreateState() m.State { return &rootState{} }

type rootState struct {
	m.StateOf[*Root]
	theme   *m.ThemeData
	lang    i18n.Lang
	session *profile.Profile
}

func (s *rootState) InitState() {
	s.Widget().env.binding.Store(m.WidgetsBindingOf(s.Context()))
	s.theme = NewTheme()
	s.lang = s.Widget().env.Settings.Get().Language
}

// setLanguage shows the language at once and saves it in the background.
func (s *rootState) setLanguage(l i18n.Lang) {
	if !l.Valid() {
		return
	}
	s.SetState(func() { s.lang = l })
	env := s.Widget().env
	s.Go(func(ctx context.Context) {
		if err := env.Settings.SetLanguage(ctx, l); err != nil {
			env.Log.Error("saving the language failed", "language", string(l), "err", err)
		}
	})
}

// logIn may run on any goroutine, so it reads the widget inside SetState.
func (s *rootState) logIn(p profile.Profile) {
	s.SetState(func() {
		s.Widget().env.Log.Info("login", "username", p.Username, "role", string(p.Role))
		s.session = &p
	})
}

func (s *rootState) logOut() {
	if s.session != nil {
		env := s.Widget().env
		env.Log.Info("logout", "username", s.session.Username)
		// The student's phone link ends with the login (SPEC §10.1). Ending it
		// waits for a submission that is being opened, so it is not done on the
		// UI thread.
		if env.Submission != nil {
			username := s.session.Username
			s.Go(func(context.Context) { env.Submission.EndLink(username) })
		}
	}
	s.SetState(func() { s.session = nil })
}

func (s *rootState) Build(m.BuildContext) m.Widget {
	cat := i18n.For(s.lang)
	app := m.NewMaterialApp(m.MaterialAppOpts{
		Title:                      Title,
		Theme:                      s.theme,
		DebugShowCheckedModeBanner: m.Some(false),
		Home:                       NewHome(),
	})
	return newScope(s.Widget().env, cat, s.session, actions{
		setLanguage: s.setLanguage,
		logIn:       s.logIn,
		logOut:      s.logOut,
	}, app)
}

// Home is the first route: it shows the first-run screen when there is no
// admin, the login screen when nobody is logged in, and the home of the
// logged-in user (Env.Screens.Admin for an admin and Env.Screens.Student for a
// student, the placeholder when there is none).
type Home struct{ m.StatelessWidget }

// NewHome returns the home route's widget.
func NewHome(opts ...m.WidgetOpts) *Home {
	return &Home{StatelessWidget: m.NewStatelessWidgetBase(opts...)}
}

// Build picks the screen.
func (h *Home) Build(bc m.BuildContext) m.Widget {
	scope := ScopeOf(bc)
	switch session := scope.Session(); {
	case session != nil && session.Role == profile.RoleAdmin && scope.Env().Screens.Admin != nil:
		return scope.Env().Screens.Admin()
	case session != nil && session.Role == profile.RoleAdmin:
		return NewAdminHome()
	case session != nil && scope.Env().Screens.Student != nil:
		return scope.Env().Screens.Student(session.Username)
	case session != nil:
		return NewStudentHome()
	case scope.Env().Profiles.NeedsFirstAdmin():
		return NewFirstRun()
	default:
		return NewLogin()
	}
}

package app

import (
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
)

// actions are what the root offers the screens below it.
type actions struct {
	setLanguage func(i18n.Lang)
	logIn       func(profile.Profile)
	logOut      func()
}

// Scope gives every screen the shared environment, the translations and the
// session of the user who is logged in. It sits above the Material app, so the
// routes pushed on the navigator see it too. Screens read it with ScopeOf and
// rebuild when the language or the user changes.
type Scope struct {
	m.InheritedWidget
	env     *Env
	cat     i18n.Catalog
	session *profile.Profile
	act     actions
}

// newScope creates the scope around child.
func newScope(env *Env, cat i18n.Catalog, session *profile.Profile, act actions, child m.Widget) *Scope {
	return &Scope{InheritedWidget: m.NewInheritedWidgetBase(child), env: env, cat: cat, session: session, act: act}
}

// UpdateShouldNotify is whether the language or the logged-in user changed.
func (s *Scope) UpdateShouldNotify(old m.Widget) bool {
	o := old.(*Scope)
	return o.cat.Lang() != s.cat.Lang() || o.session != s.session
}

// ScopeOf returns the scope above the context. It panics when there is none:
// every screen of the application is built below the root.
func ScopeOf(bc m.BuildContext) *Scope {
	scope, ok := m.DependOn[*Scope](bc)
	if !ok {
		panic("a Radio Tutor screen needs the app Scope above it")
	}
	return scope
}

// Env is the shared environment (stores, folders, logger).
func (s *Scope) Env() *Env { return s.env }

// Catalog translates message keys into the language of the UI.
func (s *Scope) Catalog() i18n.Catalog { return s.cat }

// T is Catalog().T.
func (s *Scope) T(key string, args ...string) string { return s.cat.T(key, args...) }

// Session is the profile of the user who is logged in, or nil.
func (s *Scope) Session() *profile.Profile { return s.session }

// SetLanguage switches the language of the UI at once and saves it in the
// settings in the background.
func (s *Scope) SetLanguage(l i18n.Lang) { s.act.setLanguage(l) }

// LogIn makes p the logged-in user. It may be called from any goroutine.
func (s *Scope) LogIn(p profile.Profile) { s.act.logIn(p) }

// LogOut closes every screen above the first one and logs the user out, which
// shows the login screen again.
func (s *Scope) LogOut(bc m.BuildContext) {
	m.NavigatorPopUntil(bc, func(r m.Route) bool { return r.IsFirst() })
	s.act.logOut()
}

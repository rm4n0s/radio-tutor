package app

import (
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
)

// AlreadyRunningSize is the size of the already-running window.
var AlreadyRunningSize = m.Size{Width: 520, Height: 420}

// KeyAlreadyRunningOK is the key of the OK button of the already-running window.
const KeyAlreadyRunningOK = "already-running-ok"

// AlreadyRunning is the root of the small window that tells the user that
// another Radio Tutor holds the lock (SPEC §4). Its OK button ends the app.
type AlreadyRunning struct {
	m.StatelessWidget
	lang i18n.Lang
}

// NewAlreadyRunning returns the window's root, in the language.
func NewAlreadyRunning(lang i18n.Lang, opts ...m.WidgetOpts) *AlreadyRunning {
	return &AlreadyRunning{StatelessWidget: m.NewStatelessWidgetBase(opts...), lang: lang}
}

// Build builds a Material app with the message and the OK button.
func (w *AlreadyRunning) Build(m.BuildContext) m.Widget {
	cat := i18n.For(w.lang)
	return m.NewMaterialApp(m.MaterialAppOpts{
		Title:                      Title,
		Theme:                      NewTheme(),
		DebugShowCheckedModeBanner: m.Some(false),
		Home:                       &alreadyRunningPage{StatelessWidget: m.NewStatelessWidgetBase(), cat: cat},
	})
}

type alreadyRunningPage struct {
	m.StatelessWidget
	cat i18n.Catalog
}

func (p *alreadyRunningPage) Build(bc m.BuildContext) m.Widget {
	exit := func() { m.WidgetsBindingOf(bc).PlatformDispatcher().RequestAppExit() }
	return m.NewScaffold(m.ScaffoldOpts{Body: centered([]m.Widget{
		m.NewIcon(m.Icons.Info(), m.IconOpts{Size: m.Some(64.0)}),
		gap(Gap),
		headline(bc, p.cat.T("already_running.title")),
		gap(Gap),
		m.NewText(p.cat.T("already_running.body"), m.TextOpts{Style: m.ThemeOf(bc).TextTheme.BodyLarge, TextAlign: m.TextAlignCenter}),
		gap(Gap * 2),
		m.NewFilledButton(exit, m.NewText(p.cat.T("already_running.ok")), m.FilledButtonOpts{Key: m.NewValueKey(KeyAlreadyRunningOK)}),
	})})
}

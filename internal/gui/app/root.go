// Package app is the application shell of Radio Tutor: the root widget, the
// navigation, the theme, login and first run. It grows with the later steps.
package app

import (
	m "github.com/rm4n0s/glutter/material"
)

// Title is the title of the application window and the text of the root screen.
const Title = "Radio Tutor"

// The smallest size of the application window, in logical pixels.
const (
	MinWindowWidth  = 800
	MinWindowHeight = 600
)

// Root is the root widget of the application: a Material app whose home shows
// the name of the application, centered.
type Root struct{ m.StatelessWidget }

// NewRoot returns the root widget of the application.
func NewRoot(opts ...m.WidgetOpts) *Root {
	return &Root{StatelessWidget: m.NewStatelessWidgetBase(opts...)}
}

// Build builds the Material app with the home screen.
func (r *Root) Build(bc m.BuildContext) m.Widget {
	return m.NewMaterialApp(m.MaterialAppOpts{
		Title:                      Title,
		Theme:                      m.NewThemeData(m.ThemeDataOpts{ColorSchemeSeed: m.Some(m.Colors.Blue.Primary)}),
		DebugShowCheckedModeBanner: m.Some(false),
		Home: m.NewScaffold(m.ScaffoldOpts{
			Body: m.NewCenter(m.NewText(Title, m.TextOpts{Style: m.ThemeOf(bc).TextTheme.HeadlineMedium})),
		}),
	})
}

// Run opens the application window and blocks until it is closed.
func Run() error {
	minSize := m.Size{Width: MinWindowWidth, Height: MinWindowHeight}
	return m.RunApp(NewRoot(), m.RunAppOpts{
		AppID: "dev.radiotutor.app",
		Window: m.WindowConfig{
			Title:   Title,
			Size:    minSize,
			MinSize: minSize,
		},
	})
}

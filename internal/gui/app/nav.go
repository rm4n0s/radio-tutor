package app

import (
	m "github.com/rm4n0s/glutter/material"
)

// Navigation. The screens of the application form a stack of routes on
// Glutter's Navigator: the first route is the Home, which shows the first-run,
// login or home screen by itself; everything else (a cassette, a lesson, a
// worksheet, an admin page) is pushed on top of it and popped to go back. Log
// out pops everything back to the Home.

// Push opens the screen built by builder on top of the current one. The
// channel gets the result the screen passes to Pop.
func Push(bc m.BuildContext, builder m.WidgetBuilder) <-chan any {
	return m.NavigatorPush(bc, m.NewMaterialPageRoute(builder))
}

// Pop closes the current screen, passing the optional result to the channel
// that Push returned.
func Pop(bc m.BuildContext, result ...any) { m.NavigatorPop(bc, result...) }

// CanPop reports whether there is a screen to go back to.
func CanPop(bc m.BuildContext) bool { return m.NavigatorCanPop(bc) }

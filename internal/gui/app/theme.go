package app

import (
	m "github.com/rm4n0s/glutter/material"
)

// Sizes of the touch-friendly controls, in logical pixels.
const (
	// ButtonHeight is the smallest height of a button.
	ButtonHeight = 64.0
	// ButtonMinWidth is the smallest width of a button.
	ButtonMinWidth = 120.0
	// Gap is the standard space between widgets.
	Gap = 16.0
	// PagePadding is the padding around the content of a page.
	PagePadding = 24.0
	// ContentMaxWidth is the widest a page's content grows.
	ContentMaxWidth = 640.0
)

// textScale makes the Material 3 type scale larger than the default, for
// children.
const textScale = 1.25

// sized is a text style that only sets the font size and inherits the rest
// from the theme's typography.
func sized(size float64) *m.TextStyle {
	return m.NewTextStyle(m.TextStyleOpts{FontSize: m.Some(size * textScale)})
}

// textTheme is Material 3's type scale, each size multiplied by textScale.
func textTheme() *m.TextTheme {
	return m.NewTextTheme(m.TextThemeOpts{
		DisplayLarge: sized(57), DisplayMedium: sized(45), DisplaySmall: sized(36),
		HeadlineLarge: sized(32), HeadlineMedium: sized(28), HeadlineSmall: sized(24),
		TitleLarge: sized(22), TitleMedium: sized(16), TitleSmall: sized(14),
		BodyLarge: sized(16), BodyMedium: sized(14), BodySmall: sized(12),
		LabelLarge: sized(16), LabelMedium: sized(12), LabelSmall: sized(11),
	})
}

// NewTheme returns the theme of the application: Material 3 from a sky-blue
// seed (soft, friendly colors), type 25% larger than Material's default, large
// rounded buttons and roomy text fields.
func NewTheme() *m.ThemeData {
	seed := m.Colors.Blue.Primary
	text := textTheme()
	// Glutter: a ButtonStyle whose TextStyle is set (FilledButtonStyleFrom with
	// TextStyle) draws no label at all, even with a ForegroundColor, so the
	// buttons get their larger label from the theme's LabelLarge instead.
	buttonOpts := m.ButtonStyleFromOpts{
		MinimumSize: m.Some(m.Size{Width: ButtonMinWidth, Height: ButtonHeight}),
		Padding:     m.NewEdgeInsetsSymmetric(12, 28),
		Shape:       m.NewStadiumBorder(),
	}
	return m.NewThemeData(m.ThemeDataOpts{
		ColorSchemeSeed:   m.Some(seed),
		TextTheme:         text,
		FilledButtonTheme: m.NewFilledButtonThemeData(m.FilledButtonThemeDataOpts{Style: m.FilledButtonStyleFrom(buttonOpts)}),
		OutlinedButtonTheme: m.NewOutlinedButtonThemeData(m.OutlinedButtonThemeDataOpts{
			Style: m.OutlinedButtonStyleFrom(buttonOpts),
		}),
		TextButtonTheme: m.NewTextButtonThemeData(m.TextButtonThemeDataOpts{Style: m.TextButtonStyleFrom(buttonOpts)}),
		InputDecorationTheme: m.NewInputDecorationThemeData(m.InputDecorationThemeDataOpts{
			ContentPadding: m.NewEdgeInsetsSymmetric(18, 16),
		}),
	})
}

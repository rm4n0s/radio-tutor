package app_test

import (
	"testing"

	"github.com/rm4n0s/glutter/foundation"
	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/glutter/ui"
	"github.com/rm4n0s/glutter/widgets"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// TestButtonLabelsKeepTheThemeFont checks that the large label size the theme
// gives the buttons (ButtonStyle.TextStyle) is merged onto the default label
// style: the label keeps the theme's font family and weight and the button's
// foreground color, and is really drawn. Glutter before 1.9.6 drew no label.
func TestButtonLabelsKeepTheThemeFont(t *testing.T) {
	theme := app.NewTheme()
	cs := theme.ColorScheme
	// The reference: the label style a button gets from a theme without the
	// app's button styles.
	reference := m.NewThemeData(m.ThemeDataOpts{ColorSchemeSeed: m.Some(m.Colors.Blue.Primary)})
	press := func() {}
	tests := []struct {
		name      string
		button    widgets.Widget
		finder    gluttertest.Finder
		wantColor ui.Color
	}{
		{"filled", m.NewFilledButton(press, widgets.NewText("Go")), gluttertest.ByType[*m.FilledButton](), cs.OnPrimary()},
		{"outlined", m.NewOutlinedButton(press, widgets.NewText("Go")), gluttertest.ByType[*m.OutlinedButton](), cs.Primary()},
		{"text", m.NewTextButton(press, widgets.NewText("Go")), gluttertest.ByType[*m.TextButton](), cs.Primary()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pump := func(th *m.ThemeData) *gluttertest.Tester {
				ts := gluttertest.New(t, gluttertest.Opts{Size: ui.Size{Width: 400, Height: 600}})
				ts.PumpWidget(m.NewMaterialApp(m.MaterialAppOpts{Theme: th, Home: m.NewScaffold(m.ScaffoldOpts{Body: widgets.NewCenter(tt.button)})}))
				return ts
			}
			style := func(ts *gluttertest.Tester) *m.TextStyle {
				return gluttertest.WidgetAs[*m.Material](ts, gluttertest.First(gluttertest.Descendant(tt.finder, gluttertest.ByType[*m.Material]()))).TextStyle()
			}
			want := style(pump(reference))
			ts := pump(theme)
			got := style(ts)
			if got.FontFamily == "" || got.FontFamily != want.FontFamily {
				t.Errorf("font family = %q, want the theme's %q", got.FontFamily, want.FontFamily)
			}
			if got.FontWeight != want.FontWeight {
				t.Errorf("font weight = %v, want the theme's %v", got.FontWeight, want.FontWeight)
			}
			if wantSize := 16 * 1.25; got.FontSize != foundation.Some(wantSize) {
				t.Errorf("font size = %v, want %v", got.FontSize, wantSize)
			}
			if got.Color != foundation.Some(tt.wantColor) {
				t.Errorf("label color = %v, want the foreground %v", got.Color, tt.wantColor)
			}
			if w := ts.Size(gluttertest.ByType[*widgets.RichText]()).Width; w <= 0 {
				t.Errorf("the label is %v wide: it is not drawn", w)
			}
		})
	}
}

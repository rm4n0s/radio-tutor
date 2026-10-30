package worksheet

import (
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// maxWidth is the widest the content of the screens grows.
const maxWidth = 720.0

// gap is empty vertical space.
func gap(height float64) m.Widget {
	return m.NewSizedBox(nil, m.SizedBoxOpts{Height: m.Some(height)})
}

// hgap is empty horizontal space.
func hgap(width float64) m.Widget {
	return m.NewSizedBox(nil, m.SizedBoxOpts{Width: m.Some(width)})
}

// pageBody puts the children in a column of at most width, centered in a
// scrolling page with padding.
func pageBody(width float64, children []m.Widget) m.Widget {
	return m.NewSingleChildScrollView(
		m.NewCenter(
			m.NewConstrainedBox(
				m.NewBoxConstraints(m.BoxConstraintsOpts{MaxWidth: m.Some(width)}),
				m.NewPadding(m.NewEdgeInsetsAll(app.PagePadding), m.NewColumn(children, m.ColumnOpts{
					CrossAxisAlignment: m.CrossAxisAlignmentStretch,
				})),
			),
		),
	)
}

// logoutButton is the Log out button of the app bars.
func logoutButton(scope *app.Scope, bc m.BuildContext) m.Widget {
	return m.NewPadding(m.NewEdgeInsetsSymmetric(0, 16), m.NewFilledButtonTonalIcon(
		func() { scope.LogOut(bc) },
		m.NewText(scope.T("home.logout")),
		m.FilledButtonIconOpts{
			Icon: m.NewIcon(m.Icons.Logout()),
			Key:  m.NewValueKey(app.KeyLogout),
			Style: m.FilledButtonStyleFrom(m.ButtonStyleFromOpts{
				MinimumSize: m.Some(m.Size{Width: 56, Height: 48}),
				Padding:     m.NewEdgeInsetsSymmetric(8, 12),
				Shape:       m.NewStadiumBorder(),
			}),
		},
	))
}

// showMessage shows a short message in a snack bar at the bottom of the screen.
func showMessage(bc m.BuildContext, text string) {
	m.ScaffoldMessengerOf(bc).ShowSnackBar(m.NewSnackBar(m.NewText(text)))
}

// bigButtonStyle is the style of the large buttons of the flow.
func bigButtonStyle() *m.ButtonStyle {
	return m.FilledButtonStyleFrom(m.ButtonStyleFromOpts{MinimumSize: m.Some(m.Size{Width: app.ButtonMinWidth, Height: 72})})
}

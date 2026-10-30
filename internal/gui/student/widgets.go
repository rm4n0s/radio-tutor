package student

import (
	"strconv"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/glutter/svg"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// itoa formats a small integer for a message argument or a key.
func itoa(n int) string { return strconv.Itoa(n) }

// gap is empty vertical space.
func gap(height float64) m.Widget {
	return m.NewSizedBox(nil, m.SizedBoxOpts{Height: m.Some(height)})
}

// hgap is empty horizontal space.
func hgap(width float64) m.Widget {
	return m.NewSizedBox(nil, m.SizedBoxOpts{Width: m.Some(width)})
}

// headline is a big, bold title.
func headline(bc m.BuildContext, text string) m.Widget {
	style := m.ThemeOf(bc).TextTheme.HeadlineMedium.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600})
	return m.NewText(text, m.TextOpts{Style: style})
}

// pageBody puts the children in a column of at most maxWidth, centered in a
// scrolling page with padding.
func pageBody(maxWidth float64, children []m.Widget) m.Widget {
	return m.NewSingleChildScrollView(
		m.NewCenter(
			m.NewConstrainedBox(
				m.NewBoxConstraints(m.BoxConstraintsOpts{MaxWidth: m.Some(maxWidth)}),
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

// coverPicture draws a cassette's cover filling a box of the width and height;
// a cover that is missing or that cannot be drawn is a book on a colored
// background.
func coverPicture(bc m.BuildContext, data []byte, width, height float64) m.Widget {
	scheme := m.ThemeOf(bc).ColorScheme
	fallback := func() m.Widget {
		return m.NewContainer(
			m.NewCenter(m.NewIcon(m.Icons.MenuBook(), m.IconOpts{Size: m.Some(width / 3), Color: m.Some(scheme.OnPrimaryContainer())})),
			m.ContainerOpts{Width: m.Some(width), Height: m.Some(height), Color: m.Some(scheme.PrimaryContainer())},
		)
	}
	if len(data) == 0 {
		return fallback()
	}
	return m.NewSizedBox(svg.NewSvgPictureMemory(data, svg.SvgPictureOpts{
		Width:                m.Some(width),
		Height:               m.Some(height),
		Fit:                  m.BoxFitCover,
		PlaceholderBuilder:   func(m.BuildContext) m.Widget { return fallback() },
		ErrorBuilder:         func(m.BuildContext, error) m.Widget { return fallback() },
		ExcludeFromSemantics: true,
	}), m.SizedBoxOpts{Width: m.Some(width), Height: m.Some(height)})
}

package admin

import (
	"strconv"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// pageWidth is how wide the content of a list screen grows.
const pageWidth = 760.0

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

// body is a paragraph of body text.
func body(bc m.BuildContext, text string) m.Widget {
	return m.NewText(text, m.TextOpts{Style: m.ThemeOf(bc).TextTheme.BodyLarge})
}

// muted is small text in the secondary color.
func muted(bc m.BuildContext, text string) m.Widget {
	theme := m.ThemeOf(bc)
	return m.NewText(text, m.TextOpts{Style: theme.TextTheme.BodyMedium.CopyWith(m.TextStyleCopy{Color: m.Some(theme.ColorScheme.OnSurfaceVariant())})})
}

// mutedShort is muted text that stops at two lines.
func mutedShort(bc m.BuildContext, text string) m.Widget {
	theme := m.ThemeOf(bc)
	return m.NewText(text, m.TextOpts{
		Style:    theme.TextTheme.BodyMedium.CopyWith(m.TextStyleCopy{Color: m.Some(theme.ColorScheme.OnSurfaceVariant())}),
		MaxLines: m.Some(2), Overflow: m.TextOverflowEllipsis,
	})
}

// bold is a text in the title style.
func bold(bc m.BuildContext, text string) m.Widget {
	style := m.ThemeOf(bc).TextTheme.TitleMedium.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600})
	return m.NewText(text, m.TextOpts{Style: style, MaxLines: m.Some(2), Overflow: m.TextOverflowEllipsis})
}

// errorText shows an error message in the error color; it has app.KeyError.
func errorText(bc m.BuildContext, text string) m.Widget {
	theme := m.ThemeOf(bc)
	style := theme.TextTheme.BodyLarge.CopyWith(m.TextStyleCopy{Color: m.Some(theme.ColorScheme.Error())})
	return m.NewText(text, m.TextOpts{Key: m.NewValueKey(app.KeyError), Style: style})
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

// appBar is the app bar of a screen below the home: a title and Log out (the
// back button comes from the navigator).
func appBar(scope *app.Scope, bc m.BuildContext, title string) *m.AppBar {
	return m.NewAppBar(m.AppBarOpts{
		Title:   m.NewText(title),
		Actions: []m.Widget{logoutButton(scope, bc)},
	})
}

// showMessage shows a short message in a snack bar at the bottom of the screen.
func showMessage(bc m.BuildContext, text string) {
	m.ScaffoldMessengerOf(bc).ShowSnackBar(m.NewSnackBar(m.NewText(text)))
}

// textField is a roomy outlined text field with a label and optional helper
// text; secret hides the characters.
func textField(key string, c *m.TextEditingController, label, helper string, secret bool, onSubmitted func(string)) m.Widget {
	dec := &m.InputDecoration{
		LabelText: m.Some(label),
		Border:    m.NewOutlineInputBorder(),
	}
	if helper != "" {
		dec.HelperText = m.Some(helper)
	}
	return m.NewTextField(m.TextFieldOpts{
		Key:         m.NewValueKey(key),
		Controller:  c,
		Decoration:  dec,
		ObscureText: secret,
		OnSubmitted: onSubmitted,
	})
}

// iconAction is a round icon button with a tooltip.
func iconAction(key string, icon m.IconData, tooltip string, onPressed func()) m.Widget {
	return m.NewIconButton(onPressed, m.NewIcon(icon), m.IconButtonOpts{
		Key:     m.NewValueKey(key),
		Tooltip: tooltip,
	})
}

// badge is a small rounded tag with an icon, such as "Parent" or "Cloud".
func badge(text string, icon m.IconData) m.Widget {
	return m.NewChip(m.NewText(text), m.ChipOpts{Avatar: m.NewIcon(icon, m.IconOpts{Size: m.Some(18.0)})})
}

// dialogButton is a button of a dialog that closes it with the answer.
func dialogButton(key, text string, answer any, filled bool) m.Widget {
	return m.NewBuilder(func(bc m.BuildContext) m.Widget {
		press := func() { m.NavigatorPop(bc, answer) }
		if filled {
			return m.NewFilledButton(press, m.NewText(text), m.FilledButtonOpts{Key: m.NewValueKey(key)})
		}
		return m.NewTextButton(press, m.NewText(text), m.TextButtonOpts{Key: m.NewValueKey(key)})
	})
}

// errMessage turns an error of a store or of the file system into the message
// the admin sees: the cases a parent can fix get their own text, everything else
// is the full message with its tag (SPEC §15).
func errMessage(err error) i18n.Message {
	return app.ErrorMessage(err, true)
}

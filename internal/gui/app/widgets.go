package app

import (
	"strconv"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
)

// Keys of the widgets that tests and the later steps look for.
const (
	// KeyLogout is the key of the Log out button.
	KeyLogout = "logout"
	// KeyUsername is the key of the username field of the first-run screen.
	KeyUsername = "username"
	// KeyDisplayName is the key of the display-name field of the first-run screen.
	KeyDisplayName = "display-name"
	// KeyPassword is the key of the password field (first run and login).
	KeyPassword = "password"
	// KeyPasswordAgain is the key of the repeated password field of the first-run screen.
	KeyPasswordAgain = "password-again"
	// KeySubmit is the key of the Create account and Enter buttons.
	KeySubmit = "submit"
	// KeyBack is the key of the Back button of the login screen.
	KeyBack = "back"
	// KeyProfilePrefix starts the key of a profile button of the login screen;
	// the username follows.
	KeyProfilePrefix = "profile-"
	// KeyLanguagePrefix starts the key of a language button; the language code
	// follows.
	KeyLanguagePrefix = "lang-"
	// KeyError is the key of the error message text.
	KeyError = "error"
	// KeyNoticePrefix starts the key of a notice; its index follows.
	KeyNoticePrefix = "notice-"
)

// gap is empty vertical space.
func gap(height float64) m.Widget {
	return m.NewSizedBox(nil, m.SizedBoxOpts{Height: m.Some(height)})
}

// hgap is empty horizontal space.
func hgap(width float64) m.Widget {
	return m.NewSizedBox(nil, m.SizedBoxOpts{Width: m.Some(width)})
}

// centered puts children in a column of at most ContentMaxWidth, centered in a
// scrolling page with padding.
func centered(children []m.Widget) m.Widget {
	return m.NewSingleChildScrollView(
		m.NewCenter(
			m.NewConstrainedBox(
				m.NewBoxConstraints(m.BoxConstraintsOpts{MaxWidth: m.Some(ContentMaxWidth)}),
				m.NewPadding(m.NewEdgeInsetsAll(PagePadding), m.NewColumn(children, m.ColumnOpts{
					CrossAxisAlignment: m.CrossAxisAlignmentStretch,
				})),
			),
		),
	)
}

// headline is a big, bold title.
func headline(bc m.BuildContext, text string) m.Widget {
	style := m.ThemeOf(bc).TextTheme.HeadlineMedium.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600})
	return m.NewText(text, m.TextOpts{Style: style, TextAlign: m.TextAlignCenter})
}

// body is a paragraph of body text.
func body(bc m.BuildContext, text string) m.Widget {
	return m.NewText(text, m.TextOpts{Style: m.ThemeOf(bc).TextTheme.BodyLarge})
}

// errorText shows an error message in the error color; it has KeyError.
func errorText(bc m.BuildContext, text string) m.Widget {
	theme := m.ThemeOf(bc)
	style := theme.TextTheme.BodyLarge.CopyWith(m.TextStyleCopy{Color: m.Some(theme.ColorScheme.Error())})
	return m.NewText(text, m.TextOpts{Key: m.NewValueKey(KeyError), Style: style})
}

// smallButtonStyle is the style of the buttons that are smaller than a
// touch-friendly button (the language switch).
func smallButtonStyle() *m.ButtonStyle { return m.TextButtonStyleFrom(smallButtonOpts()) }

// smallFilledButtonStyle is smallButtonStyle for a filled button.
func smallFilledButtonStyle() *m.ButtonStyle { return m.FilledButtonStyleFrom(smallButtonOpts()) }

func smallButtonOpts() m.ButtonStyleFromOpts {
	return m.ButtonStyleFromOpts{
		MinimumSize: m.Some(m.Size{Width: 56, Height: 48}),
		Padding:     m.NewEdgeInsetsSymmetric(8, 12),
		Shape:       m.NewStadiumBorder(),
	}
}

// LanguageSwitch is the EN / ΕΛ pair of buttons that changes the language of
// the UI; the current language is the filled one.
type LanguageSwitch struct{ m.StatelessWidget }

// NewLanguageSwitch returns the language switch.
func NewLanguageSwitch(opts ...m.WidgetOpts) *LanguageSwitch {
	return &LanguageSwitch{StatelessWidget: m.NewStatelessWidgetBase(opts...)}
}

// Build builds the two buttons, right-aligned.
func (w *LanguageSwitch) Build(bc m.BuildContext) m.Widget {
	scope := ScopeOf(bc)
	var buttons []m.Widget
	for _, l := range i18n.Langs() {
		label := m.NewText(l.Label())
		key := m.NewValueKey(KeyLanguagePrefix + string(l))
		if l == scope.Catalog().Lang() {
			buttons = append(buttons, m.NewFilledButtonTonal(func() {}, label, m.FilledButtonOpts{Key: key, Style: smallFilledButtonStyle()}))
			continue
		}
		buttons = append(buttons, m.NewTextButton(func() { scope.SetLanguage(l) }, label, m.TextButtonOpts{Key: key, Style: smallButtonStyle()}))
	}
	return m.NewAlign(m.NewRow([]m.Widget{
		m.NewTooltip(m.NewIcon(m.Icons.Language()), m.TooltipOpts{Message: scope.T("lang.switch")}),
		hgap(8),
		buttons[0], hgap(4), buttons[1],
	}, m.RowOpts{MainAxisSize: m.MainAxisSizeMin}), m.AlignOpts{Alignment: m.AlignmentCenterRight})
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

// busyIndicator is the spinner shown instead of a button while the password is
// hashed.
func busyIndicator() m.Widget {
	return m.NewCenter(m.NewSizedBox(m.NewCircularProgressIndicator(), m.SizedBoxOpts{Width: m.Some(ButtonHeight), Height: m.Some(ButtonHeight)}))
}

// itoa formats a small integer for a message argument.
func itoa(n int) string { return strconv.Itoa(n) }

package worksheet

import (
	"math"
	"strconv"
	"unicode/utf8"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/glutter/svg"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

const (
	badgeSize     = 64.0
	pictureHeight = 180.0
)

// banner tells the child what to do on paper (SPEC §9).
func banner(bc m.BuildContext, scope *app.Scope) m.Widget {
	scheme := m.ThemeOf(bc).ColorScheme
	style := m.ThemeOf(bc).TextTheme.TitleMedium.CopyWith(m.TextStyleCopy{Color: m.Some(scheme.OnPrimaryContainer())})
	return m.NewContainer(
		m.NewRow([]m.Widget{
			m.NewIcon(m.Icons.EditNote(), m.IconOpts{Size: m.Some(40.0), Color: m.Some(scheme.OnPrimaryContainer())}),
			hgap(app.Gap),
			m.NewExpanded(m.NewText(scope.T("worksheet.banner"), m.TextOpts{Style: style})),
		}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter}),
		m.ContainerOpts{
			Key:     m.NewValueKey(KeyBanner),
			Padding: m.NewEdgeInsetsAll(app.Gap),
			Decoration: m.NewBoxDecoration(m.BoxDecorationOpts{
				Color:        m.Some(scheme.PrimaryContainer()),
				BorderRadius: m.NewBorderRadiusCircular(16),
			}),
		},
	)
}

// badgeWidth is the width of a label badge: round for one or two characters,
// wider for a longer label (up to six).
func badgeWidth(label string) float64 {
	n := utf8.RuneCountInString(label)
	if n <= 2 {
		return badgeSize
	}
	return math.Min(24+20*float64(n), 4*badgeSize)
}

// badge is the big round label of an exercise.
func badge(bc m.BuildContext, key, label string, muted bool) m.Widget {
	theme := m.ThemeOf(bc)
	bg, fg := theme.ColorScheme.Primary(), theme.ColorScheme.OnPrimary()
	if muted {
		bg, fg = theme.ColorScheme.SurfaceContainerHighest(), theme.ColorScheme.OnSurfaceVariant()
	}
	style := theme.TextTheme.HeadlineSmall.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW700, Color: m.Some(fg)})
	return m.NewContainer(
		m.NewText(label, m.TextOpts{Style: style}),
		m.ContainerOpts{
			Key:        m.NewValueKey(key),
			Alignment:  m.AlignmentCenter,
			Width:      m.Some(badgeWidth(label)),
			Height:     m.Some(badgeSize),
			Decoration: m.NewBoxDecoration(m.BoxDecorationOpts{Color: m.Some(bg), BorderRadius: m.NewBorderRadiusCircular(badgeSize / 2)}),
		},
	)
}

// card is the card of one exercise: the checkbox, the label badge, the title,
// the prompt with its picture and speaker, the state, the attempts left and the
// hints so far (SPEC §9).
func (s *screenState) card(bc m.BuildContext, scope *app.Scope, ex exerciseView) m.Widget {
	theme := m.ThemeOf(bc)
	available := ex.State == progress.StateAvailable
	title := theme.TextTheme.TitleLarge.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600})

	details := []m.Widget{
		m.NewText(ex.Title, m.TextOpts{Style: title}),
		gap(6),
		m.NewText(ex.Prompt, m.TextOpts{Style: theme.TextTheme.BodyLarge}),
	}
	if len(ex.Picture) > 0 {
		details = append(details, gap(12), prompt(ex.Picture))
	}
	if ex.HasAudio {
		details = append(details, gap(12), s.speaker(scope, ex))
	}
	details = append(details, s.statusLines(bc, scope, ex)...)

	row := m.NewPadding(m.NewEdgeInsetsAll(app.Gap), m.NewRow([]m.Widget{
		s.checkbox(scope, ex, available),
		hgap(8),
		badge(bc, KeyBadgePrefix+ex.ID, ex.Label, !available),
		hgap(app.Gap),
		m.NewExpanded(m.NewColumn(details, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart})),
	}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart}))
	opacity := 1.0
	if ex.State == progress.StateLocked {
		opacity = 0.5
	}
	return m.NewCard(m.NewOpacity(opacity, row), m.CardOpts{Key: m.NewValueKey(KeyCardPrefix + ex.ID)})
}

// checkbox is the tick that selects an exercise for the submission; only an
// available exercise can be ticked.
func (s *screenState) checkbox(scope *app.Scope, ex exerciseView, available bool) m.Widget {
	var onChanged func(m.Opt[bool])
	if available {
		onChanged = func(v m.Opt[bool]) { s.toggle(ex.ID, v.OrElse(false)) }
	}
	box := m.NewCheckbox(m.Some(s.selected[ex.ID]), onChanged, m.CheckboxOpts{Key: m.NewValueKey(KeySelectPrefix + ex.ID)})
	return m.NewSizedBox(m.NewTooltip(box, m.TooltipOpts{Message: scope.T("worksheet.select")}), m.SizedBoxOpts{Width: m.Some(48.0), Height: m.Some(48.0)})
}

// prompt draws the picture of an exercise; a picture that cannot be drawn is
// left out.
func prompt(data []byte) m.Widget {
	return m.NewConstrainedBox(
		m.NewBoxConstraints(m.BoxConstraintsOpts{MaxHeight: m.Some(pictureHeight)}),
		svg.NewSvgPictureMemory(data, svg.SvgPictureOpts{
			Fit:                  m.BoxFitContain,
			Height:               m.Some(pictureHeight),
			PlaceholderBuilder:   func(m.BuildContext) m.Widget { return gap(pictureHeight) },
			ErrorBuilder:         func(m.BuildContext, error) m.Widget { return gap(0) },
			ExcludeFromSemantics: true,
		}),
	)
}

// speaker is the button that reads the prompt aloud (or stops it).
func (s *screenState) speaker(scope *app.Scope, ex exerciseView) m.Widget {
	playing := s.playing == ex.ID
	icon, label := m.Icons.VolumeUp(), scope.T("worksheet.listen")
	if playing {
		icon, label = m.Icons.StopCircle(), scope.T("worksheet.stop")
	}
	return m.NewAlign(m.NewFilledButtonTonalIcon(func() { s.speak(ex.ID) }, m.NewText(label), m.FilledButtonIconOpts{
		Icon: m.NewIcon(icon),
		Key:  m.NewValueKey(KeySpeakerPrefix + ex.ID),
	}), m.AlignOpts{Alignment: m.AlignmentCenterLeft})
}

// statusLines are the state of the exercise, the tries left and the hints.
func (s *screenState) statusLines(bc m.BuildContext, scope *app.Scope, ex exerciseView) []m.Widget {
	theme := m.ThemeOf(bc)
	var out []m.Widget
	line := func(icon m.IconData, color m.Color, text string) m.Widget {
		return m.NewPadding(m.NewEdgeInsetsFromLTRB(0, 10, 0, 0), m.NewRow([]m.Widget{
			m.NewIcon(icon, m.IconOpts{Size: m.Some(24.0), Color: m.Some(color)}),
			hgap(8),
			m.NewExpanded(m.NewText(text, m.TextOpts{Style: theme.TextTheme.BodyLarge})),
		}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart}))
	}
	switch ex.State {
	case progress.StatePassed:
		out = append(out, line(m.Icons.Star(), theme.ColorScheme.Primary(), scope.T("student.state.passed")))
	case progress.StateNeedsReview:
		out = append(out, line(m.Icons.HourglassEmpty(), theme.ColorScheme.Tertiary(), scope.T("worksheet.needs_review")))
	case progress.StateLocked:
		out = append(out, line(m.Icons.Lock(), theme.ColorScheme.OnSurfaceVariant(), scope.T("student.state.locked")))
	case progress.StateAvailable:
		if ex.Limited {
			out = append(out, line(m.Icons.Replay(), theme.ColorScheme.OnSurfaceVariant(), scope.T("worksheet.tries_left", "n", strconv.Itoa(ex.AttemptsLeft))))
		}
	}
	for i, h := range ex.Hints {
		out = append(out, m.NewKeyedSubtree(
			line(m.Icons.Lightbulb(), theme.ColorScheme.Tertiary(), scope.T("worksheet.hint", "n", strconv.Itoa(i+1), "text", h)),
			m.WidgetOpts{Key: m.NewValueKey(KeyHintPrefix + ex.ID + "-" + strconv.Itoa(i+1))}))
	}
	return out
}

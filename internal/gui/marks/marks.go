// Package marks is how a grading verdict looks (SPEC §10.1): a check for
// correct, ~ for partial, a cross for incorrect and ? for unreadable and not
// found, in a colored circle. The results the child sees (package worksheet)
// and the submissions the parent reviews (package admin) show the same marks,
// so the icon, the color and the i18n key of the words live here.
package marks

import (
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
)

// Look is how a verdict is shown.
type Look struct {
	// Icon is the icon of the mark; unused when Text is set.
	Icon m.IconData
	// Text is shown instead of the icon (for ~ and ?).
	Text string
	// Color is the color of the circle.
	Color m.Color
	// Words is the i18n key of the words for the child ("flow.verdict.*").
	Words string
}

// LookOf returns how the verdict is shown. A verdict nobody knows looks like
// unreadable.
func LookOf(scheme *m.ColorScheme, v progress.Verdict) Look {
	switch v {
	case progress.VerdictCorrect:
		return Look{Icon: m.Icons.Check(), Color: m.NewColor(0xFF2E7D32), Words: "flow.verdict.correct"}
	case progress.VerdictPartial:
		return Look{Text: "~", Color: m.NewColor(0xFFE65100), Words: "flow.verdict.partial"}
	case progress.VerdictIncorrect:
		return Look{Icon: m.Icons.Close(), Color: scheme.Error(), Words: "flow.verdict.incorrect"}
	case progress.VerdictNotFound:
		return Look{Text: "?", Color: scheme.Outline(), Words: "flow.verdict.not_found"}
	}
	return Look{Text: "?", Color: scheme.Outline(), Words: "flow.verdict.unreadable"}
}

// Size is the size of a mark.
type Size struct {
	// Box is the width and height of the circle.
	Box float64
	// Icon is the size of the icon inside it.
	Icon float64
	// Text is the style of the ~ or ? inside it (the color is made white and
	// the weight bold).
	Text *m.TextStyle
}

// Mark is the round mark of a look: white on the look's color.
func Mark(key string, lk Look, size Size) m.Widget {
	white := m.NewColor(0xFFFFFFFF)
	var inner m.Widget
	if lk.Text != "" {
		inner = m.NewText(lk.Text, m.TextOpts{Style: size.Text.CopyWith(m.TextStyleCopy{Color: m.Some(white), FontWeight: m.FontWeightW700})})
	} else {
		inner = m.NewIcon(lk.Icon, m.IconOpts{Size: m.Some(size.Icon), Color: m.Some(white)})
	}
	return m.NewContainer(inner, m.ContainerOpts{
		Key:        m.NewValueKey(key),
		Width:      m.Some(size.Box),
		Height:     m.Some(size.Box),
		Alignment:  m.AlignmentCenter,
		Decoration: m.NewBoxDecoration(m.BoxDecorationOpts{Color: m.Some(lk.Color), Shape: m.BoxShapeCircle}),
	})
}

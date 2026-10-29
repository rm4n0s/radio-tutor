package app

import (
	m "github.com/rm4n0s/glutter/material"
)

// Notices shows the messages for the admin that startup produced (Env.Notices),
// for example that a damaged file was set aside (SPEC §4). Each one has an OK
// button that hides it until the next start.
type Notices struct{ m.StatefulWidget }

// NewNotices returns the notice list; it is empty when there is nothing to say.
func NewNotices(opts ...m.WidgetOpts) *Notices {
	return &Notices{StatefulWidget: m.NewStatefulWidgetBase(opts...)}
}

// CreateState creates the state.
func (w *Notices) CreateState() m.State { return &noticesState{dismissed: map[int]bool{}} }

type noticesState struct {
	m.StateOf[*Notices]
	dismissed map[int]bool
}

func (s *noticesState) Build(bc m.BuildContext) m.Widget {
	scope := ScopeOf(bc)
	var children []m.Widget
	for i, notice := range scope.Env().Notices {
		if s.dismissed[i] {
			continue
		}
		card := m.NewCard(m.NewPadding(m.NewEdgeInsetsAll(Gap), m.NewColumn([]m.Widget{
			m.NewRow([]m.Widget{
				m.NewIcon(m.Icons.Warning()),
				hgap(Gap),
				m.NewExpanded(m.NewText(scope.Catalog().Msg(notice), m.TextOpts{
					Key:   m.NewValueKey(KeyNoticePrefix + itoa(i)),
					Style: m.ThemeOf(bc).TextTheme.BodyMedium,
				})),
			}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart}),
			m.NewAlign(m.NewTextButton(func() { s.SetState(func() { s.dismissed[i] = true }) },
				m.NewText(scope.T("notice.dismiss"))), m.AlignOpts{Alignment: m.AlignmentCenterRight}),
		}, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStretch})))
		children = append(children, card, gap(Gap))
	}
	return m.NewColumn(children, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStretch, MainAxisSize: m.MainAxisSizeMin})
}

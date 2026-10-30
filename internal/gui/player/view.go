package player

import (
	"github.com/rm4n0s/glutter/foundation"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/glutter/painting"
	"github.com/rm4n0s/glutter/rendering"
	"github.com/rm4n0s/glutter/widgets"
	"github.com/rm4n0s/radio-tutor/internal/core/timeline"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// Build shows the loading indicator, the failure, or the player.
func (s *lessonState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	req := s.Widget().req
	title := ""
	if l := req.Cassette.Lessons[req.LessonID]; l != nil {
		title = l.Title
	}
	var body m.Widget
	switch {
	case s.failed:
		body = centeredMessage(m.NewText(scope.T("player.failed"), m.TextOpts{Style: m.ThemeOf(bc).TextTheme.BodyLarge}))
	case !s.loaded:
		body = centeredMessage(m.NewColumn([]m.Widget{
			m.NewCircularProgressIndicator(m.CircularProgressIndicatorOpts{Key: m.NewValueKey(KeyLoading)}),
			gap(app.Gap),
			m.NewText(scope.T("player.loading"), m.TextOpts{Style: m.ThemeOf(bc).TextTheme.BodyLarge}),
		}, m.ColumnOpts{MainAxisSize: m.MainAxisSizeMin}))
	default:
		body = s.loadedView(bc, scope)
	}
	return s.shortcuts(m.NewScaffold(m.ScaffoldOpts{
		AppBar: m.NewAppBar(m.AppBarOpts{
			Leading: m.NewIconButton(s.back, m.NewIcon(m.Icons.ArrowBack()), m.IconButtonOpts{
				Key:     m.NewValueKey(KeyBack),
				Tooltip: scope.T("player.back"),
			}),
			Title: m.NewText(title),
		}),
		Body: body,
	}))
}

// loadedView is the loaded player: the stage, the notice, the captions and the
// controls.
func (s *lessonState) loadedView(bc m.BuildContext, scope *app.Scope) m.Widget {
	s.states = s.eval.Evaluate(s.pos, s.states)
	stage := m.NewPadding(m.NewEdgeInsetsAll(stagePadding), widgets.NewLayoutBuilder(
		func(_ m.BuildContext, c rendering.BoxConstraints) m.Widget {
			return m.NewCenter(m.NewKeyedSubtree(s.stage.widget(c.MaxWidth, c.MaxHeight, s.states), m.WidgetOpts{Key: m.NewValueKey(KeyStage)}))
		},
	))
	children := []m.Widget{m.NewExpanded(stage)}
	if s.notice != "" {
		children = append(children, s.noticeCard(scope))
	}
	children = append(children, s.captionBar(bc), s.controls(bc, scope))
	return m.NewColumn(children, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStretch})
}

// noticeCard tells the child something about the sound, with an OK button.
func (s *lessonState) noticeCard(scope *app.Scope) m.Widget {
	return m.NewPadding(m.NewEdgeInsetsSymmetric(4, app.Gap), m.NewCard(m.NewPadding(m.NewEdgeInsetsAll(12), m.NewRow([]m.Widget{
		m.NewIcon(m.Icons.VolumeOff()),
		hgap(12),
		m.NewExpanded(m.NewText(scope.T(s.notice), m.TextOpts{Key: m.NewValueKey(KeyNotice)})),
		hgap(12),
		m.NewTextButton(func() { s.SetState(func() { s.notice = "" }) }, m.NewText(scope.T("player.ok")), m.TextButtonOpts{Key: m.NewValueKey(KeyNoticeOK)}),
	}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter}))))
}

// captionBar is the captions below the stage: a fixed height of two lines of
// 24 logical pixels, so the layout never jumps (SPEC §8).
func (s *lessonState) captionBar(bc m.BuildContext) m.Widget {
	text := ""
	if c, ok := timeline.CaptionAt(s.captions, s.pos); ok {
		text = c.Text
	}
	style := m.ThemeOf(bc).TextTheme.BodyLarge.CopyWith(m.TextStyleCopy{
		FontSize: m.Some(captionFontSize),
		Height:   m.Some(captionLineGap),
	})
	return m.NewSizedBox(
		m.NewPadding(m.NewEdgeInsetsSymmetric(captionPadding, app.Gap), m.NewCenter(m.NewText(text, m.TextOpts{
			Key:       m.NewValueKey(KeyCaption),
			Style:     style,
			TextAlign: m.TextAlignCenter,
			MaxLines:  foundation.Some(captionLines),
			Overflow:  painting.TextOverflowEllipsis,
		}))),
		m.SizedBoxOpts{Key: m.NewValueKey(KeyCaptionBar), Height: m.Some(captionFontSize*captionLineGap*captionLines + 2*captionPadding)},
	)
}

// controls are the progress bar with the time, and below them the buttons.
func (s *lessonState) controls(bc m.BuildContext, scope *app.Scope) m.Widget {
	theme := m.ThemeOf(bc)
	fraction := 0.0
	if s.duration > 0 {
		fraction = min(1, s.pos/s.duration)
	}
	bar := m.NewRow([]m.Widget{
		m.NewExpanded(m.NewLinearProgressIndicator(m.LinearProgressIndicatorOpts{
			Key:       m.NewValueKey(KeyProgress),
			Value:     m.Some(fraction),
			MinHeight: m.Some(12.0),
		})),
		hgap(app.Gap),
		m.NewText(formatTime(s.pos)+" / "+formatTime(s.duration), m.TextOpts{
			Key:   m.NewValueKey(KeyTime),
			Style: theme.TextTheme.TitleMedium,
		}),
	}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter})

	label, icon := scope.T("player.play"), m.Icons.PlayArrow()
	if s.playing {
		label, icon = scope.T("player.pause"), m.Icons.Pause()
	}
	// The buttons of the lesson are on the left and wrap onto a second line when
	// the window is narrow (Greek labels with Skip and Next at the minimum
	// width); Skip and Next stay on the right.
	left := m.NewWrap([]m.Widget{
		m.NewFilledButtonIcon(s.toggle, m.NewText(label), m.FilledButtonIconOpts{Icon: m.NewIcon(icon), Key: m.NewValueKey(KeyPlay)}),
		m.NewOutlinedButtonIcon(func() { s.send(cmdStop) }, m.NewText(scope.T("player.stop")), m.OutlinedButtonIconOpts{
			Icon: m.NewIcon(m.Icons.Stop()),
			Key:  m.NewValueKey(KeyStop),
		}),
		m.NewRow([]m.Widget{s.soundButton(scope), s.volumeSlider(scope)}, m.RowOpts{MainAxisSize: m.MainAxisSizeMin, CrossAxisAlignment: m.CrossAxisAlignmentCenter}),
	}, m.WrapOpts{Spacing: app.Gap, RunSpacing: app.Gap / 2, CrossAxisAlignment: m.WrapCrossAlignmentCenter})
	buttons := []m.Widget{m.NewExpanded(left)}
	req := s.Widget().req
	if s.skip && req.OpenNext != nil {
		buttons = append(buttons, hgap(app.Gap), m.NewTextButton(s.forward, m.NewText(scope.T("player.skip")), m.TextButtonOpts{Key: m.NewValueKey(KeySkip)}))
	}
	if s.completed {
		next := scope.T("player.next")
		if req.OpenNext == nil {
			next = scope.T("player.done")
		}
		buttons = append(buttons, hgap(app.Gap), m.NewFilledButton(s.forward, m.NewText(next), m.FilledButtonOpts{Key: m.NewValueKey(KeyNext)}))
	}
	return m.NewPadding(m.NewEdgeInsetsSymmetric(app.Gap, app.Gap), m.NewColumn([]m.Widget{
		bar,
		gap(app.Gap),
		m.NewRow(buttons, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter}),
	}, m.ColumnOpts{MainAxisSize: m.MainAxisSizeMin, CrossAxisAlignment: m.CrossAxisAlignmentStretch}))
}

// centeredMessage centers a widget with padding.
func centeredMessage(w m.Widget) m.Widget {
	return m.NewCenter(m.NewPadding(m.NewEdgeInsetsAll(app.PagePadding), w))
}

// gap is empty vertical space.
func gap(height float64) m.Widget {
	return m.NewSizedBox(nil, m.SizedBoxOpts{Height: m.Some(height)})
}

// hgap is empty horizontal space.
func hgap(width float64) m.Widget {
	return m.NewSizedBox(nil, m.SizedBoxOpts{Width: m.Some(width)})
}

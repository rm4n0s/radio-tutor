package student

import (
	"context"
	"sync/atomic"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

const (
	cassetteMaxWidth = 640.0
	rowIconSize      = 40.0
	markIconSize     = 28.0
)

// Cassette is the screen of one cassette (SPEC §13): its items in order, the
// lessons with a play icon, each worksheet as one row with the labels and states
// of its exercises, locked items greyed with a lock, and the Continue button.
type Cassette struct {
	m.StatefulWidget
	username string
	id       string
	screens  Screens
}

// NewCassette returns the screen of the installed cassette id for the student
// with the username.
func NewCassette(username, id string, screens Screens, opts ...m.WidgetOpts) *Cassette {
	return &Cassette{StatefulWidget: m.NewStatefulWidgetBase(opts...), username: username, id: id, screens: screens}
}

// CreateState creates the state.
func (w *Cassette) CreateState() m.State { return &cassetteState{} }

type cassetteState struct {
	m.StateOf[*Cassette]
	started    bool
	generation atomic.Int64
	view       *view
	failed     bool
}

// DidChangeDependencies starts the first load (the Scope cannot be read in
// InitState).
func (s *cassetteState) DidChangeDependencies() {
	if s.started {
		return
	}
	s.started = true
	env, w := app.ScopeOf(s.Context()).Env(), s.Widget()
	s.Go(func(ctx context.Context) { s.load(ctx, env, w.username, w.id) })
}

// load opens the cassette and shows it; it runs on a goroutine, so it never
// reads a widget.
func (s *cassetteState) load(ctx context.Context, env *app.Env, username, id string) {
	gen := s.generation.Add(1)
	v, err := loadCassette(ctx, env, username, id)
	if err != nil && ctx.Err() == nil {
		env.Log.Error("opening a cassette failed", "username", username, "cassette", id, "err", err)
	}
	s.SetState(func() {
		if s.generation.Load() != gen {
			return
		}
		s.view, s.failed = v, err != nil
	})
}

func (s *cassetteState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	title := ""
	var body m.Widget
	switch {
	case s.failed:
		body = pageBody(cassetteMaxWidth, []m.Widget{
			m.NewText(scope.T("student.cassette.load_failed"), m.TextOpts{Style: m.ThemeOf(bc).TextTheme.BodyLarge}),
		})
	case s.view == nil:
		body = m.NewCenter(m.NewCircularProgressIndicator())
	default:
		title = s.view.Cassette.Manifest.Title
		body = pageBody(cassetteMaxWidth, s.content(bc, scope, s.view))
	}
	return m.NewScaffold(m.ScaffoldOpts{
		AppBar: m.NewAppBar(m.AppBarOpts{
			Title:   m.NewText(title),
			Actions: []m.Widget{logoutButton(scope, bc)},
		}),
		Body: body,
	})
}

// content is the header, the Continue button and the rows.
func (s *cassetteState) content(bc m.BuildContext, scope *app.Scope, v *view) []m.Widget {
	theme := m.ThemeOf(bc)
	mf := v.Cassette.Manifest
	fraction := 0.0
	if v.Total > 0 {
		fraction = float64(v.Done) / float64(v.Total)
	}
	header := m.NewRow([]m.Widget{
		m.NewClipRRect(coverPicture(bc, v.Cover, 90, 120), m.ClipRRectOpts{BorderRadius: m.NewBorderRadiusCircular(8)}),
		hgap(app.Gap),
		m.NewExpanded(m.NewColumn([]m.Widget{
			m.NewText(mf.Subject, m.TextOpts{Style: theme.TextTheme.TitleMedium}),
			gap(8),
			m.NewLinearProgressIndicator(m.LinearProgressIndicatorOpts{Value: m.Some(fraction), MinHeight: m.Some(10.0)}),
			gap(4),
			m.NewText(scope.T("student.progress", "done", itoa(v.Done), "total", itoa(v.Total)), m.TextOpts{Style: theme.TextTheme.BodyMedium}),
		}, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart})),
	}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter})
	children := []m.Widget{
		header,
		gap(app.Gap),
		m.NewFilledButtonIcon(func() { s.open(s.Context(), v, v.Continue) }, m.NewText(scope.T("student.cassette.continue")), m.FilledButtonIconOpts{
			Icon:  m.NewIcon(m.Icons.PlayArrow()),
			Key:   m.NewValueKey(KeyContinue),
			Style: m.FilledButtonStyleFrom(m.ButtonStyleFromOpts{MinimumSize: m.Some(m.Size{Width: app.ButtonMinWidth, Height: 72})}),
		}),
		gap(app.Gap),
	}
	worksheets := map[int]cassette.Worksheet{}
	for _, w := range v.Cassette.Worksheets() {
		worksheets[w.Start] = w
	}
	for i, it := range mf.Items {
		switch {
		case it.Type == cassette.ItemLesson:
			children = append(children, s.lessonRow(bc, scope, v, it))
		case worksheets[i].ExerciseIDs != nil:
			children = append(children, s.worksheetRow(bc, scope, v, worksheets[i]))
		}
	}
	return children
}

// rowCard is a row of the item list: a roomy card that can be tapped, greyed
// when locked.
func rowCard(key string, locked bool, onTap func(), leading, content, trailing m.Widget) m.Widget {
	row := m.NewPadding(m.NewEdgeInsetsAll(app.Gap), m.NewRow([]m.Widget{
		leading, hgap(app.Gap), m.NewExpanded(content), hgap(app.Gap), trailing,
	}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter}))
	opacity := 1.0
	if locked {
		opacity = 0.5
	}
	return m.NewCard(m.NewOpacity(opacity, m.NewInkWell(
		m.NewConstrainedBox(m.NewBoxConstraints(m.BoxConstraintsOpts{MinHeight: m.Some(72.0)}), row),
		m.InkWellOpts{Key: m.NewValueKey(key), OnTap: onTap},
	)), m.CardOpts{ClipBehavior: m.ClipAntiAlias})
}

// lessonRow is the row of a lesson: a play icon, the title and the state.
func (s *cassetteState) lessonRow(bc m.BuildContext, scope *app.Scope, v *view, it cassette.Item) m.Widget {
	theme := m.ThemeOf(bc)
	state := v.States[it.ID]
	title := theme.TextTheme.TitleMedium.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600})
	small := theme.TextTheme.BodySmall.CopyWith(m.TextStyleCopy{Color: m.Some(theme.ColorScheme.OnSurfaceVariant())})
	rowKey := KeyItemPrefix + it.ID
	return rowCard(rowKey, state == progress.StateLocked,
		func() { s.open(s.Context(), v, it) },
		m.NewIcon(m.Icons.PlayCircle(), m.IconOpts{Size: m.Some(rowIconSize), Color: m.Some(theme.ColorScheme.Primary())}),
		m.NewColumn([]m.Widget{
			m.NewText(v.Cassette.Lessons[it.ID].Title, m.TextOpts{Style: title}),
			m.NewText(scope.T("student.cassette.lesson"), m.TextOpts{Style: small}),
		}, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart}),
		trailingMark(theme, scope, state, rowKey),
	)
}

// worksheetRow is the row of a worksheet: one row with the label and state of
// each of its exercises.
func (s *cassetteState) worksheetRow(bc m.BuildContext, scope *app.Scope, v *view, w cassette.Worksheet) m.Widget {
	theme := m.ThemeOf(bc)
	state := worksheetState(w, v.States)
	title := theme.TextTheme.TitleMedium.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600})
	var chips []m.Widget
	for _, id := range w.ExerciseIDs {
		chips = append(chips, exerciseChip(theme, scope, id, v.Cassette.Exercises[id].Label, v.States[id]))
	}
	rowKey := WorksheetKey(w.Index)
	return rowCard(rowKey, state == progress.StateLocked,
		func() { s.open(s.Context(), v, cassette.Item{Type: cassette.ItemExercise, ID: w.ExerciseIDs[0]}) },
		m.NewIcon(m.Icons.EditNote(), m.IconOpts{Size: m.Some(rowIconSize), Color: m.Some(theme.ColorScheme.Primary())}),
		m.NewColumn([]m.Widget{
			m.NewText(scope.T("student.cassette.worksheet", "n", itoa(w.Index+1)), m.TextOpts{Style: title}),
			gap(8),
			m.NewWrap(chips, m.WrapOpts{Spacing: 8, RunSpacing: 8}),
		}, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart}),
		trailingMark(theme, scope, state, rowKey),
	)
}

// exerciseChip is the label of an exercise with the mark of its state.
func exerciseChip(theme *m.ThemeData, scope *app.Scope, id, label string, state progress.ItemState) m.Widget {
	children := []m.Widget{m.NewText(label, m.TextOpts{Style: theme.TextTheme.TitleMedium})}
	if mark := stateMark(theme, scope, state, ""); mark != nil {
		children = append(children, hgap(6), mark)
	}
	return m.NewContainer(m.NewRow(children, m.RowOpts{MainAxisSize: m.MainAxisSizeMin, CrossAxisAlignment: m.CrossAxisAlignmentCenter}), m.ContainerOpts{
		Key:     m.NewValueKey(KeyExercisePrefix + id),
		Padding: m.NewEdgeInsetsSymmetric(8, 14),
		Decoration: m.NewBoxDecoration(m.BoxDecorationOpts{
			Color:        m.Some(theme.ColorScheme.SurfaceContainerHighest()),
			BorderRadius: m.NewBorderRadiusCircular(20),
		}),
	})
}

// trailingMark is the state mark at the end of a row; an item that is simply
// available gets empty space of the same size.
func trailingMark(theme *m.ThemeData, scope *app.Scope, state progress.ItemState, rowKey string) m.Widget {
	if mark := stateMark(theme, scope, state, rowKey+KeyStateSuffix); mark != nil {
		return mark
	}
	return m.NewSizedBox(nil, m.SizedBoxOpts{Width: m.Some(markIconSize)})
}

// stateMark is the icon of a state with a tooltip that names it: a lock for
// locked, a check for completed and passed, a flag for needs review. An item
// that is simply available has none (nil).
func stateMark(theme *m.ThemeData, scope *app.Scope, state progress.ItemState, key string) m.Widget {
	var icon m.IconData
	var color func() m.Color
	switch state {
	case progress.StateLocked:
		icon, color = m.Icons.Lock(), theme.ColorScheme.OnSurfaceVariant
	case progress.StateCompleted, progress.StatePassed:
		icon, color = m.Icons.CheckCircle(), theme.ColorScheme.Primary
	case progress.StateNeedsReview:
		icon, color = m.Icons.Flag(), theme.ColorScheme.Error
	default:
		return nil
	}
	mark := m.NewTooltip(
		m.NewIcon(icon, m.IconOpts{Size: m.Some(markIconSize), Color: m.Some(color())}),
		m.TooltipOpts{Message: scope.T("student.state." + string(state))},
	)
	if key == "" {
		return mark
	}
	return m.NewSizedBox(mark, m.SizedBoxOpts{Key: m.NewValueKey(key)})
}

// open opens a lesson, or the worksheet of an exercise, or says that it is
// locked. The item is remembered as last_item so that Continue returns to it,
// and the cassette is read again when the screen above closes.
func (s *cassetteState) open(bc m.BuildContext, v *view, it cassette.Item) {
	scope := app.ScopeOf(bc)
	w := s.Widget()
	var builder m.WidgetBuilder
	switch it.Type {
	case cassette.ItemLesson:
		if v.States[it.ID] == progress.StateLocked {
			showMessage(bc, scope.T("student.cassette.locked"))
			return
		}
		req := LessonRequest{Username: w.username, Cassette: v.Cassette, FS: v.FS, LessonID: it.ID}
		builder = func(m.BuildContext) m.Widget {
			if w.screens.Lesson != nil {
				return w.screens.Lesson(req)
			}
			return comingSoon("student.soon.lesson", "title", v.Cassette.Lessons[it.ID].Title)
		}
	default:
		ws, ok := v.Cassette.WorksheetOf(it.ID)
		if !ok {
			return
		}
		if worksheetState(ws, v.States) == progress.StateLocked {
			showMessage(bc, scope.T("student.cassette.locked"))
			return
		}
		req := WorksheetRequest{Username: w.username, Cassette: v.Cassette, FS: v.FS, Worksheet: ws}
		builder = func(m.BuildContext) m.Widget {
			if w.screens.Worksheet != nil {
				return w.screens.Worksheet(req)
			}
			return comingSoon("student.soon.worksheet", "n", itoa(ws.Index+1))
		}
	}
	env := scope.Env()
	ch := app.Push(bc, builder)
	s.Go(func(ctx context.Context) {
		err := env.Progress.Update(ctx, w.username, w.id, func(p *progress.Progress) error {
			return progress.SetLastItem(v.Cassette, p, it.ID)
		})
		if err != nil && !isBrokenFile(err) && ctx.Err() == nil {
			env.Log.Error("saving the last item failed", "username", w.username, "cassette", w.id, "err", err)
		}
		select {
		case <-ch:
			s.load(ctx, env, w.username, w.id)
		case <-ctx.Done():
		}
	})
}

// comingSoon is the placeholder of a lesson or a worksheet screen that a later
// step provides (6.4 the lesson player, 6.5 the worksheet).
func comingSoon(key string, args ...string) m.Widget {
	return m.NewBuilder(func(bc m.BuildContext) m.Widget {
		scope := app.ScopeOf(bc)
		return m.NewScaffold(m.ScaffoldOpts{
			AppBar: m.NewAppBar(m.AppBarOpts{Actions: []m.Widget{logoutButton(scope, bc)}}),
			Body: pageBody(cassetteMaxWidth, []m.Widget{
				m.NewText(scope.T(key, args...), m.TextOpts{Style: m.ThemeOf(bc).TextTheme.BodyLarge}),
			}),
		})
	})
}

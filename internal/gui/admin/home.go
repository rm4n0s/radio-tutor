package admin

import (
	"context"
	"sync/atomic"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// Sizes of the home, in logical pixels.
const (
	homeMaxWidth = 760.0
	tileWidth    = 224.0
	tileHeight   = 176.0
	tileGap      = 16.0
)

// Home is the home of an admin (SPEC §12.1): tiles for Students, Cassettes,
// Progress, Settings, AI providers and Play as me, the notices from startup and
// the count of exercises waiting for review.
type Home struct {
	m.StatefulWidget
	screens Screens
}

// NewHome returns the admin home.
func NewHome(screens Screens, opts ...m.WidgetOpts) *Home {
	return &Home{StatefulWidget: m.NewStatefulWidgetBase(opts...), screens: screens}
}

// CreateState creates the state.
func (w *Home) CreateState() m.State { return &homeState{} }

type homeState struct {
	m.StateOf[*Home]
	started    bool
	generation atomic.Int64
	reviews    int
	// noticed is true once the notice at login was decided: it is shown at most
	// once per home (that is, per login).
	noticed bool
}

// DidChangeDependencies starts the first count (the Scope cannot be read in
// InitState).
func (s *homeState) DidChangeDependencies() {
	if s.started {
		return
	}
	s.started = true
	s.reload()
}

// reload counts the exercises that wait for review on a goroutine.
func (s *homeState) reload() {
	env := app.ScopeOf(s.Context()).Env()
	s.Go(func(ctx context.Context) { s.load(ctx, env) })
}

// load counts and shows the number; it runs on a goroutine. After the first
// count of a login that found exercises waiting, it opens the notice (SPEC
// §12.1).
func (s *homeState) load(ctx context.Context, env *app.Env) {
	gen := s.generation.Add(1)
	items, err := scanQueue(ctx, env)
	if err != nil {
		if ctx.Err() == nil {
			env.Log.Error("counting the exercises to review failed", "err", err)
		}
		return
	}
	s.SetState(func() {
		if s.generation.Load() != gen {
			return
		}
		s.reviews = len(items)
		if s.noticed {
			return
		}
		s.noticed = true
		if len(items) > 0 {
			s.notify(items)
		}
	})
}

// notify opens the notice that exercises are waiting; Review now opens the
// review screen. It runs on the UI thread.
func (s *homeState) notify(items []waitingItem) {
	showDialog(s, func(m.BuildContext) m.Widget { return newReviewNotice(items) }, func(now bool) {
		if now {
			s.open(s.Context(), func(m.BuildContext) m.Widget { return NewReviewQueue() })
		}
	})
}

// open pushes a screen and counts again when it closes.
func (s *homeState) open(bc m.BuildContext, build m.WidgetBuilder) {
	env := app.ScopeOf(bc).Env()
	ch := app.Push(bc, build)
	s.Go(func(ctx context.Context) {
		select {
		case <-ch:
			s.load(ctx, env)
		case <-ctx.Done():
		}
	})
}

func (s *homeState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	env := scope.Env()
	providerText := scope.T("admin.home.providers_none")
	switch active, ok := activeProvider(env); {
	case env.Providers == nil:
		providerText = scope.T("admin.home.providers_unusable")
	case ok:
		providerText = scope.T("admin.home.providers_active", "name", active.Name, "kind", scope.T("admin.provider.kind."+string(active.Kind)))
	}
	reviewText := scope.T("admin.home.review", "n", itoa(s.reviews))
	tiles := []m.Widget{
		s.tile(bc, KeyStudents, m.Icons.People(), scope.T("admin.students"), mutedShort(bc, scope.T("admin.home.students_hint")),
			func() { s.open(bc, func(m.BuildContext) m.Widget { return NewStudents() }) }),
		s.tile(bc, KeyCassettes, m.Icons.LibraryBooks(), scope.T("admin.cassettes"), mutedShort(bc, scope.T("admin.home.cassettes_hint")),
			func() { s.open(bc, func(m.BuildContext) m.Widget { return NewCassettes() }) }),
		s.tile(bc, KeyProgress, m.Icons.Insights(), scope.T("admin.progress"), s.reviewLine(bc, reviewText),
			func() { s.open(bc, s.progressScreen) }),
		s.tile(bc, KeySettings, m.Icons.Settings(), scope.T("admin.settings"), mutedShort(bc, scope.T("admin.home.settings_hint")),
			func() { s.open(bc, func(m.BuildContext) m.Widget { return NewSettings(s.Widget().screens) }) }),
		s.tile(bc, KeyProviders, m.Icons.SmartToy(), scope.T("admin.providers"), mutedShort(bc, providerText),
			func() { s.open(bc, func(m.BuildContext) m.Widget { return NewProviders() }) }),
		s.tile(bc, KeyPlayAsMe, m.Icons.PlayArrow(), scope.T("admin.play"), mutedShort(bc, scope.T("admin.home.play_hint")),
			func() { s.open(bc, s.playAsMe) }),
	}
	return m.NewScaffold(m.ScaffoldOpts{
		AppBar: m.NewAppBar(m.AppBarOpts{
			Title:   m.NewText(scope.T("home.admin.title")),
			Actions: []m.Widget{logoutButton(scope, bc)},
		}),
		Body: pageBody(homeMaxWidth, []m.Widget{
			app.NewNotices(),
			headline(bc, scope.T("home.admin.hello", "name", scope.Session().DisplayName)),
			gap(app.Gap),
			m.NewWrap(tiles, m.WrapOpts{Spacing: tileGap, RunSpacing: tileGap}),
		}),
	})
}

// reviewLine is the "n waiting for review" text of the Progress tile; it is
// bold when something waits.
func (s *homeState) reviewLine(bc m.BuildContext, text string) m.Widget {
	theme := m.ThemeOf(bc)
	style := theme.TextTheme.BodyMedium.CopyWith(m.TextStyleCopy{Color: m.Some(theme.ColorScheme.OnSurfaceVariant())})
	if s.reviews > 0 {
		style = theme.TextTheme.BodyMedium.CopyWith(m.TextStyleCopy{
			Color: m.Some(theme.ColorScheme.Error()), FontWeight: m.FontWeightW700,
		})
	}
	return m.NewText(text, m.TextOpts{Key: m.NewValueKey(KeyReviewCount), Style: style, MaxLines: m.Some(2), Overflow: m.TextOverflowEllipsis})
}

// progressScreen is the progress screen of step 6.6, or a placeholder.
func (s *homeState) progressScreen(bc m.BuildContext) m.Widget {
	if f := s.Widget().screens.Progress; f != nil {
		return f()
	}
	scope := app.ScopeOf(bc)
	return m.NewScaffold(m.ScaffoldOpts{
		AppBar: appBar(scope, bc, scope.T("admin.progress")),
		Body:   pageBody(pageWidth, []m.Widget{body(bc, scope.T("admin.progress.soon"))}),
	})
}

// playAsMe is the shelf of the admin, the same screen a student has (SPEC §3.3).
func (s *homeState) playAsMe(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	if f := scope.Env().Screens.Student; f != nil {
		return f(scope.Session().Username)
	}
	return app.NewStudentHome()
}

// tile is a big tappable card of the home.
func (s *homeState) tile(bc m.BuildContext, key string, icon m.IconData, title string, subtitle m.Widget, onTap func()) m.Widget {
	theme := m.ThemeOf(bc)
	titleStyle := theme.TextTheme.TitleLarge.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600})
	return m.NewSizedBox(m.NewCard(
		m.NewInkWell(m.NewPadding(m.NewEdgeInsetsAll(16), m.NewColumn([]m.Widget{
			m.NewIcon(icon, m.IconOpts{Size: m.Some(36.0), Color: m.Some(theme.ColorScheme.Primary())}),
			gap(8),
			m.NewText(title, m.TextOpts{Style: titleStyle, MaxLines: m.Some(1), Overflow: m.TextOverflowEllipsis}),
			gap(4),
			subtitle,
		}, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart})),
			m.InkWellOpts{Key: m.NewValueKey(key), OnTap: onTap}),
		m.CardOpts{ClipBehavior: m.ClipAntiAlias},
	), m.SizedBoxOpts{Width: m.Some(tileWidth), Height: m.Some(tileHeight)})
}

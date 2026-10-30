package student

import (
	"context"
	"sync/atomic"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// Sizes of the shelf, in logical pixels: four cards fit a row of the smallest
// window.
const (
	shelfMaxWidth = 800.0
	cardWidth     = 168.0
	coverHeight   = 224.0 // the cover is 3:4
	cardGap       = 16.0
)

// Shelf is the home of a student (SPEC §13): the cassettes the student may use
// as cards with cover, title, subject and progress bar, the Import button when
// allowed, the student's name and Log out at the top.
type Shelf struct {
	m.StatefulWidget
	username string
	screens  Screens
}

// NewShelf returns the shelf of the student with the username. The admin's Play
// as me shows it for the admin's own username.
func NewShelf(username string, screens Screens, opts ...m.WidgetOpts) *Shelf {
	return &Shelf{StatefulWidget: m.NewStatefulWidgetBase(opts...), username: username, screens: screens}
}

// CreateState creates the state.
func (w *Shelf) CreateState() m.State { return &shelfState{} }

type shelfState struct {
	m.StateOf[*Shelf]
	started bool
	// generation numbers the loads, so that a slow old one never overwrites a
	// newer one.
	generation atomic.Int64
	cards      []card
	loaded     bool
	loadFailed bool
	importing  bool
	cancel     context.CancelFunc
}

// DidChangeDependencies starts the first load (the Scope cannot be read in
// InitState).
func (s *shelfState) DidChangeDependencies() {
	if s.started {
		return
	}
	s.started = true
	s.reload()
}

// reload reads the shelf again on a goroutine. It runs on the UI thread: the
// goroutine gets what it needs from the widgets as arguments.
func (s *shelfState) reload() {
	env, username := app.ScopeOf(s.Context()).Env(), s.Widget().username
	s.Go(func(ctx context.Context) { s.load(ctx, env, username) })
}

// load reads the shelf and shows it; it runs on a goroutine, so it never reads
// a widget.
func (s *shelfState) load(ctx context.Context, env *app.Env, username string) {
	gen := s.generation.Add(1)
	cards, err := loadShelf(ctx, env, username)
	if err != nil {
		if ctx.Err() == nil {
			env.Log.Error("loading the shelf failed", "username", username, "err", err)
		}
		s.SetState(func() {
			if s.generation.Load() == gen {
				s.loaded, s.loadFailed, s.cards = true, true, nil
			}
		})
		return
	}
	s.SetState(func() {
		if s.generation.Load() == gen {
			s.loaded, s.loadFailed, s.cards = true, false, cards
		}
	})
}

func (s *shelfState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	canImport := scope.Session().Role == profile.RoleAdmin || scope.Env().Settings.Get().StudentsCanImport
	children := []m.Widget{headline(bc, scope.T("home.student.title")), gap(app.Gap)}
	if canImport {
		children = append(children, s.importArea(bc, scope), gap(app.Gap))
	}
	children = append(children, s.shelf(bc, scope))
	return m.NewScaffold(m.ScaffoldOpts{
		AppBar: m.NewAppBar(m.AppBarOpts{
			Title: m.NewRow([]m.Widget{
				m.NewIcon(m.Icons.Person()), hgap(8),
				m.NewText(scope.T("home.student.hello", "name", scope.Session().DisplayName)),
			}, m.RowOpts{MainAxisSize: m.MainAxisSizeMin}),
			Actions: []m.Widget{logoutButton(scope, bc)},
		}),
		Body: pageBody(shelfMaxWidth, children),
	})
}

// importArea is the Import button, or the progress indicator with Cancel while
// an import runs.
func (s *shelfState) importArea(bc m.BuildContext, scope *app.Scope) m.Widget {
	if !s.importing {
		return m.NewAlign(m.NewFilledButtonIcon(s.startImport, m.NewText(scope.T("student.import")), m.FilledButtonIconOpts{
			Icon: m.NewIcon(m.Icons.FileOpen()),
			Key:  m.NewValueKey(KeyImport),
		}), m.AlignOpts{Alignment: m.AlignmentCenterLeft})
	}
	return m.NewRow([]m.Widget{
		m.NewSizedBox(m.NewCircularProgressIndicator(), m.SizedBoxOpts{Width: m.Some(36.0), Height: m.Some(36.0)}),
		hgap(app.Gap),
		m.NewExpanded(m.NewText(scope.T("student.import.working"), m.TextOpts{Style: m.ThemeOf(bc).TextTheme.BodyLarge})),
		hgap(app.Gap),
		m.NewOutlinedButton(s.cancelImport, m.NewText(scope.T("student.import.cancel")), m.OutlinedButtonOpts{Key: m.NewValueKey(KeyImportCancel)}),
	}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter})
}

// cancelImport stops the running import (and closes the file dialog).
func (s *shelfState) cancelImport() {
	if s.cancel != nil {
		s.cancel()
	}
}

// shelf is the cards, or the message that stands in for them.
func (s *shelfState) shelf(bc m.BuildContext, scope *app.Scope) m.Widget {
	body := m.ThemeOf(bc).TextTheme.BodyLarge
	switch {
	case !s.loaded:
		return m.NewCenter(m.NewCircularProgressIndicator())
	case s.loadFailed:
		return m.NewText(scope.T("student.shelf.load_failed"), m.TextOpts{Style: body})
	case len(s.cards) == 0:
		return m.NewText(scope.T("student.shelf.empty"), m.TextOpts{Style: body})
	}
	var cards []m.Widget
	for _, c := range s.cards {
		cards = append(cards, s.card(bc, scope, c))
	}
	return m.NewWrap(cards, m.WrapOpts{Spacing: cardGap, RunSpacing: cardGap})
}

// card is the card of one cassette; tapping it opens the cassette.
func (s *shelfState) card(bc m.BuildContext, scope *app.Scope, c card) m.Widget {
	theme := m.ThemeOf(bc)
	fraction := 0.0
	if c.Total > 0 {
		fraction = float64(c.Done) / float64(c.Total)
	}
	title := theme.TextTheme.TitleMedium.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600})
	subject := theme.TextTheme.BodyMedium.CopyWith(m.TextStyleCopy{Color: m.Some(theme.ColorScheme.OnSurfaceVariant())})
	info := m.NewPadding(m.NewEdgeInsetsAll(12), m.NewColumn([]m.Widget{
		m.NewText(c.Title, m.TextOpts{Style: title, MaxLines: m.Some(2), Overflow: m.TextOverflowEllipsis}),
		gap(2),
		m.NewText(c.Subject, m.TextOpts{Style: subject, MaxLines: m.Some(1), Overflow: m.TextOverflowEllipsis}),
		gap(10),
		m.NewLinearProgressIndicator(m.LinearProgressIndicatorOpts{
			Key:       m.NewValueKey(KeyProgressPrefix + c.ID),
			Value:     m.Some(fraction),
			MinHeight: m.Some(10.0),
		}),
		gap(4),
		m.NewText(scope.T("student.progress", "done", itoa(c.Done), "total", itoa(c.Total)), m.TextOpts{Style: theme.TextTheme.BodySmall}),
	}, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart}))
	w, env := s.Widget(), scope.Env()
	open := func() {
		ch := app.Push(bc, func(m.BuildContext) m.Widget { return NewCassette(w.username, c.ID, w.screens) })
		// The student may have made progress: read the shelf again when the
		// cassette screen closes.
		s.Go(func(ctx context.Context) {
			select {
			case <-ch:
				s.load(ctx, env, w.username)
			case <-ctx.Done():
			}
		})
	}
	return m.NewSizedBox(m.NewCard(
		m.NewInkWell(m.NewColumn([]m.Widget{coverPicture(bc, c.Cover, cardWidth, coverHeight), info},
			m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStretch}),
			m.InkWellOpts{Key: m.NewValueKey(KeyCardPrefix + c.ID), OnTap: open}),
		m.CardOpts{ClipBehavior: m.ClipAntiAlias},
	), m.SizedBoxOpts{Width: m.Some(cardWidth)})
}

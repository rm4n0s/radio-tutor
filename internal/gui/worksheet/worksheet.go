// Package worksheet is the worksheet area of the student screens (SPEC §9 and
// §10.1): the worksheet with a card per exercise, the submission flow (the QR
// code for the phone, the grading indicator, the results and the failure
// screen) and the playing of the prompt audio.
//
// The submission flow talks to the submission service of app.Env only through
// its events: a goroutine reads Session.Events and hands each event to the UI
// thread with SetState, so no widget is touched from another goroutine.
package worksheet

import (
	"context"
	"strconv"
	"sync/atomic"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/submission"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/student"
)

// Keys of the widgets that tests look for.
const (
	// KeyBanner is the key of the banner that tells the child to write the
	// labels on the paper.
	KeyBanner = "banner"
	// KeyCheck is the key of the Check my answers button.
	KeyCheck = "check"
	// KeyCardPrefix starts the key of the card of an exercise; the exercise id
	// follows.
	KeyCardPrefix = "card-"
	// KeyNext is the key of the Next (or Done) button that replaces Check my
	// answers when every exercise of the worksheet is passed or waits for a
	// parent.
	KeyNext = "next"
	// KeyBadgePrefix starts the key of the label badge of a card.
	KeyBadgePrefix = "badge-"
	// KeySelectPrefix starts the key of the checkbox of a card.
	KeySelectPrefix = "select-"
	// KeySpeakerPrefix starts the key of the speaker button of a card.
	KeySpeakerPrefix = "speak-"
	// KeyHintPrefix starts the key of a hint of a card or of a result; the
	// exercise id follows.
	KeyHintPrefix = "hint-"
	// KeyLoadFailed is the key of the message shown when the worksheet cannot
	// be read.
	KeyLoadFailed = "load-failed"
)

// Screen is the worksheet screen (SPEC §9): the banner, one card per exercise
// of the worksheet, the checkboxes and Check my answers.
type Screen struct {
	m.StatefulWidget
	req student.WorksheetRequest
}

// New returns the worksheet screen for the request. It is the function that
// goes into student.Screens.Worksheet.
func New(req student.WorksheetRequest, opts ...m.WidgetOpts) *Screen {
	return &Screen{StatefulWidget: m.NewStatefulWidgetBase(opts...), req: req}
}

// Builder returns student.Screens.Worksheet.
func Builder() func(student.WorksheetRequest) m.Widget {
	return func(req student.WorksheetRequest) m.Widget { return New(req) }
}

// CreateState creates the state.
func (w *Screen) CreateState() m.State { return &screenState{} }

type screenState struct {
	m.StateOf[*Screen]
	started    bool
	generation atomic.Int64
	sheet      *sheet
	failed     bool
	selected   map[string]bool
	audio      *promptAudio
	playing    string
	leaving    bool
}

// DidChangeDependencies starts the first load (the Scope cannot be read in
// InitState).
func (s *screenState) DidChangeDependencies() {
	if s.started {
		return
	}
	s.started = true
	env := app.ScopeOf(s.Context()).Env()
	req := s.Widget().req
	s.audio = newPromptAudio(env, req.FS)
	s.Go(func(ctx context.Context) { s.load(ctx, env, req) })
}

// Dispose stops the prompt audio.
func (s *screenState) Dispose() {
	if s.audio != nil {
		s.audio.Close()
	}
}

// load reads the worksheet; it runs on a goroutine.
func (s *screenState) load(ctx context.Context, env *app.Env, req student.WorksheetRequest) {
	gen := s.generation.Add(1)
	sh, err := loadSheet(ctx, env, req)
	if err != nil && ctx.Err() == nil {
		env.Log.Error("opening a worksheet failed", "username", req.Username, "cassette", req.Cassette.Manifest.ID, "err", err)
	}
	s.SetState(func() {
		if s.generation.Load() != gen {
			return
		}
		s.sheet, s.failed = sh, err != nil
		s.selected = map[string]bool{}
		if sh == nil {
			return
		}
		n := 0
		for _, ex := range sh.Exercises {
			if ex.State == progress.StateAvailable && n < submission.MaxExercises {
				s.selected[ex.ID] = true
				n++
			}
		}
	})
}

// chosen returns the ids of the selected exercises in the order of the
// worksheet.
func (s *screenState) chosen() []string {
	var ids []string
	if s.sheet == nil {
		return nil
	}
	for _, ex := range s.sheet.Exercises {
		if s.selected[ex.ID] {
			ids = append(ids, ex.ID)
		}
	}
	return ids
}

// toggle ticks or unticks an exercise, within the limit of one submission.
func (s *screenState) toggle(id string, on bool) {
	scope := app.ScopeOf(s.Context())
	if on && len(s.chosen()) >= submission.MaxExercises {
		showMessage(s.Context(), scope.T("worksheet.too_many", "max", strconv.Itoa(submission.MaxExercises)))
		return
	}
	s.SetState(func() { s.selected[id] = on })
}

// check opens the submission flow for the selected exercises and reloads the
// worksheet when it closes.
func (s *screenState) check() {
	ids := s.chosen()
	if len(ids) == 0 {
		return
	}
	req := s.Widget().req
	env := app.ScopeOf(s.Context()).Env()
	s.audio.Stop()
	s.playing = ""
	ch := app.Push(s.Context(), func(m.BuildContext) m.Widget {
		return newFlow(flowRequest{Username: req.Username, Cassette: req.Cassette, ExerciseIDs: ids})
	})
	s.Go(func(ctx context.Context) {
		select {
		case <-ch:
			s.load(ctx, env, req)
		case <-ctx.Done():
		}
	})
}

// speak plays the prompt audio of an exercise, or stops it when it plays.
func (s *screenState) speak(id string) {
	env := app.ScopeOf(s.Context()).Env()
	scope := app.ScopeOf(s.Context())
	s.Go(func(ctx context.Context) {
		now, err := s.audio.Toggle(ctx, id, func(playing string) {
			s.SetState(func() { s.playing = playing })
		})
		if err != nil && ctx.Err() == nil {
			env.Log.Error("playing the prompt audio failed", "exercise", id, "err", err)
			s.SetState(func() {
				s.playing = ""
				showMessage(s.Context(), scope.T("worksheet.no_sound"))
			})
			return
		}
		s.SetState(func() { s.playing = now })
	})
}

func (s *screenState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	title := ""
	var body m.Widget
	switch {
	case s.failed:
		body = pageBody(maxWidth, []m.Widget{
			m.NewText(scope.T("worksheet.load_failed"), m.TextOpts{Key: m.NewValueKey(KeyLoadFailed), Style: m.ThemeOf(bc).TextTheme.BodyLarge}),
		})
	case s.sheet == nil:
		body = m.NewCenter(m.NewCircularProgressIndicator())
	default:
		title = scope.T("worksheet.title", "n", strconv.Itoa(s.Widget().req.Worksheet.Index+1))
		body = s.content(bc, scope)
	}
	return m.NewScaffold(m.ScaffoldOpts{
		AppBar: m.NewAppBar(m.AppBarOpts{
			Title:   m.NewText(title),
			Actions: []m.Widget{logoutButton(scope, bc)},
		}),
		Body: body,
	})
}

// content is the scrolling list and the bar with Check my answers, or with
// Next when the worksheet is finished.
func (s *screenState) content(bc m.BuildContext, scope *app.Scope) m.Widget {
	theme := m.ThemeOf(bc)
	children := []m.Widget{banner(bc, scope), gap(app.Gap)}
	for _, ex := range s.sheet.Exercises {
		children = append(children, s.card(bc, scope, ex), gap(12))
	}
	var summary string
	var action m.Widget
	if s.finished() {
		label := scope.T("worksheet.next")
		if s.Widget().req.OpenNext == nil {
			label = scope.T("worksheet.done")
		}
		summary = scope.T("worksheet.finished")
		action = m.NewFilledButton(s.forward, m.NewText(label), m.FilledButtonOpts{
			Key:   m.NewValueKey(KeyNext),
			Style: m.FilledButtonStyleFrom(m.ButtonStyleFromOpts{MinimumSize: m.Some(m.Size{Width: app.ButtonMinWidth, Height: 72})}),
		})
	} else {
		n := len(s.chosen())
		summary = scope.T("worksheet.selected", "n", strconv.Itoa(n))
		if n == 0 {
			summary = scope.T("worksheet.none_selected")
		}
		action = m.NewFilledButtonIcon(s.onCheck(n), m.NewText(scope.T("worksheet.check")), m.FilledButtonIconOpts{
			Icon:  m.NewIcon(m.Icons.PhotoCamera()),
			Key:   m.NewValueKey(KeyCheck),
			Style: m.FilledButtonStyleFrom(m.ButtonStyleFromOpts{MinimumSize: m.Some(m.Size{Width: app.ButtonMinWidth, Height: 72})}),
		})
	}
	bar := m.NewMaterial(m.NewPadding(m.NewEdgeInsetsSymmetric(12, app.PagePadding), m.NewRow([]m.Widget{
		m.NewExpanded(m.NewText(summary, m.TextOpts{Style: theme.TextTheme.TitleMedium})),
		hgap(app.Gap),
		action,
	}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter})), m.MaterialOpts{Elevation: 8, Color: m.Some(theme.ColorScheme.SurfaceContainer())})
	return m.NewColumn([]m.Widget{
		m.NewExpanded(pageBody(maxWidth, children)),
		bar,
	})
}

// onCheck is the handler of Check my answers; nil (a disabled button) when
// nothing is selected.
func (s *screenState) onCheck(selected int) func() {
	if selected == 0 {
		return nil
	}
	return s.check
}

// finished reports whether every exercise of the loaded worksheet is passed or
// waits for a parent (SPEC §7.3 and §9): the item after the worksheet is
// unlocked then.
func (s *screenState) finished() bool {
	if s.sheet == nil || len(s.sheet.Exercises) == 0 {
		return false
	}
	for _, ex := range s.sheet.Exercises {
		if ex.State != progress.StatePassed && ex.State != progress.StateNeedsReview {
			return false
		}
	}
	return true
}

// forward is the handler of Next and Done: it opens the item that follows the
// worksheet or, when the worksheet ends the cassette, goes back to the cassette
// screen. It stops the prompt audio, and a second press does nothing.
func (s *screenState) forward() {
	if s.leaving {
		return
	}
	s.leaving = true
	s.audio.Stop()
	s.playing = ""
	if next := s.Widget().req.OpenNext; next != nil {
		next(s.Context())
		return
	}
	app.Pop(s.Context())
}

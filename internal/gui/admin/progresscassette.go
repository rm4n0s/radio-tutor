package admin

import (
	"context"
	"slices"
	"sync/atomic"
	"time"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// cassettePage is the page of one student's progress in one cassette (SPEC
// §12.3, §7.7): the exercises that wait for the admin, the switches, the items
// with their states and the override buttons, the submissions and Reset all
// progress. For a cassette that is not installed it only shows what the save
// file holds.
type cassettePage struct {
	m.StatefulWidget
	person     profile.Profile
	cassetteID string
}

func newCassettePage(p profile.Profile, id string) *cassettePage {
	return &cassettePage{StatefulWidget: m.NewStatefulWidgetBase(), person: p, cassetteID: id}
}

// CreateState creates the state.
func (w *cassettePage) CreateState() m.State { return &cassettePageState{} }

type cassettePageState struct {
	m.StateOf[*cassettePage]
	started    bool
	generation atomic.Int64
	data       *reviewData
	failed     bool
}

// DidChangeDependencies starts the first load.
func (s *cassettePageState) DidChangeDependencies() {
	if s.started {
		return
	}
	s.started = true
	s.reload()
}

// reload reads the save file and the cassette on a goroutine.
func (s *cassettePageState) reload() {
	env := app.ScopeOf(s.Context()).Env()
	w := s.Widget()
	s.Go(func(ctx context.Context) {
		gen := s.generation.Add(1)
		d, err := loadReview(ctx, env, w.person.Username, w.cassetteID)
		if err != nil && ctx.Err() == nil {
			env.Log.Error("reading the progress failed", "username", w.person.Username, "cassette", w.cassetteID, "err", err)
		}
		s.SetState(func() {
			if s.generation.Load() == gen {
				s.data, s.failed = d, err != nil
			}
		})
	})
}

// act is the function the override buttons call: it applies the override as the
// logged-in admin and shows the new states.
func (s *cassettePageState) act(scope *app.Scope) func(progress.Override) {
	if s.data == nil || s.data.Cassette == nil {
		return nil
	}
	data, user := s.data, s.Widget().person.Username
	return func(o progress.Override) { runOverride(s, scope, data, user, o, s.reload) }
}

// runOverride applies the override on a goroutine, with the admin's username and
// the time of now, and calls done on the UI thread afterwards (a failure is
// shown in a snack bar first).
func runOverride(host dialogHost, scope *app.Scope, data *reviewData, username string, o progress.Override, done func()) {
	env := scope.Env()
	o.By, o.At = scope.Session().Username, time.Now()
	host.Go(func(ctx context.Context) {
		err := applyOverride(ctx, env, data.Cassette, username, o)
		if err != nil && ctx.Err() == nil {
			env.Log.Error("an override failed", "username", username, "cassette", data.ID, "action", string(o.Action), "err", err)
		}
		host.SetState(func() {
			if err != nil && ctx.Err() == nil {
				showMessage(host.Context(), scope.Catalog().Msg(errMessage(err)))
			}
			done()
		})
	})
}

// section is the title of a group on the page.
func section(bc m.BuildContext, key, text string) m.Widget {
	style := m.ThemeOf(bc).TextTheme.TitleLarge.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600})
	var k m.Key
	if key != "" {
		k = m.NewValueKey(key)
	}
	return m.NewPadding(m.NewEdgeInsetsSymmetric(6, 0), m.NewText(text, m.TextOpts{Key: k, Style: style}))
}

func (s *cassettePageState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	w := s.Widget()
	title := w.cassetteID
	var children []m.Widget
	switch {
	case s.failed:
		children = []m.Widget{errorText(bc, scope.T("admin.progress.load_failed"))}
	case s.data == nil:
		children = []m.Widget{m.NewCenter(m.NewCircularProgressIndicator())}
	default:
		title = s.data.Title
		children = s.content(bc, scope)
	}
	return m.NewScaffold(m.ScaffoldOpts{
		AppBar: appBar(scope, bc, title),
		Body:   pageBody(pageWidth, children),
	})
}

// content is the whole page once the data is there.
func (s *cassettePageState) content(bc m.BuildContext, scope *app.Scope) []m.Widget {
	d := s.data
	person := s.Widget().person
	act := s.act(scope)
	children := []m.Widget{
		headline(bc, d.Title),
		gap(2),
		muted(bc, scope.T("admin.progress.of_student", "name", person.DisplayName)),
		gap(app.Gap),
	}
	if d.Cassette == nil {
		key := "admin.progress.read_only"
		if !d.NotInstalled {
			key = "admin.progress.read_only_damaged"
		}
		children = append(children, m.NewText(scope.T(key), m.TextOpts{
			Key: m.NewValueKey(KeyReadOnly), Style: m.ThemeOf(bc).TextTheme.BodyLarge.CopyWith(m.TextStyleCopy{Color: m.Some(m.ThemeOf(bc).ColorScheme.Error())}),
		}), gap(app.Gap))
	}
	items := s.itemViews()
	if waiting := filterWaiting(items); len(waiting) > 0 {
		children = append(children, section(bc, KeyWaitingSection, scope.T("admin.progress.waiting", "n", itoa(len(waiting)))), gap(8))
		for _, v := range waiting {
			children = append(children, itemCard(bc, scope, SectionWaiting, v, act), gap(12))
		}
		children = append(children, gap(app.Gap/2))
	}
	children = append(children, s.switches(scope), gap(app.Gap))
	children = append(children, section(bc, "", scope.T("admin.progress.items")), gap(8))
	for _, v := range items {
		children = append(children, itemCard(bc, scope, SectionItems, v, act), gap(12))
	}
	children = append(children, gap(app.Gap/2), section(bc, "", scope.T("admin.progress.submissions")), gap(8))
	children = append(children, s.submissions(bc, scope)...)
	children = append(children, gap(app.Gap), m.NewAlign(m.NewOutlinedButtonIcon(s.confirmReset(scope), m.NewText(scope.T("admin.progress.reset_all")), m.OutlinedButtonIconOpts{
		Icon: m.NewIcon(m.Icons.RestartAlt()), Key: m.NewValueKey(KeyResetAll), Style: smallStyle(),
	}), m.AlignOpts{Alignment: m.AlignmentCenterLeft}))
	return children
}

// itemViews are the cards of the page in the order of the cassette; for a
// cassette that cannot be read, the entries of the save file by id.
func (s *cassettePageState) itemViews() []itemView {
	d := s.data
	p := d.Progress
	var out []itemView
	if d.Cassette == nil {
		for _, id := range sortedKeys(p.Lessons) {
			out = append(out, itemView{id: id, title: id, lesson: true, lp: p.Lessons[id]})
		}
		for _, id := range sortedKeys(p.Exercises) {
			ep := p.Exercises[id]
			out = append(out, itemView{id: id, label: id, title: id, ep: ep, state: ep.State})
		}
		return out
	}
	for _, it := range d.Cassette.Manifest.Items {
		switch it.Type {
		case cassette.ItemLesson:
			title := it.ID
			if le := d.Cassette.Lessons[it.ID]; le != nil {
				title = le.Title
			}
			out = append(out, itemView{id: it.ID, title: title, lesson: true, state: d.States[it.ID], lp: p.Lessons[it.ID]})
		case cassette.ItemExercise:
			v := itemView{id: it.ID, label: it.ID, title: it.ID, state: d.States[it.ID], ep: p.Exercises[it.ID]}
			if ex := d.Cassette.Exercises[it.ID]; ex != nil {
				v.label, v.title, v.maxAttempts = ex.Label, ex.Title, ex.MaxAttempts
			}
			out = append(out, v)
		}
	}
	return out
}

// sortedKeys returns the keys of the map in order.
func sortedKeys[V any](mp map[string]V) []string {
	keys := make([]string, 0, len(mp))
	for k := range mp {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// filterWaiting keeps the exercises that wait for the admin.
func filterWaiting(items []itemView) []itemView {
	var out []itemView
	for _, v := range items {
		if !v.lesson && v.state == progress.StateNeedsReview {
			out = append(out, v)
		}
	}
	return out
}

// switches are the two per-cassette switches; a change is saved at once.
func (s *cassettePageState) switches(scope *app.Scope) m.Widget {
	d := s.data
	change := func(set func(*progress.Settings, bool)) func(bool) {
		if d.Cassette == nil {
			return nil
		}
		return func(v bool) { s.saveSettings(scope, func(st *progress.Settings) { set(st, v) }) }
	}
	return m.NewColumn([]m.Widget{
		m.NewSwitchListTile(d.Progress.Settings.SkipLessons, change(func(st *progress.Settings, v bool) { st.SkipLessons = v }), m.SwitchListTileOpts{
			Key:            m.NewValueKey(KeySkipLessons),
			Title:          m.NewText(scope.T("admin.progress.skip_lessons")),
			Subtitle:       m.NewText(scope.T("admin.progress.skip_lessons_hint")),
			ContentPadding: m.NewEdgeInsetsAll(0),
		}),
		m.NewSwitchListTile(d.Progress.Settings.FreeNavigation, change(func(st *progress.Settings, v bool) { st.FreeNavigation = v }), m.SwitchListTileOpts{
			Key:            m.NewValueKey(KeyFreeNavigation),
			Title:          m.NewText(scope.T("admin.progress.free_navigation")),
			Subtitle:       m.NewText(scope.T("admin.progress.free_navigation_hint")),
			ContentPadding: m.NewEdgeInsetsAll(0),
		}),
	}, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStretch})
}

// saveSettings shows the new switch at once and saves it on a goroutine; the
// page is read again afterwards, which also undoes the change if saving failed.
func (s *cassettePageState) saveSettings(scope *app.Scope, change func(*progress.Settings)) {
	env := scope.Env()
	w := s.Widget()
	s.SetState(func() { change(&s.data.Progress.Settings) })
	s.Go(func(ctx context.Context) {
		err := setSettings(ctx, env, w.person.Username, w.cassetteID, change)
		if err != nil && ctx.Err() == nil {
			env.Log.Error("saving the switches failed", "username", w.person.Username, "cassette", w.cassetteID, "err", err)
		}
		s.SetState(func() {
			if err != nil && ctx.Err() == nil {
				showMessage(s.Context(), scope.Catalog().Msg(errMessage(err)))
			}
			s.reload()
		})
	})
}

// confirmReset returns the function of the Reset all progress button, which is
// nil (a disabled button) for a cassette that cannot be read.
func (s *cassettePageState) confirmReset(scope *app.Scope) func() {
	if s.data.Cassette == nil {
		return nil
	}
	env := scope.Env()
	w := s.Widget()
	return func() {
		build := func(m.BuildContext) m.Widget {
			return newResetDialog(scope.T("admin.progress.reset_all_title", "name", w.person.DisplayName, "cassette", s.data.Title))
		}
		showDialog(s, build, func(a resetAnswer) {
			if !a.confirmed {
				return
			}
			by := scope.Session().Username
			s.Go(func(ctx context.Context) {
				err := resetAll(ctx, env, w.person.Username, w.cassetteID, by, time.Now(), a.deletePhotos)
				if err != nil && ctx.Err() == nil {
					env.Log.Error("resetting the progress failed", "username", w.person.Username, "cassette", w.cassetteID, "err", err)
				}
				s.SetState(func() {
					switch {
					case err == nil:
						showMessage(s.Context(), scope.T("admin.progress.reset_all_done"))
					case ctx.Err() == nil:
						showMessage(s.Context(), scope.Catalog().Msg(errMessage(err)))
					}
					s.reload()
				})
			})
		})
	}
}

// resetAnswer is what the reset dialog answers.
type resetAnswer struct {
	// confirmed is true when the admin chose Reset.
	confirmed bool
	// deletePhotos is the checkbox: delete the photos of the cassette too.
	deletePhotos bool
}

// resetDialog is the confirmation of Reset all progress with the unchecked
// checkbox that also deletes the photos.
type resetDialog struct {
	m.StatefulWidget
	title string
}

func newResetDialog(title string) *resetDialog {
	return &resetDialog{StatefulWidget: m.NewStatefulWidgetBase(), title: title}
}

// CreateState creates the state.
func (w *resetDialog) CreateState() m.State { return &resetDialogState{} }

type resetDialogState struct {
	m.StateOf[*resetDialog]
	deletePhotos bool
}

func (s *resetDialogState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	return m.NewAlertDialog(m.AlertDialogOpts{
		Title: m.NewText(s.Widget().title),
		Content: m.NewSizedBox(m.NewColumn([]m.Widget{
			body(bc, scope.T("admin.progress.reset_all_body")),
			gap(app.Gap),
			m.NewCheckboxListTile(m.Some(s.deletePhotos), func(v m.Opt[bool]) {
				s.SetState(func() { s.deletePhotos = v.OrElse(false) })
			}, m.CheckboxListTileOpts{
				Key:             m.NewValueKey(KeyAlsoPhotos),
				Title:           m.NewText(scope.T("admin.progress.reset_all_photos")),
				ControlAffinity: m.ListTileControlAffinityLeading,
			}),
		}, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStretch, MainAxisSize: m.MainAxisSizeMin}), m.SizedBoxOpts{Width: m.Some(460.0)}),
		Scrollable: true,
		Actions: []m.Widget{
			dialogButton(KeyDialogCancel, scope.T("admin.cancel"), resetAnswer{}, false),
			m.NewBuilder(func(bc m.BuildContext) m.Widget {
				return m.NewFilledButton(func() { m.NavigatorPop(bc, resetAnswer{confirmed: true, deletePhotos: s.deletePhotos}) },
					m.NewText(scope.T("admin.progress.reset_all")), m.FilledButtonOpts{Key: m.NewValueKey(KeyDialogOK)})
			}),
		},
	})
}

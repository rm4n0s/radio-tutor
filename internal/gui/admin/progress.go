package admin

import (
	"context"
	"sync/atomic"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// Keys of the progress screens that tests and later steps look for.
const (
	// KeyProgressStudentPrefix starts the key of a student's row on the first
	// progress screen; the username follows.
	KeyProgressStudentPrefix = "progress-student-"
	// KeyProgressCassettePrefix starts the key of a cassette's row on the
	// student's page; the cassette id follows.
	KeyProgressCassettePrefix = "progress-cassette-"
	// KeyDeleteStudentPhotos is the key of the "Delete all photos of this
	// student" button.
	KeyDeleteStudentPhotos = "delete-student-photos"
	// KeyWaitingSection is the key of the "Waiting for you" title of a
	// cassette's page.
	KeyWaitingSection = "waiting-section"
	// KeySkipLessons is the key of the skip lessons switch.
	KeySkipLessons = "switch-skip-lessons"
	// KeyFreeNavigation is the key of the free navigation switch.
	KeyFreeNavigation = "switch-free-navigation"
	// KeyResetAll is the key of the "Reset all progress" button.
	KeyResetAll = "reset-all-progress"
	// KeyAlsoPhotos is the key of the checkbox "Also delete the photos" of the
	// reset dialog.
	KeyAlsoPhotos = "also-delete-photos"
	// KeySubmissionPrefix starts the key of a submission's row; the submission
	// id follows.
	KeySubmissionPrefix = "submission-"
	// KeyReadOnly is the key of the note on the page of a cassette that is not
	// installed.
	KeyReadOnly = "progress-read-only"
	// KeyLoadFailed is the key of the text that says the progress could not be
	// read.
	KeyLoadFailed = "progress-load-failed"
)

// Progress is the progress and review screen of the admin (SPEC §12.3). It lists
// the students; a student opens the page with their cassettes, and a cassette
// the page with the items, the switches, the overrides and the submissions.
type Progress struct{ m.StatefulWidget }

// NewProgress returns the progress screen.
func NewProgress(opts ...m.WidgetOpts) *Progress {
	return &Progress{StatefulWidget: m.NewStatefulWidgetBase(opts...)}
}

// CreateState creates the state.
func (w *Progress) CreateState() m.State { return &progressState{} }

type progressState struct {
	m.StateOf[*Progress]
	started    bool
	generation atomic.Int64
	waiting    waitingCounts
}

// DidChangeDependencies starts the first load (the Scope cannot be read in
// InitState).
func (s *progressState) DidChangeDependencies() {
	if s.started {
		return
	}
	s.started = true
	env := app.ScopeOf(s.Context()).Env()
	s.Go(func(ctx context.Context) { s.load(ctx, env) })
}

// load counts the exercises that wait, for the badges; it runs on a goroutine.
func (s *progressState) load(ctx context.Context, env *app.Env) {
	gen := s.generation.Add(1)
	counts, err := scanWaiting(ctx, env, nil)
	if err != nil {
		if ctx.Err() == nil {
			env.Log.Error("counting the exercises to review failed", "err", err)
		}
		return
	}
	s.SetState(func() {
		if s.generation.Load() == gen {
			s.waiting = counts
		}
	})
}

func (s *progressState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	env := scope.Env()
	var children []m.Widget
	if n := s.waiting.total(); n > 0 {
		children = append(children, m.NewAlign(m.NewFilledButtonIcon(func() {
			openScreen(s, bc, func(m.BuildContext) m.Widget { return NewReviewQueue() }, func(ctx context.Context) { s.load(ctx, env) })
		}, m.NewText(scope.T("admin.review.open", "n", itoa(n))), m.FilledButtonIconOpts{
			Icon: m.NewIcon(m.Icons.Flag()), Key: m.NewValueKey(KeyOpenReview), Style: bigAction(),
		}), m.AlignOpts{Alignment: m.AlignmentCenterLeft}), gap(app.Gap))
	}
	children = append(children, body(bc, scope.T("admin.progress.pick_student")), gap(app.Gap))
	for _, p := range env.Profiles.List() {
		children = append(children, s.row(bc, scope, env, p), gap(12))
	}
	return m.NewScaffold(m.ScaffoldOpts{
		AppBar: appBar(scope, bc, scope.T("admin.progress")),
		Body:   pageBody(pageWidth, children),
	})
}

// row is the card of one person.
func (s *progressState) row(bc m.BuildContext, scope *app.Scope, env *app.Env, p profile.Profile) m.Widget {
	lines := []m.Widget{
		m.NewText(p.DisplayName, m.TextOpts{
			Key:   m.NewValueKey(KeyProgressStudentPrefix + p.Username),
			Style: m.ThemeOf(bc).TextTheme.TitleLarge.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600}),
		}),
		gap(2),
		muted(bc, p.Username),
	}
	trailing := []m.Widget{badge(scope.T("admin.role."+string(p.Role)), roleIcon(p.Role))}
	if n := s.waiting.ofStudent(p.Username); n > 0 {
		trailing = append(trailing, hgap(8), waitingBadge(bc, scope, n))
	}
	trailing = append(trailing, hgap(8), m.NewIcon(m.Icons.ChevronRight()))
	open := func() {
		openScreen(s, bc, func(m.BuildContext) m.Widget { return newProgressStudent(p) }, func(ctx context.Context) { s.load(ctx, env) })
	}
	return m.NewCard(m.NewInkWell(m.NewPadding(m.NewEdgeInsetsAll(16), m.NewRow([]m.Widget{
		m.NewExpanded(m.NewColumn(lines, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart})),
		hgap(8),
		m.NewRow(trailing, m.RowOpts{MainAxisSize: m.MainAxisSizeMin}),
	}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter})), m.InkWellOpts{OnTap: open}),
		m.CardOpts{Margin: m.NewEdgeInsetsAll(0), ClipBehavior: m.ClipAntiAlias})
}

// waitingBadge says how many exercises wait for the admin, in the error color.
func waitingBadge(bc m.BuildContext, scope *app.Scope, n int) m.Widget {
	theme := m.ThemeOf(bc)
	return m.NewRow([]m.Widget{
		m.NewIcon(m.Icons.Flag(), m.IconOpts{Size: m.Some(24.0), Color: m.Some(theme.ColorScheme.Error())}),
		hgap(4),
		m.NewText(scope.T("admin.home.review", "n", itoa(n)), m.TextOpts{
			Style: theme.TextTheme.BodyMedium.CopyWith(m.TextStyleCopy{Color: m.Some(theme.ColorScheme.Error()), FontWeight: m.FontWeightW700}),
		}),
	}, m.RowOpts{MainAxisSize: m.MainAxisSizeMin})
}

// openScreen pushes a screen and runs then on a goroutine when it closes.
func openScreen(host dialogHost, bc m.BuildContext, build m.WidgetBuilder, then func(ctx context.Context)) {
	ch := app.Push(bc, build)
	host.Go(func(ctx context.Context) {
		select {
		case <-ch:
			then(ctx)
		case <-ctx.Done():
		}
	})
}

// progressStudent is the page of one person: the cassettes they have progress in.
type progressStudent struct {
	m.StatefulWidget
	person profile.Profile
}

func newProgressStudent(p profile.Profile) *progressStudent {
	return &progressStudent{StatefulWidget: m.NewStatefulWidgetBase(), person: p}
}

// CreateState creates the state.
func (w *progressStudent) CreateState() m.State { return &progressStudentState{} }

type progressStudentState struct {
	m.StateOf[*progressStudent]
	started    bool
	generation atomic.Int64
	entries    []cassetteEntry
	loaded     bool
	failed     bool
}

// DidChangeDependencies starts the first load.
func (s *progressStudentState) DidChangeDependencies() {
	if s.started {
		return
	}
	s.started = true
	env := app.ScopeOf(s.Context()).Env()
	s.Go(func(ctx context.Context) { s.load(ctx, env) })
}

// load lists the cassettes on a goroutine.
func (s *progressStudentState) load(ctx context.Context, env *app.Env) {
	gen := s.generation.Add(1)
	username := s.Widget().person.Username
	counts, err := scanWaiting(ctx, env, []string{username})
	var entries []cassetteEntry
	if err == nil {
		entries, err = listEntries(ctx, env, username, counts[username])
	}
	if err != nil && ctx.Err() == nil {
		env.Log.Error("listing the cassettes of a student failed", "username", username, "err", err)
	}
	s.SetState(func() {
		if s.generation.Load() == gen {
			s.entries, s.loaded, s.failed = entries, true, err != nil
		}
	})
}

func (s *progressStudentState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	p := s.Widget().person
	children := []m.Widget{headline(bc, p.DisplayName), gap(4), body(bc, scope.T("admin.progress.pick_cassette")), gap(app.Gap)}
	switch {
	case !s.loaded:
		children = append(children, m.NewCenter(m.NewCircularProgressIndicator()))
	case s.failed:
		children = append(children, errorText(bc, scope.T("admin.progress.load_failed")))
	case len(s.entries) == 0:
		children = append(children, body(bc, scope.T("admin.progress.no_cassettes")))
	default:
		for _, e := range s.entries {
			children = append(children, s.row(bc, scope, e), gap(12))
		}
	}
	children = append(children, gap(app.Gap), m.NewAlign(m.NewOutlinedButtonIcon(func() { s.deletePhotos(scope) },
		m.NewText(scope.T("admin.progress.delete_student_photos")), m.OutlinedButtonIconOpts{
			Icon: m.NewIcon(m.Icons.DeleteForever()), Key: m.NewValueKey(KeyDeleteStudentPhotos), Style: smallStyle(),
		}), m.AlignOpts{Alignment: m.AlignmentCenterLeft}))
	return m.NewScaffold(m.ScaffoldOpts{
		AppBar: appBar(scope, bc, scope.T("admin.progress.student_title", "name", p.DisplayName)),
		Body:   pageBody(pageWidth, children),
	})
}

// row is the card of one cassette.
func (s *progressStudentState) row(bc m.BuildContext, scope *app.Scope, e cassetteEntry) m.Widget {
	env := scope.Env()
	lines := []m.Widget{bold(bc, e.Title), gap(2), muted(bc, e.ID)}
	var trailing []m.Widget
	switch {
	case !e.Installed:
		trailing = append(trailing, badge(scope.T("admin.progress.not_installed"), m.Icons.CloudOff()), hgap(8))
	case e.Waiting > 0:
		trailing = append(trailing, waitingBadge(bc, scope, e.Waiting), hgap(8))
	}
	trailing = append(trailing, m.NewIcon(m.Icons.ChevronRight()))
	person := s.Widget().person
	open := func() {
		openScreen(s, bc, func(m.BuildContext) m.Widget { return newCassettePage(person, e.ID) }, func(ctx context.Context) { s.load(ctx, env) })
	}
	return m.NewCard(m.NewInkWell(m.NewPadding(m.NewEdgeInsetsAll(16), m.NewRow([]m.Widget{
		m.NewIcon(m.Icons.MenuBook(), m.IconOpts{Size: m.Some(32.0)}),
		hgap(app.Gap),
		m.NewExpanded(m.NewColumn(lines, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart, Key: m.NewValueKey(KeyProgressCassettePrefix + e.ID)})),
		hgap(8),
		m.NewRow(trailing, m.RowOpts{MainAxisSize: m.MainAxisSizeMin}),
	}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter})), m.InkWellOpts{OnTap: open}),
		m.CardOpts{Margin: m.NewEdgeInsetsAll(0), ClipBehavior: m.ClipAntiAlias})
}

// deletePhotos asks for confirmation, then deletes all the photos of the
// student (SPEC §11.7).
func (s *progressStudentState) deletePhotos(scope *app.Scope) {
	env := scope.Env()
	p := s.Widget().person
	build := func(m.BuildContext) m.Widget {
		return confirmDialog(scope, scope.T("admin.progress.delete_student_photos_title", "name", p.DisplayName),
			scope.T("admin.progress.delete_student_photos_body", "name", p.DisplayName), scope.T("admin.progress.delete_photos"))
	}
	showDialog(s, build, func(yes bool) {
		if !yes {
			return
		}
		s.Go(func(ctx context.Context) {
			err := deleteStudentPhotos(ctx, env, p.Username)
			if err != nil && ctx.Err() == nil {
				env.Log.Error("deleting the photos of a student failed", "username", p.Username, "err", err)
			}
			s.SetState(func() {
				switch {
				case err == nil:
					showMessage(s.Context(), scope.T("admin.progress.photos_deleted"))
				case ctx.Err() == nil:
					showMessage(s.Context(), scope.Catalog().Msg(errMessage(err)))
				}
			})
		})
	})
}

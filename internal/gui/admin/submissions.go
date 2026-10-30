package admin

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/providers"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// Keys of the submission screens that tests and later steps look for.
const (
	// KeyPhotoPrefix starts the key of a photo's thumbnail; its number (from 1)
	// follows.
	KeyPhotoPrefix = "photo-"
	// KeyPhotoViewer is the key of the full-size photo view.
	KeyPhotoViewer = "photo-viewer"
	// KeyPhotoZoom is the key of the zoomable area of the full-size photo view.
	KeyPhotoZoom = "photo-zoom"
	// KeyPageNotes is the key of the text with the grader's notes on the page.
	KeyPageNotes = "page-notes"
	// KeyDeletePhotos is the key of the "Delete photos" button of a submission.
	KeyDeletePhotos = "delete-photos"
	// KeyResultPrefix starts the key of the card of one exercise's result in a
	// submission; the exercise id follows.
	KeyResultPrefix = "sub-result-"
	// KeyVerdictPrefix starts the key of the verdict mark of a result; the
	// exercise id follows.
	KeyVerdictPrefix = "sub-verdict-"
	// KeyTranscriptionPrefix starts the key of the transcription of a result.
	KeyTranscriptionPrefix = "sub-transcription-"
	// KeyFeedbackPrefix starts the key of the feedback for the child of a result.
	KeyFeedbackPrefix = "sub-feedback-"
	// KeyParentNotesPrefix starts the key of the notes for the parent of a result.
	KeyParentNotesPrefix = "sub-notes-"
	// KeyNoPhotos is the key of the text that says a submission has no photos.
	KeyNoPhotos = "no-photos"
	// KeyProviderLine is the key of the row with the provider's name and kind.
	KeyProviderLine = "submission-provider"
)

// photoPath returns the file of the photo of the submission, or "" when the
// name in the save file cannot be a file name.
func photoPath(env *app.Env, username, cassetteID, submissionID, name string) string {
	dir, err := photoFolder(env, username, cassetteID, submissionID)
	if err != nil || !safeElement(name) {
		return ""
	}
	return filepath.Join(dir, name)
}

// labels are the labels of the exercises of a submission, joined.
func (d *reviewData) labels(ids []string) string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		label := id
		if d.Cassette != nil {
			if ex := d.Cassette.Exercises[id]; ex != nil {
				label = ex.Label
			}
		}
		out = append(out, label)
	}
	return strings.Join(out, ", ")
}

// statusColor is the color of a status: failed in the error color.
func statusColor(theme *m.ThemeData, st progress.SubmissionStatus) m.Color {
	switch st {
	case progress.SubmissionFailed:
		return theme.ColorScheme.Error()
	case progress.SubmissionGraded:
		return theme.ColorScheme.Primary()
	}
	return theme.ColorScheme.OnSurfaceVariant()
}

// statusText is the words of a status in its color.
func statusText(bc m.BuildContext, scope *app.Scope, st progress.SubmissionStatus) m.Widget {
	theme := m.ThemeOf(bc)
	return m.NewText(scope.T("admin.progress.status."+string(st)), m.TextOpts{
		Style: theme.TextTheme.TitleMedium.CopyWith(m.TextStyleCopy{Color: m.Some(statusColor(theme, st)), FontWeight: m.FontWeightW600}),
	})
}

// providerLine is the name of the provider that graded with a badge for its kind
// (SPEC §11.7). A provider that was deleted since is shown as removed.
func providerLine(bc m.BuildContext, scope *app.Scope, name string) m.Widget {
	var cfg providers.Config
	var found bool
	if store := scope.Env().Providers; store != nil {
		cfg, found = store.Get(name)
	}
	kind := badge(scope.T("admin.progress.provider_unknown"), m.Icons.HelpOutline())
	if found {
		icon := m.Icons.Cloud()
		if cfg.Kind == providers.KindLocal {
			icon = m.Icons.Computer()
		}
		kind = badge(scope.T("admin.provider.kind."+string(cfg.Kind)), icon)
	}
	return m.NewRow([]m.Widget{
		m.NewFlexible(m.NewText(name, m.TextOpts{Style: m.ThemeOf(bc).TextTheme.BodyLarge, MaxLines: m.Some(1), Overflow: m.TextOverflowEllipsis})),
		hgap(8),
		kind,
	}, m.RowOpts{Key: m.NewValueKey(KeyProviderLine), MainAxisSize: m.MainAxisSizeMin})
}

// submissions is the list of the submissions of the page, newest first.
func (s *cassettePageState) submissions(bc m.BuildContext, scope *app.Scope) []m.Widget {
	d := s.data
	if len(d.Progress.Submissions) == 0 {
		return []m.Widget{muted(bc, scope.T("admin.progress.no_submissions"))}
	}
	var out []m.Widget
	subs := slices.Clone(d.Progress.Submissions)
	slices.SortStableFunc(subs, func(a, b progress.Submission) int { return b.CreatedAt.Compare(a.CreatedAt) })
	for _, sub := range subs {
		out = append(out, s.submissionRow(bc, scope, sub), gap(12))
	}
	return out
}

// submissionRow is the card of one submission: date, exercises, provider and
// kind, and status.
func (s *cassettePageState) submissionRow(bc m.BuildContext, scope *app.Scope, sub progress.Submission) m.Widget {
	d := s.data
	person := s.Widget().person
	lines := []m.Widget{
		m.NewRow([]m.Widget{
			m.NewExpanded(bold(bc, formatTime(sub.CreatedAt))),
			hgap(8),
			statusText(bc, scope, sub.Status),
		}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter}),
		gap(2),
		muted(bc, scope.T("admin.progress.submission_exercises", "labels", d.labels(sub.ExerciseIDs))),
	}
	if sub.Provider != "" {
		lines = append(lines, gap(6), providerLine(bc, scope, sub.Provider))
	}
	id := d.ID
	open := func() {
		openScreen(s, bc, func(m.BuildContext) m.Widget { return newSubmissionPage(person, id, sub.ID) }, func(context.Context) { s.reload() })
	}
	return m.NewCard(m.NewInkWell(m.NewPadding(m.NewEdgeInsetsAll(app.Gap), m.NewColumn(lines, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStretch})),
		m.InkWellOpts{Key: m.NewValueKey(KeySubmissionPrefix + sub.ID), OnTap: open}),
		m.CardOpts{Margin: m.NewEdgeInsetsAll(0), ClipBehavior: m.ClipAntiAlias})
}

// submissionPage is the screen of one submission: its photos, the page notes and
// the result of each exercise with the override buttons.
type submissionPage struct {
	m.StatefulWidget
	person       profile.Profile
	cassetteID   string
	submissionID string
}

func newSubmissionPage(p profile.Profile, cassetteID, submissionID string) *submissionPage {
	return &submissionPage{StatefulWidget: m.NewStatefulWidgetBase(), person: p, cassetteID: cassetteID, submissionID: submissionID}
}

// CreateState creates the state.
func (w *submissionPage) CreateState() m.State { return &submissionPageState{} }

type submissionPageState struct {
	m.StateOf[*submissionPage]
	started    bool
	generation atomic.Int64
	data       *reviewData
	failed     bool
}

// DidChangeDependencies starts the first load.
func (s *submissionPageState) DidChangeDependencies() {
	if s.started {
		return
	}
	s.started = true
	s.reload()
}

// reload reads the save file and the cassette on a goroutine.
func (s *submissionPageState) reload() {
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

func (s *submissionPageState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	var sub *progress.Submission
	if s.data != nil {
		sub = s.data.Progress.Submission(s.Widget().submissionID)
	}
	var title string
	var children []m.Widget
	switch {
	case s.failed || (s.data != nil && sub == nil):
		children = []m.Widget{errorText(bc, scope.T("admin.progress.load_failed"))}
	case s.data == nil:
		children = []m.Widget{m.NewCenter(m.NewCircularProgressIndicator())}
	default:
		title = scope.T("admin.submission.title", "date", formatTime(sub.CreatedAt))
		children = s.content(bc, scope, sub)
	}
	return m.NewScaffold(m.ScaffoldOpts{
		AppBar: appBar(scope, bc, title),
		Body:   pageBody(pageWidth, children),
	})
}

// content is the whole screen once the data is there.
func (s *submissionPageState) content(bc m.BuildContext, scope *app.Scope, sub *progress.Submission) []m.Widget {
	d := s.data
	head := []m.Widget{
		m.NewRow([]m.Widget{
			statusText(bc, scope, sub.Status),
			hgap(12),
			muted(bc, scope.T("admin.progress.submission_exercises", "labels", d.labels(sub.ExerciseIDs))),
		}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter}),
	}
	if sub.Provider != "" {
		head = append(head, gap(8), providerLine(bc, scope, sub.Provider))
	}
	if sub.Model != "" {
		head = append(head, gap(4), muted(bc, scope.T("admin.submission.model", "model", sub.Model)))
	}
	if sub.Status == progress.SubmissionFailed && sub.Error != "" {
		head = append(head, gap(8), errorLine(bc, scope.T("admin.submission.error", "error", sub.Error)))
	}
	children := slices.Concat(head, []m.Widget{gap(app.Gap * 1.5), section(bc, "", scope.T("admin.submission.photos")), gap(8)})
	children = append(children, s.photos(bc, scope, sub)...)
	if sub.PageNotes != "" {
		children = append(children, gap(app.Gap*1.5), section(bc, "", scope.T("admin.submission.page_notes")), gap(8),
			m.NewText(sub.PageNotes, m.TextOpts{Key: m.NewValueKey(KeyPageNotes), Style: m.ThemeOf(bc).TextTheme.BodyLarge}))
	}
	children = append(children, gap(app.Gap*1.5), section(bc, "", scope.T("admin.submission.results")), gap(8))
	if len(sub.Results) == 0 {
		children = append(children, muted(bc, scope.T("admin.submission.no_results")))
	}
	var act func(progress.Override)
	if d.Cassette != nil {
		person := s.Widget().person
		act = func(o progress.Override) { runOverride(s, scope, d, person.Username, o, s.reload) }
	}
	for _, r := range sub.Results {
		children = append(children, s.resultCard(bc, scope, r, act), gap(12))
	}
	return children
}

// photos are the thumbnails of the submission and its Delete photos button.
func (s *submissionPageState) photos(bc m.BuildContext, scope *app.Scope, sub *progress.Submission) []m.Widget {
	if len(sub.Photos) == 0 {
		return []m.Widget{m.NewText(scope.T("admin.submission.no_photos"), m.TextOpts{
			Key: m.NewValueKey(KeyNoPhotos), Style: m.ThemeOf(bc).TextTheme.BodyMedium.CopyWith(m.TextStyleCopy{Color: m.Some(m.ThemeOf(bc).ColorScheme.OnSurfaceVariant())}),
		})}
	}
	w := s.Widget()
	env := scope.Env()
	var thumbs []m.Widget
	for i, name := range sub.Photos {
		path := photoPath(env, w.person.Username, w.cassetteID, sub.ID, name)
		n := i + 1
		total := len(sub.Photos)
		thumbs = append(thumbs, thumbnail(bc, scope, KeyPhotoPrefix+itoa(n), path, func() {
			app.Push(bc, func(m.BuildContext) m.Widget { return newPhotoViewer(path, n, total) })
		}))
	}
	return []m.Widget{
		m.NewWrap(thumbs, m.WrapOpts{Spacing: 12, RunSpacing: 12}),
		gap(6),
		muted(bc, scope.T("admin.submission.zoom_hint")),
		gap(app.Gap),
		m.NewAlign(m.NewOutlinedButtonIcon(func() { s.deletePhotos(scope, sub.ID) }, m.NewText(scope.T("admin.submission.delete_photos")), m.OutlinedButtonIconOpts{
			Icon: m.NewIcon(m.Icons.DeleteForever()), Key: m.NewValueKey(KeyDeletePhotos), Style: smallStyle(),
		}), m.AlignOpts{Alignment: m.AlignmentCenterLeft}),
	}
}

// Sizes of a photo's thumbnail, in logical pixels, and the width the photo is
// decoded at for it.
const (
	thumbWidth  = 150.0
	thumbHeight = 200.0
	thumbDecode = 300
)

// thumbnail is a tappable small picture of a photo file; a file that is missing
// or unreadable shows a broken-image icon.
func thumbnail(bc m.BuildContext, scope *app.Scope, key, path string, onTap func()) m.Widget {
	theme := m.ThemeOf(bc)
	missing := func(m.BuildContext, error) m.Widget {
		return m.NewColoredBox(theme.ColorScheme.SurfaceContainerHighest(), m.NewCenter(m.NewIcon(m.Icons.BrokenImage(), m.IconOpts{
			Size: m.Some(48.0), Color: m.Some(theme.ColorScheme.OnSurfaceVariant()),
		})))
	}
	picture := m.NewImageFile(path, m.ImageFileOpts{
		CacheWidth: m.Some(thumbDecode),
		ImageOpts:  m.ImageOpts{Fit: m.BoxFitCover, ErrorBuilder: missing},
	})
	return m.NewSizedBox(m.NewClipRRect(m.NewInkWell(picture, m.InkWellOpts{Key: m.NewValueKey(key), OnTap: onTap}),
		m.ClipRRectOpts{BorderRadius: m.NewBorderRadiusCircular(8)}), m.SizedBoxOpts{Width: m.Some(thumbWidth), Height: m.Some(thumbHeight)})
}

// resultCard is the result of one exercise: the verdict mark, the transcription,
// what the child was told, the notes for the parent, the state of the exercise
// now and the override buttons.
func (s *submissionPageState) resultCard(bc m.BuildContext, scope *app.Scope, r progress.Result, act func(progress.Override)) m.Widget {
	d := s.data
	theme := m.ThemeOf(bc)
	v := itemView{id: r.ExerciseID, label: r.ExerciseID, title: r.ExerciseID, state: d.States[r.ExerciseID], ep: d.Progress.Exercises[r.ExerciseID]}
	if d.Cassette != nil {
		if ex := d.Cassette.Exercises[r.ExerciseID]; ex != nil {
			v.label, v.title, v.maxAttempts = ex.Label, ex.Title, ex.MaxAttempts
		}
	}
	head := []m.Widget{
		verdictMark(bc, KeyVerdictPrefix+r.ExerciseID, r.Verdict),
		hgap(app.Gap),
		m.NewExpanded(m.NewColumn([]m.Widget{
			bold(bc, scope.T("admin.progress.exercise_title", "label", v.label, "title", v.title)),
			gap(2),
			muted(bc, scope.T("admin.submission.verdict", "verdict", scope.T("admin.verdict."+string(r.Verdict)))),
		}, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart})),
	}
	if v.state != "" {
		head = append(head, hgap(8), stateBadge(bc, scope, StateKey(SectionSubmission, r.ExerciseID), v.state))
	}
	children := []m.Widget{m.NewRow(head, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter})}
	for _, f := range []struct{ key, label, text string }{
		{KeyTranscriptionPrefix, "admin.submission.transcription", r.Transcription},
		{KeyFeedbackPrefix, "admin.submission.feedback", r.FeedbackForChild},
		{KeyParentNotesPrefix, "admin.submission.notes", r.NotesForParent},
	} {
		if f.text == "" {
			continue
		}
		children = append(children, gap(12), muted(bc, scope.T(f.label)), gap(2),
			m.NewText(f.text, m.TextOpts{Key: m.NewValueKey(f.key + r.ExerciseID), Style: theme.TextTheme.BodyLarge}))
	}
	if buttons := v.overrideButtons(scope, SectionSubmission, act); len(buttons) > 0 {
		children = append(children, gap(12), m.NewWrap(buttons, m.WrapOpts{Spacing: 12, RunSpacing: 8}))
	}
	return m.NewCard(m.NewPadding(m.NewEdgeInsetsAll(app.Gap), m.NewColumn(children, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStretch})),
		m.CardOpts{Key: m.NewValueKey(KeyResultPrefix + r.ExerciseID), Margin: m.NewEdgeInsetsAll(0)})
}

// deletePhotos asks for confirmation, then deletes the photos of the submission
// and clears the list in the save file; the record stays.
func (s *submissionPageState) deletePhotos(scope *app.Scope, submissionID string) {
	env := scope.Env()
	w := s.Widget()
	build := func(m.BuildContext) m.Widget {
		return confirmDialog(scope, scope.T("admin.submission.delete_photos_title"), scope.T("admin.submission.delete_photos_body"), scope.T("admin.submission.delete_photos"))
	}
	showDialog(s, build, func(yes bool) {
		if !yes {
			return
		}
		s.Go(func(ctx context.Context) {
			err := deleteSubmissionPhotos(ctx, env, w.person.Username, w.cassetteID, submissionID)
			if err != nil && ctx.Err() == nil {
				env.Log.Error("deleting the photos of a submission failed", "username", w.person.Username, "cassette", w.cassetteID, "submission", submissionID, "err", err)
			}
			s.SetState(func() {
				switch {
				case err == nil:
					showMessage(s.Context(), scope.T("admin.progress.photos_deleted"))
				case ctx.Err() == nil:
					showMessage(s.Context(), scope.Catalog().Msg(errMessage(err)))
				}
				s.reload()
			})
		})
	})
}

// photoViewer shows one photo at full size; it can be zoomed and moved with the
// fingers or the mouse wheel.
type photoViewer struct {
	m.StatelessWidget
	path          string
	number, total int
}

func newPhotoViewer(path string, n, total int) *photoViewer {
	return &photoViewer{StatelessWidget: m.NewStatelessWidgetBase(), path: path, number: n, total: total}
}

// Build builds the viewer.
func (w *photoViewer) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	theme := m.ThemeOf(bc)
	missing := func(m.BuildContext, error) m.Widget {
		return m.NewCenter(m.NewIcon(m.Icons.BrokenImage(), m.IconOpts{Size: m.Some(96.0), Color: m.Some(m.NewColor(0xFFFFFFFF))}))
	}
	picture := m.NewImageFile(w.path, m.ImageFileOpts{ImageOpts: m.ImageOpts{Fit: m.BoxFitContain, ErrorBuilder: missing}})
	return m.NewScaffold(m.ScaffoldOpts{
		Key:    m.NewValueKey(KeyPhotoViewer),
		AppBar: appBar(scope, bc, scope.T("admin.submission.photo_title", "n", itoa(w.number), "total", itoa(w.total))),
		Body: m.NewColoredBox(theme.ColorScheme.Scrim(), m.NewSizedBoxExpand(
			m.NewInteractiveViewer(m.NewSizedBoxExpand(picture), m.InteractiveViewerOpts{
				Key: m.NewValueKey(KeyPhotoZoom), MinScale: m.Some(1.0), MaxScale: m.Some(8.0),
			}))),
	})
}

package admin

import (
	"context"
	"sync/atomic"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// Keys of the review queue and of the notice at login. The keys of the items
// are a prefix followed by QueueID of the exercise; the keys of an attempt add
// "/" and the submission id, and the key of one of its photos "/" and the photo
// number (from 1).
const (
	// KeyReviewNotice is the key of the dialog that tells an admin at login that
	// exercises are waiting.
	KeyReviewNotice = "review-notice"
	// KeyNoticeNow is the key of the Review now button of that dialog.
	KeyNoticeNow = "review-notice-now"
	// KeyNoticeLater is the key of the Later button of that dialog.
	KeyNoticeLater = "review-notice-later"
	// KeyNoticeLinePrefix starts the key of a line of the list in that dialog;
	// QueueID follows.
	KeyNoticeLinePrefix = "review-notice-line-"
	// KeyOpenReview is the key of the button of the progress screen that opens
	// the review queue.
	KeyOpenReview = "open-review"
	// KeyQueue is the key of the review queue's list.
	KeyQueue = "review-queue"
	// KeyQueueEmpty is the key of the text "Nothing is waiting".
	KeyQueueEmpty = "review-empty"
	// KeyQueueHome is the key of the button that leads back to the home.
	KeyQueueHome = "review-home"
	// KeyQueueItemPrefix starts the key of the card of a waiting exercise.
	KeyQueueItemPrefix = "review-item-"
	// KeyQueuePromptPrefix starts the key of the exercise's prompt text.
	KeyQueuePromptPrefix = "review-prompt-"
	// KeyQueueReferenceButtonPrefix starts the key of the button that shows or
	// hides the reference solution.
	KeyQueueReferenceButtonPrefix = "review-reference-button-"
	// KeyQueueReferencePrefix starts the key of the reference solution's text.
	KeyQueueReferencePrefix = "review-reference-"
	// KeyQueueAttemptPrefix starts the key of the box of one attempt.
	KeyQueueAttemptPrefix = "review-attempt-"
	// KeyQueuePhotoPrefix starts the key of the thumbnail of an attempt's photo.
	KeyQueuePhotoPrefix = "review-photo-"
	// KeyQueueNoPhotosPrefix starts the key of the "Photos deleted" text of an
	// attempt.
	KeyQueueNoPhotosPrefix = "review-no-photos-"
	// KeyQueueVerdictPrefix starts the key of the verdict mark of an attempt.
	KeyQueueVerdictPrefix = "review-verdict-"
	// KeyQueuePassPrefix starts the key of the Pass button of an exercise.
	KeyQueuePassPrefix = "review-pass-"
	// KeyQueueMorePrefix starts the key of the Give more attempts button of an
	// exercise.
	KeyQueueMorePrefix = "review-more-"
)

// reviewNotice is the dialog that tells an admin, right after logging in, that
// exercises are waiting, and lists them (SPEC §12.1). It answers true for
// Review now and false for Later.
type reviewNotice struct {
	m.StatelessWidget
	items []waitingItem
}

func newReviewNotice(items []waitingItem) *reviewNotice {
	return &reviewNotice{StatelessWidget: m.NewStatelessWidgetBase(), items: items}
}

// noticeMaxLines is how many exercises the notice lists; the rest are counted.
const noticeMaxLines = 6

// Build builds the dialog.
func (w *reviewNotice) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	theme := m.ThemeOf(bc)
	children := []m.Widget{
		m.NewText(scope.T("admin.home.review", "n", itoa(len(w.items))), m.TextOpts{
			Style: theme.TextTheme.TitleMedium.CopyWith(m.TextStyleCopy{Color: m.Some(theme.ColorScheme.Error()), FontWeight: m.FontWeightW700}),
		}),
		gap(12),
	}
	for i, it := range w.items {
		if i == noticeMaxLines {
			children = append(children, muted(bc, scope.T("admin.review.notice.more", "n", itoa(len(w.items)-noticeMaxLines))))
			break
		}
		children = append(children, m.NewPadding(m.NewEdgeInsetsSymmetric(4, 0), m.NewRow([]m.Widget{
			m.NewIcon(m.Icons.HourglassEmpty(), m.IconOpts{Size: m.Some(22.0), Color: m.Some(theme.ColorScheme.Error())}),
			hgap(8),
			m.NewExpanded(m.NewText(it.listLine(scope), m.TextOpts{
				Key: m.NewValueKey(KeyNoticeLinePrefix + it.ID()), Style: theme.TextTheme.BodyLarge, MaxLines: m.Some(3), Overflow: m.TextOverflowEllipsis,
			})),
		}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart})))
	}
	return m.NewAlertDialog(m.AlertDialogOpts{
		Title: m.NewText(scope.T("admin.review.notice.title")),
		Content: m.NewSizedBox(m.NewColumn(children, m.ColumnOpts{
			Key: m.NewValueKey(KeyReviewNotice), CrossAxisAlignment: m.CrossAxisAlignmentStretch, MainAxisSize: m.MainAxisSizeMin,
		}), m.SizedBoxOpts{Width: m.Some(460.0)}),
		Scrollable: true,
		Actions: []m.Widget{
			dialogButton(KeyNoticeLater, scope.T("admin.review.later"), false, false),
			dialogButton(KeyNoticeNow, scope.T("admin.review.now"), true, true),
		},
	})
}

// ReviewQueue is the review screen (SPEC §12.3): the exercises that wait for a
// parent, over every student and cassette, oldest first, each with the photos
// of the attempts that failed and the two decisions, Pass and Give more
// attempts.
type ReviewQueue struct{ m.StatefulWidget }

// NewReviewQueue returns the review screen.
func NewReviewQueue(opts ...m.WidgetOpts) *ReviewQueue {
	return &ReviewQueue{StatefulWidget: m.NewStatefulWidgetBase(opts...)}
}

// CreateState creates the state.
func (w *ReviewQueue) CreateState() m.State { return &reviewQueueState{} }

type reviewQueueState struct {
	m.StateOf[*ReviewQueue]
	started    bool
	generation atomic.Int64
	items      []waitingItem
	loaded     bool
	failed     bool
	// reference holds the ids of the items whose reference solution is open.
	reference map[string]bool
}

// DidChangeDependencies starts the first load (the Scope cannot be read in
// InitState).
func (s *reviewQueueState) DidChangeDependencies() {
	if s.started {
		return
	}
	s.started = true
	s.reference = map[string]bool{}
	s.reload()
}

// reload reads the queue on a goroutine.
func (s *reviewQueueState) reload() {
	env := app.ScopeOf(s.Context()).Env()
	s.Go(func(ctx context.Context) {
		gen := s.generation.Add(1)
		items, err := scanQueue(ctx, env)
		if err != nil && ctx.Err() == nil {
			env.Log.Error("reading the review queue failed", "err", err)
		}
		s.SetState(func() {
			if s.generation.Load() == gen {
				s.items, s.loaded, s.failed = items, true, err != nil
			}
		})
	})
}

func (s *reviewQueueState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	var children []m.Widget
	switch {
	case s.failed:
		children = []m.Widget{errorText(bc, scope.T("admin.progress.load_failed"))}
	case !s.loaded:
		children = []m.Widget{m.NewCenter(m.NewCircularProgressIndicator())}
	case len(s.items) == 0:
		children = s.empty(bc, scope)
	default:
		children = []m.Widget{
			m.NewText(scope.T("admin.progress.waiting", "n", itoa(len(s.items))), m.TextOpts{
				Key: m.NewValueKey(KeyQueue), Style: m.ThemeOf(bc).TextTheme.HeadlineMedium.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600}),
			}),
			gap(app.Gap),
		}
		for _, it := range s.items {
			children = append(children, s.card(bc, scope, it), gap(app.Gap))
		}
	}
	return m.NewScaffold(m.ScaffoldOpts{
		AppBar: appBar(scope, bc, scope.T("admin.review.title")),
		Body:   pageBody(pageWidth, children),
	})
}

// empty is what the screen says when nothing is waiting, with the way back home.
func (s *reviewQueueState) empty(bc m.BuildContext, scope *app.Scope) []m.Widget {
	theme := m.ThemeOf(bc)
	return []m.Widget{
		gap(app.Gap * 2),
		m.NewIcon(m.Icons.CheckCircle(), m.IconOpts{Size: m.Some(72.0), Color: m.Some(theme.ColorScheme.Primary())}),
		gap(app.Gap),
		m.NewText(scope.T("admin.review.empty"), m.TextOpts{
			Key: m.NewValueKey(KeyQueueEmpty), TextAlign: m.TextAlignCenter,
			Style: theme.TextTheme.HeadlineMedium.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600}),
		}),
		gap(app.Gap * 1.5),
		m.NewCenter(m.NewFilledButtonIcon(func() {
			m.NavigatorPopUntil(bc, func(r m.Route) bool { return r.IsFirst() })
		}, m.NewText(scope.T("admin.review.home")), m.FilledButtonIconOpts{
			Icon: m.NewIcon(m.Icons.Home()), Key: m.NewValueKey(KeyQueueHome),
		})),
	}
}

// card is the card of one waiting exercise: who, which cassette, the exercise,
// the attempts that failed and the two decisions.
func (s *reviewQueueState) card(bc m.BuildContext, scope *app.Scope, it waitingItem) m.Widget {
	theme := m.ThemeOf(bc)
	id := it.ID()
	children := []m.Widget{
		m.NewRow([]m.Widget{
			m.NewIcon(m.Icons.Person(), m.IconOpts{Size: m.Some(24.0), Color: m.Some(theme.ColorScheme.Primary())}),
			hgap(8),
			m.NewExpanded(muted(bc, scope.T("admin.review.child_cassette", "name", it.DisplayName, "cassette", it.CassetteTitle))),
		}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter}),
		gap(6),
		m.NewText(scope.T("admin.progress.exercise_title", "label", it.Label, "title", it.Title), m.TextOpts{
			Style: theme.TextTheme.TitleLarge.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600}),
		}),
		gap(8),
		m.NewText(it.Prompt, m.TextOpts{Key: m.NewValueKey(KeyQueuePromptPrefix + id), Style: theme.TextTheme.BodyLarge}),
		gap(8),
		s.referenceBlock(bc, scope, it),
		gap(app.Gap),
		section(bc, "", scope.T("admin.review.attempts")),
		gap(8),
	}
	if len(it.Attempts) == 0 {
		children = append(children, muted(bc, scope.T("admin.review.no_attempts")))
	}
	for _, a := range it.Attempts {
		children = append(children, s.attempt(bc, scope, it, a), gap(12))
	}
	children = append(children, gap(4), m.NewWrap([]m.Widget{
		m.NewFilledButtonIcon(func() { s.decide(scope, it, progress.ActionMarkPassed) }, m.NewText(scope.T("admin.review.pass")), m.FilledButtonIconOpts{
			Icon: m.NewIcon(m.Icons.Check()), Key: m.NewValueKey(KeyQueuePassPrefix + id), Style: bigAction(),
		}),
		m.NewOutlinedButtonIcon(func() { s.confirmMore(scope, it) }, m.NewText(scope.T("admin.review.more")), m.OutlinedButtonIconOpts{
			Icon: m.NewIcon(m.Icons.Replay()), Key: m.NewValueKey(KeyQueueMorePrefix + id), Style: smallStyle(),
		}),
	}, m.WrapOpts{Spacing: 12, RunSpacing: 8}))
	return m.NewCard(m.NewPadding(m.NewEdgeInsetsAll(app.Gap),
		m.NewColumn(children, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStretch})),
		m.CardOpts{Key: m.NewValueKey(KeyQueueItemPrefix + id), Margin: m.NewEdgeInsetsAll(0)})
}

// bigAction is the style of the filled decision button.
func bigAction() *m.ButtonStyle {
	return m.FilledButtonStyleFrom(m.ButtonStyleFromOpts{
		MinimumSize: m.Some(m.Size{Width: 56, Height: 48}),
		Padding:     m.NewEdgeInsetsSymmetric(8, 18),
		Shape:       m.NewStadiumBorder(),
	})
}

// referenceBlock is the reference solution of the exercise, shut until the parent
// opens it.
func (s *reviewQueueState) referenceBlock(bc m.BuildContext, scope *app.Scope, it waitingItem) m.Widget {
	id := it.ID()
	open := s.reference[id]
	label, icon := scope.T("admin.review.reference_show"), m.Icons.ExpandMore()
	if open {
		label, icon = scope.T("admin.review.reference_hide"), m.Icons.ExpandLess()
	}
	children := []m.Widget{m.NewAlign(m.NewTextButtonIcon(func() {
		s.SetState(func() { s.reference[id] = !open })
	}, m.NewText(label), m.TextButtonIconOpts{Icon: m.NewIcon(icon), Key: m.NewValueKey(KeyQueueReferenceButtonPrefix + id)}),
		m.AlignOpts{Alignment: m.AlignmentCenterLeft})}
	if open {
		theme := m.ThemeOf(bc)
		children = append(children, gap(4), m.NewText(it.Reference, m.TextOpts{
			Key: m.NewValueKey(KeyQueueReferencePrefix + id), Style: theme.TextTheme.BodyLarge,
		}))
	}
	return m.NewColumn(children, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStretch})
}

// attempt is the box of one attempt that did not pass: its date, the photos of
// the submission (or the note that they were deleted), the verdict and the
// texts.
func (s *reviewQueueState) attempt(bc m.BuildContext, scope *app.Scope, it waitingItem, a waitingAttempt) m.Widget {
	theme := m.ThemeOf(bc)
	env := scope.Env()
	key := it.ID() + "/" + a.SubmissionID
	children := []m.Widget{
		m.NewRow([]m.Widget{
			verdictMark(bc, KeyQueueVerdictPrefix+key, a.Result.Verdict),
			hgap(app.Gap),
			m.NewExpanded(m.NewColumn([]m.Widget{
				bold(bc, formatTime(env.Location, a.At)),
				gap(2),
				muted(bc, scope.T("admin.submission.verdict", "verdict", scope.T("admin.verdict."+string(a.Result.Verdict)))),
			}, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart})),
		}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter}),
		gap(12),
	}
	if len(a.Photos) == 0 {
		children = append(children, m.NewText(scope.T("admin.review.photos_deleted"), m.TextOpts{
			Key:   m.NewValueKey(KeyQueueNoPhotosPrefix + key),
			Style: theme.TextTheme.BodyMedium.CopyWith(m.TextStyleCopy{Color: m.Some(theme.ColorScheme.OnSurfaceVariant()), FontStyle: m.FontStyleItalic}),
		}))
	} else {
		var thumbs []m.Widget
		for i, name := range a.Photos {
			path := photoPath(env, it.Username, it.CassetteID, a.SubmissionID, name)
			n, total := i+1, len(a.Photos)
			thumbs = append(thumbs, thumbnail(bc, scope, KeyQueuePhotoPrefix+key+"/"+itoa(n), path, func() {
				app.Push(bc, func(m.BuildContext) m.Widget { return newPhotoViewer(path, n, total) })
			}))
		}
		children = append(children, m.NewWrap(thumbs, m.WrapOpts{Spacing: 12, RunSpacing: 12}))
	}
	children = append(children, resultTexts(bc, scope, a.Result, key)...)
	return m.NewContainer(m.NewColumn(children, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStretch}), m.ContainerOpts{
		Key:     m.NewValueKey(KeyQueueAttemptPrefix + key),
		Padding: m.NewEdgeInsetsAll(12),
		Decoration: m.NewBoxDecoration(m.BoxDecorationOpts{
			Color:        m.Some(theme.ColorScheme.SurfaceContainerHighest()),
			BorderRadius: m.NewBorderRadiusCircular(12),
		}),
	})
}

// confirmMore asks the parent to confirm giving more attempts, saying that the
// child has to solve the exercise before going further.
func (s *reviewQueueState) confirmMore(scope *app.Scope, it waitingItem) {
	build := func(m.BuildContext) m.Widget {
		return confirmDialog(scope, scope.T("admin.review.more_title"),
			scope.T("admin.review.more_body", "label", it.Label, "name", it.DisplayName), scope.T("admin.review.more"))
	}
	showDialog(s, build, func(yes bool) {
		if yes {
			s.decide(scope, it, progress.ActionResetAttempts)
		}
	})
}

// decide applies the parent's decision to the exercise (Pass is Mark passed,
// Give more attempts is Reset attempts, SPEC §7.7) and reads the queue again,
// from which the exercise has gone.
func (s *reviewQueueState) decide(scope *app.Scope, it waitingItem, action progress.Action) {
	data := &reviewData{ID: it.CassetteID, Title: it.CassetteTitle, Cassette: it.Cassette}
	runOverride(s, scope, data, it.Username, progress.Override{Action: action, ExerciseID: it.ExerciseID}, s.reload)
}

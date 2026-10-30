package worksheet

import (
	"strconv"
	"strings"

	"github.com/rm4n0s/errors"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/submission"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/marks"
)

const (
	markSize      = 56.0
	flowMaxWidth  = 760.0
	qrPadding     = 16.0
	qrCardRadius  = 16.0
	firewallParts = "sudo firewall-cmd --add-port=%d/tcp --permanent && sudo firewall-cmd --reload"
)

// openingView is the screen while the submission is being opened.
func (s *flowState) openingView(bc m.BuildContext, scope *app.Scope) m.Widget {
	return m.NewCenter(m.NewColumn([]m.Widget{
		m.NewCircularProgressIndicator(),
		gap(app.Gap),
		m.NewText(scope.T("flow.preparing"), m.TextOpts{Style: m.ThemeOf(bc).TextTheme.TitleMedium}),
	}, m.ColumnOpts{MainAxisSize: m.MainAxisSizeMin}))
}

// labels are the labels of the exercises of the submission, "1, 2, B3".
func (s *flowState) labels() string {
	var out []string
	for _, id := range s.Widget().req.ExerciseIDs {
		if ex := s.Widget().req.Cassette.Exercises[id]; ex != nil {
			out = append(out, ex.Label)
		}
	}
	return strings.Join(out, ", ")
}

// qrView is the QR dialog (SPEC §10.1): the code, the address as text, the
// labels, "Scan this with your phone", the photos received so far, the hint
// about the firewall when it is time, and Cancel.
func (s *flowState) qrView(bc m.BuildContext, scope *app.Scope) m.Widget {
	theme := m.ThemeOf(bc)
	title := theme.TextTheme.HeadlineSmall.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600})
	code := m.NewContainer(qrCode(KeyQR, s.matrix), m.ContainerOpts{
		Padding: m.NewEdgeInsetsAll(qrPadding),
		Decoration: m.NewBoxDecoration(m.BoxDecorationOpts{
			Color:        m.Some(m.NewColor(0xFFFFFFFF)),
			BorderRadius: m.NewBorderRadiusCircular(qrCardRadius),
			Border:       m.NewBorderAll(m.BorderSideOpts{Color: m.Some(theme.ColorScheme.OutlineVariant()), Width: m.Some(2.0)}),
		}),
	})
	right := []m.Widget{
		m.NewText(scope.T("flow.scan"), m.TextOpts{Style: title}),
		gap(app.Gap),
		m.NewText(s.sess.URL, m.TextOpts{Key: m.NewValueKey(KeyURL), Style: theme.TextTheme.TitleMedium}),
		gap(12),
		m.NewText(scope.T("flow.exercises", "labels", s.labels()), m.TextOpts{Key: m.NewValueKey(KeyLabels), Style: theme.TextTheme.BodyLarge}),
		gap(12),
	}
	if status := s.phoneStatus(scope); status != "" {
		right = append(right, m.NewText(status, m.TextOpts{Key: m.NewValueKey(KeyPhoneStatus), Style: theme.TextTheme.BodyLarge}), gap(12))
	}
	right = append(right,
		m.NewText(scope.T("flow.photos", "n", strconv.Itoa(s.photos)), m.TextOpts{
			Key: m.NewValueKey(KeyPhotos), Style: theme.TextTheme.TitleLarge.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600}),
		}),
		gap(app.Gap),
	)
	if s.firewall && !s.pageOpened {
		right = append(right, s.firewallHint(bc, scope), gap(app.Gap))
	}
	right = append(right, cancelButton(bc, scope, true))
	return pageBody(flowMaxWidth+60, []m.Widget{
		m.NewRow([]m.Widget{
			code,
			hgap(app.PagePadding),
			m.NewExpanded(m.NewColumn(right, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart})),
		}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart}),
	})
}

// phoneStatus says what the phone is doing.
func (s *flowState) phoneStatus(scope *app.Scope) string {
	switch {
	case s.uploading:
		return scope.T("flow.uploading")
	case s.pageOpened:
		return scope.T("flow.opened")
	}
	return ""
}

// firewallHint is the hint shown when no phone opened the page within a minute
// (SPEC §10.2): the port from the settings and the command of the README.
func (s *flowState) firewallHint(bc m.BuildContext, scope *app.Scope) m.Widget {
	scheme := m.ThemeOf(bc).ColorScheme
	port := s.env.Settings.Get().Phone.Port
	command := strings.Replace(firewallParts, "%d", strconv.Itoa(port), 1)
	return m.NewContainer(
		m.NewText(scope.T("flow.firewall", "port", strconv.Itoa(port), "command", command), m.TextOpts{
			Key:   m.NewValueKey(KeyFirewall),
			Style: m.ThemeOf(bc).TextTheme.BodyMedium.CopyWith(m.TextStyleCopy{Color: m.Some(scheme.OnTertiaryContainer())}),
		}),
		m.ContainerOpts{
			Padding:    m.NewEdgeInsetsAll(12),
			Decoration: m.NewBoxDecoration(m.BoxDecorationOpts{Color: m.Some(scheme.TertiaryContainer()), BorderRadius: m.NewBorderRadiusCircular(12)}),
		},
	)
}

// cancelButton is the Cancel button; it closes the screen, which cancels the
// submission.
func cancelButton(bc m.BuildContext, scope *app.Scope, outlined bool) m.Widget {
	press := func() { app.Pop(bc, false) }
	label := m.NewText(scope.T("flow.cancel"))
	if outlined {
		return m.NewOutlinedButton(press, label, m.OutlinedButtonOpts{Key: m.NewValueKey(KeyCancel)})
	}
	return m.NewTextButton(press, label, m.TextButtonOpts{Key: m.NewValueKey(KeyCancel)})
}

// gradingView is the indicator shown while the photos are graded.
func (s *flowState) gradingView(bc m.BuildContext, scope *app.Scope) m.Widget {
	theme := m.ThemeOf(bc)
	return m.NewCenter(m.NewColumn([]m.Widget{
		m.NewSizedBox(m.NewCircularProgressIndicator(), m.SizedBoxOpts{Key: m.NewValueKey(KeyGrading), Width: m.Some(72.0), Height: m.Some(72.0)}),
		gap(app.PagePadding),
		m.NewText(scope.T("flow.grading"), m.TextOpts{Style: theme.TextTheme.HeadlineSmall}),
		gap(8),
		m.NewText(scope.T("flow.grading_wait"), m.TextOpts{Style: theme.TextTheme.BodyLarge}),
		gap(8),
		m.NewText(scope.T("flow.photos", "n", strconv.Itoa(s.photos)), m.TextOpts{Key: m.NewValueKey(KeyPhotos), Style: theme.TextTheme.BodyLarge}),
		gap(app.PagePadding),
		cancelButton(bc, scope, false),
	}, m.ColumnOpts{MainAxisSize: m.MainAxisSizeMin}))
}

// mark is the big mark of a result, with a tooltip that says the words.
func mark(bc m.BuildContext, scope *app.Scope, key string, lk marks.Look) m.Widget {
	return m.NewTooltip(marks.Mark(key, lk, marks.Size{Box: markSize, Icon: 36, Text: m.ThemeOf(bc).TextTheme.HeadlineLarge}),
		m.TooltipOpts{Message: scope.T(lk.Words)})
}

// resultsView is the results screen (SPEC §10.1): a row per exercise with its
// label, the big mark, the feedback for the child, the hint that was unlocked
// and a star for the exercises that are passed now.
func (s *flowState) resultsView(bc m.BuildContext, scope *app.Scope) m.Widget {
	g := s.graded
	req := s.Widget().req
	theme := m.ThemeOf(bc)
	changes := map[string]progress.ExerciseChange{}
	for _, ch := range g.Changes.Exercises {
		changes[ch.ExerciseID] = ch
	}
	var children []m.Widget
	for _, r := range g.Results {
		ex := req.Cassette.Exercises[r.ExerciseID]
		if ex == nil {
			continue
		}
		children = append(children, resultRow(bc, scope, ex, r, changes[r.ExerciseID]), gap(12))
	}
	if g.Downgraded != "" && isAdmin(scope) {
		children = append(children, m.NewText(scope.T("flow.downgraded", "mode", string(g.Downgraded)), m.TextOpts{
			Key: m.NewValueKey(KeyDowngraded), Style: theme.TextTheme.BodyMedium,
		}), gap(12))
	}
	children = append(children, gap(8), m.NewFilledButtonIcon(func() { app.Pop(bc, true) }, m.NewText(scope.T("flow.back")), m.FilledButtonIconOpts{
		Icon:  m.NewIcon(m.Icons.ArrowBack()),
		Key:   m.NewValueKey(KeyBack),
		Style: bigButtonStyle(),
	}))
	return pageBody(flowMaxWidth, children)
}

// resultRow is the row of one exercise in the results.
func resultRow(bc m.BuildContext, scope *app.Scope, ex *cassette.Exercise, r progress.Result, ch progress.ExerciseChange) m.Widget {
	theme := m.ThemeOf(bc)
	lk := marks.LookOf(theme.ColorScheme, r.Verdict)
	words := theme.TextTheme.TitleLarge.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600})
	text := []m.Widget{m.NewText(scope.T(lk.Words), m.TextOpts{Style: words})}
	if r.FeedbackForChild != "" {
		text = append(text, gap(4), m.NewText(r.FeedbackForChild, m.TextOpts{Key: m.NewValueKey(KeyFeedbackPrefix + r.ExerciseID), Style: theme.TextTheme.BodyLarge}))
	}
	if ch.HintShown > 0 && ch.HintShown <= len(ex.Hints) {
		text = append(text, gap(8), resultLine(theme, KeyNewHintPrefix+r.ExerciseID, m.Icons.Lightbulb(), theme.ColorScheme.Tertiary(),
			scope.T("flow.new_hint", "text", ex.Hints[ch.HintShown-1])))
	}
	if ch.NeedsReview || ch.Ignored == progress.IgnoredNeedsReview {
		text = append(text, gap(8), resultLine(theme, KeyReviewPrefix+r.ExerciseID, m.Icons.Flag(), theme.ColorScheme.Error(), scope.T("worksheet.needs_review")))
	}
	row := []m.Widget{
		badge(bc, KeyBadgePrefix+"result-"+r.ExerciseID, ex.Label, false),
		hgap(app.Gap),
		mark(bc, scope, KeyMarkPrefix+r.ExerciseID, lk),
		hgap(app.Gap),
		m.NewExpanded(m.NewColumn(text, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart})),
	}
	if ch.Passed {
		row = append(row, hgap(app.Gap), m.NewTooltip(
			m.NewIcon(m.Icons.Star(), m.IconOpts{Key: m.NewValueKey(KeyStarPrefix + r.ExerciseID), Size: m.Some(44.0), Color: m.Some(m.NewColor(0xFFF9A825))}),
			m.TooltipOpts{Message: scope.T("student.state.passed")},
		))
	}
	return m.NewCard(m.NewPadding(m.NewEdgeInsetsAll(app.Gap), m.NewRow(row, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter})),
		m.CardOpts{Key: m.NewValueKey(KeyResultPrefix + r.ExerciseID)})
}

// resultLine is an icon with a line of text inside a result.
func resultLine(theme *m.ThemeData, key string, icon m.IconData, color m.Color, text string) m.Widget {
	return m.NewKeyedSubtree(m.NewRow([]m.Widget{
		m.NewIcon(icon, m.IconOpts{Size: m.Some(24.0), Color: m.Some(color)}),
		hgap(8),
		m.NewExpanded(m.NewText(text, m.TextOpts{Style: theme.TextTheme.BodyLarge})),
	}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart}), m.WidgetOpts{Key: m.NewValueKey(key)})
}

// failureText is the child-friendly message of a failure, by the tag of its
// cause (SPEC §10.1, §10.2). The details stay for the admins.
func failureText(cat i18n.Catalog, f *failure) string {
	if f.expired {
		return cat.T("flow.failed.expired")
	}
	if f.err == nil {
		return cat.T("flow.failed.other")
	}
	if submission.NoProvider(f.err) {
		return cat.T("flow.failed.no_provider")
	}
	appErr, ok := errors.FromError(f.err)
	if !ok {
		return cat.T("flow.failed.checker")
	}
	switch appErr.Tag {
	case "PhoneNoNetwork":
		return cat.T("flow.failed.no_network")
	case "PhonePortInUse":
		return cat.T("flow.failed.port")
	case "OpenAIAuth", "OpenAINotFound", "OpenAIRateLimited", "OpenAINetwork", "OpenAITimeout", "OpenAIServer", "OpenAIStatus",
		"OpenAIBodyInvalid", "OpenAINoChoices", "OpenAIContentInvalid", "GradingReplyInvalid", "GradingReplyTruncated":
		return cat.T("flow.failed.checker")
	}
	if f.canRetry {
		return cat.T("flow.failed.checker")
	}
	return cat.T("flow.failed.other")
}

// failureDetails is the technical text only an admin sees; empty for a child.
func failureDetails(cat i18n.Catalog, admin bool, f *failure) string {
	if f.err == nil || !admin {
		return ""
	}
	if appErr, ok := errors.FromError(f.err); ok {
		return cat.T("flow.failed.details", "message", appErr.Message, "tag", appErr.Tag)
	}
	return cat.T("flow.failed.details", "message", f.err.Error(), "tag", "Unknown")
}

// isAdmin reports whether the user who is logged in is an admin.
func isAdmin(scope *app.Scope) bool {
	return scope.Session() != nil && scope.Session().Role == profile.RoleAdmin
}

// failureView is the screen after a failure: a friendly message, Try again and
// Cancel.
func (s *flowState) failureView(bc m.BuildContext, scope *app.Scope) m.Widget {
	theme := m.ThemeOf(bc)
	f := s.fail
	if f == nil {
		f = &failure{}
	}
	children := []m.Widget{
		m.NewIcon(m.Icons.CloudOff(), m.IconOpts{Size: m.Some(72.0), Color: m.Some(theme.ColorScheme.OnSurfaceVariant())}),
		gap(app.Gap),
		m.NewText(failureText(scope.Catalog(), f), m.TextOpts{Key: m.NewValueKey(KeyFailure), Style: theme.TextTheme.HeadlineSmall, TextAlign: m.TextAlignCenter}),
	}
	if details := failureDetails(scope.Catalog(), isAdmin(scope), f); details != "" {
		children = append(children, gap(12), m.NewText(details, m.TextOpts{Key: m.NewValueKey(KeyDetails), Style: theme.TextTheme.BodyMedium, TextAlign: m.TextAlignCenter}))
	}
	children = append(children,
		gap(app.PagePadding),
		m.NewFilledButtonIcon(s.tryAgain, m.NewText(scope.T("flow.try_again")), m.FilledButtonIconOpts{
			Icon: m.NewIcon(m.Icons.Refresh()), Key: m.NewValueKey(KeyRetry), Style: bigButtonStyle(),
		}),
		gap(8),
		cancelButton(bc, scope, false),
	)
	return pageBody(flowMaxWidth, []m.Widget{m.NewColumn(children, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter})})
}

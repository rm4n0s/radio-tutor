package admin

import (
	"time"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// The sections of the progress page, which keep the keys of an item that is
// shown twice (once under "Waiting for you", once in the list) apart, and the
// section of the submission screen.
const (
	// SectionItems is the list of all the items of a cassette.
	SectionItems = "items"
	// SectionWaiting is the "Waiting for you" list of a cassette.
	SectionWaiting = "waiting"
	// SectionSubmission is the results of a submission.
	SectionSubmission = "submission"
)

// ItemKey is the key of the card of an item (a lesson or an exercise) in the
// section.
func ItemKey(section, id string) string { return "item-" + section + "-" + id }

// StateKey is the key of the text that says the state of an item in the
// section.
func StateKey(section, id string) string { return "item-state-" + section + "-" + id }

// OverrideKey is the key of the override button for the action on the item (an
// exercise or a lesson id) in the section.
func OverrideKey(section string, action progress.Action, id string) string {
	return "override-" + section + "-" + string(action) + "-" + id
}

// timeLayout is how the dates are shown: the local time of the computer.
const timeLayout = "2006-01-02 15:04"

// formatTime is a time as the admin sees it.
func formatTime(t time.Time) string { return t.Local().Format(timeLayout) }

// itemView is what the card of one lesson or exercise shows.
type itemView struct {
	id, title string
	// label is the exercise's label; empty for a lesson.
	label  string
	lesson bool
	// state is the state by the unlock rule; empty for an entry of a cassette
	// that cannot be read.
	state       progress.ItemState
	lp          progress.LessonProgress
	ep          progress.ExerciseProgress
	maxAttempts int
}

// stateStyle is the icon and the color of a state.
func stateStyle(theme *m.ThemeData, state progress.ItemState) (m.IconData, m.Color) {
	switch state {
	case progress.StateLocked:
		return m.Icons.Lock(), theme.ColorScheme.OnSurfaceVariant()
	case progress.StateCompleted, progress.StatePassed:
		return m.Icons.CheckCircle(), theme.ColorScheme.Primary()
	case progress.StateNeedsReview:
		return m.Icons.Flag(), theme.ColorScheme.Error()
	}
	return m.Icons.RadioButtonUnchecked(), theme.ColorScheme.Tertiary()
}

// stateBadge is the icon and the words of a state.
func stateBadge(bc m.BuildContext, scope *app.Scope, key string, state progress.ItemState) m.Widget {
	theme := m.ThemeOf(bc)
	icon, color := stateStyle(theme, state)
	return m.NewRow([]m.Widget{
		m.NewIcon(icon, m.IconOpts{Size: m.Some(24.0), Color: m.Some(color)}),
		hgap(6),
		m.NewText(scope.T("admin.progress.state."+string(state)), m.TextOpts{
			Key:   m.NewValueKey(key),
			Style: theme.TextTheme.TitleMedium.CopyWith(m.TextStyleCopy{Color: m.Some(color), FontWeight: m.FontWeightW600}),
		}),
	}, m.RowOpts{MainAxisSize: m.MainAxisSizeMin})
}

// details are the muted lines under the title of an item: attempts and hints,
// who passed it and when, or the completion and the plays.
func (v itemView) details(scope *app.Scope) []string {
	var lines []string
	if v.lesson {
		if v.lp.Completed() {
			lines = append(lines, scope.T("admin.progress.completed_on", "date", formatTime(v.lp.CompletedAt)))
		} else {
			lines = append(lines, scope.T("admin.progress.not_completed"))
		}
		if v.lp.Plays > 0 {
			lines = append(lines, scope.T("admin.progress.plays", "n", itoa(v.lp.Plays)))
		}
		return lines
	}
	if v.maxAttempts > 0 {
		lines = append(lines, scope.T("admin.progress.attempts", "used", itoa(v.ep.Attempts), "max", itoa(v.maxAttempts)))
	} else {
		lines = append(lines, scope.T("admin.progress.attempts_unlimited", "used", itoa(v.ep.Attempts)))
	}
	lines = append(lines, scope.T("admin.progress.hints", "n", itoa(v.ep.HintsShown)))
	if v.ep.State == progress.StatePassed {
		key := "admin.progress.passed_by_ai"
		if v.ep.PassedBy == progress.PassedByParent {
			key = "admin.progress.passed_by_parent"
		}
		lines = append(lines, scope.T(key, "date", formatTime(v.ep.PassedAt)))
	}
	return lines
}

// overrideButton is a small button that performs an override.
func overrideButton(section string, action progress.Action, id, label string, icon m.IconData, press func()) m.Widget {
	return m.NewOutlinedButtonIcon(press, m.NewText(label), m.OutlinedButtonIconOpts{
		Icon: m.NewIcon(icon), Key: m.NewValueKey(OverrideKey(section, action, id)), Style: smallStyle(),
	})
}

// overrideButtons are the buttons that apply to the item right now (SPEC §7.7):
// an exercise that is not passed can be marked passed, a passed one marked not
// passed, and one with attempts, hints or a state can have them reset; a lesson
// can be marked completed or not completed.
func (v itemView) overrideButtons(scope *app.Scope, section string, act func(progress.Override)) []m.Widget {
	if act == nil {
		return nil
	}
	do := func(a progress.Action) func() {
		return func() {
			o := progress.Override{Action: a}
			if v.lesson {
				o.LessonID = v.id
			} else {
				o.ExerciseID = v.id
			}
			act(o)
		}
	}
	btn := func(a progress.Action, key string, icon m.IconData) m.Widget {
		return overrideButton(section, a, v.id, scope.T(key), icon, do(a))
	}
	var out []m.Widget
	if v.lesson {
		if v.lp.Completed() {
			return append(out, btn(progress.ActionMarkNotCompleted, "admin.progress.mark_not_completed", m.Icons.Undo()))
		}
		return append(out, btn(progress.ActionMarkCompleted, "admin.progress.mark_completed", m.Icons.Check()))
	}
	if v.ep.State == progress.StatePassed {
		out = append(out, btn(progress.ActionMarkNotPassed, "admin.progress.mark_not_passed", m.Icons.Undo()))
	} else {
		out = append(out, btn(progress.ActionMarkPassed, "admin.progress.mark_passed", m.Icons.Check()))
	}
	if v.ep.Attempts > 0 || v.ep.HintsShown > 0 || v.ep.State != "" {
		out = append(out, btn(progress.ActionResetAttempts, "admin.progress.reset_attempts", m.Icons.Replay()))
	}
	return out
}

// itemCard is the card of a lesson or an exercise: its title and state, the
// numbers and dates, and the override buttons (none when act is nil).
func itemCard(bc m.BuildContext, scope *app.Scope, section string, v itemView, act func(progress.Override)) m.Widget {
	theme := m.ThemeOf(bc)
	icon := m.Icons.EditNote()
	title := scope.T("admin.progress.exercise_title", "label", v.label, "title", v.title)
	if v.lesson {
		icon, title = m.Icons.PlayCircle(), scope.T("admin.progress.lesson_title", "title", v.title)
	}
	lines := []m.Widget{bold(bc, title)}
	for _, d := range v.details(scope) {
		lines = append(lines, gap(2), muted(bc, d))
	}
	head := []m.Widget{
		m.NewIcon(icon, m.IconOpts{Size: m.Some(36.0), Color: m.Some(theme.ColorScheme.Primary())}),
		hgap(app.Gap),
		m.NewExpanded(m.NewColumn(lines, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart})),
	}
	if v.state != "" {
		head = append(head, hgap(8), stateBadge(bc, scope, StateKey(section, v.id), v.state))
	}
	children := []m.Widget{m.NewRow(head, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter})}
	if buttons := v.overrideButtons(scope, section, act); len(buttons) > 0 {
		children = append(children, gap(12), m.NewWrap(buttons, m.WrapOpts{Spacing: 12, RunSpacing: 8}))
	}
	return m.NewCard(m.NewPadding(m.NewEdgeInsetsAll(app.Gap),
		m.NewColumn(children, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStretch})),
		m.CardOpts{Key: m.NewValueKey(ItemKey(section, v.id)), Margin: m.NewEdgeInsetsAll(0)})
}

// verdictLook is how a verdict is shown: a mark in a colored circle. It is the
// same as in the results the child sees (package worksheet): a check for correct,
// ~ for partial, a cross for incorrect and ? for the rest.
type verdictLook struct {
	icon  m.IconData
	text  string // shown instead of the icon (for ~ and ?)
	color m.Color
}

// lookOf returns how the verdict is shown.
func lookOf(scheme *m.ColorScheme, v progress.Verdict) verdictLook {
	switch v {
	case progress.VerdictCorrect:
		return verdictLook{icon: m.Icons.Check(), color: m.NewColor(0xFF2E7D32)}
	case progress.VerdictPartial:
		return verdictLook{text: "~", color: m.NewColor(0xFFE65100)}
	case progress.VerdictIncorrect:
		return verdictLook{icon: m.Icons.Close(), color: scheme.Error()}
	}
	return verdictLook{text: "?", color: scheme.Outline()}
}

// verdictMark is the round mark of a verdict.
func verdictMark(bc m.BuildContext, key string, v progress.Verdict) m.Widget {
	const size = 48.0
	theme := m.ThemeOf(bc)
	lk := lookOf(theme.ColorScheme, v)
	white := m.NewColor(0xFFFFFFFF)
	var inner m.Widget
	if lk.text != "" {
		inner = m.NewText(lk.text, m.TextOpts{Style: theme.TextTheme.HeadlineMedium.CopyWith(m.TextStyleCopy{Color: m.Some(white), FontWeight: m.FontWeightW700})})
	} else {
		inner = m.NewIcon(lk.icon, m.IconOpts{Size: m.Some(30.0), Color: m.Some(white)})
	}
	return m.NewContainer(inner, m.ContainerOpts{
		Key:        m.NewValueKey(key),
		Width:      m.Some(size),
		Height:     m.Some(size),
		Alignment:  m.AlignmentCenter,
		Decoration: m.NewBoxDecoration(m.BoxDecorationOpts{Color: m.Some(lk.color), Shape: m.BoxShapeCircle}),
	})
}

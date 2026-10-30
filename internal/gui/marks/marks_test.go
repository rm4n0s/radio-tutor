package marks_test

import (
	"testing"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/marks"
)

func TestLookOf(t *testing.T) {
	cat := i18n.For(i18n.EN)
	scheme := m.NewColorSchemeLight()
	tests := []struct {
		verdict progress.Verdict
		text    string // the character of a text mark; empty for an icon
		words   string
	}{
		{progress.VerdictCorrect, "", "Correct!"},
		{progress.VerdictPartial, "~", "Almost there"},
		{progress.VerdictIncorrect, "", "Not yet"},
		{progress.VerdictUnreadable, "?", "I could not read it"},
		{progress.VerdictNotFound, "?", "I could not find it on the page"},
		{progress.Verdict("something new"), "?", "I could not read it"},
	}
	for _, tt := range tests {
		t.Run(string(tt.verdict), func(t *testing.T) {
			lk := marks.LookOf(scheme, tt.verdict)
			if lk.Text != tt.text {
				t.Errorf("mark text %q, want %q", lk.Text, tt.text)
			}
			if got := cat.T(lk.Words); got != tt.words {
				t.Errorf("words %q, want %q", got, tt.words)
			}
		})
	}
	// Every verdict the grading package knows has a look.
	for _, v := range []grading.Verdict{grading.VerdictCorrect, grading.VerdictPartial, grading.VerdictIncorrect, grading.VerdictUnreadable, grading.VerdictNotFound} {
		if marks.LookOf(scheme, progress.Verdict(v)).Words == "" {
			t.Errorf("verdict %s has no words", v)
		}
	}
}

func TestMarkBuilds(t *testing.T) {
	scheme := m.NewColorSchemeLight()
	theme := m.NewThemeData()
	for _, v := range []progress.Verdict{progress.VerdictCorrect, progress.VerdictPartial} {
		w := marks.Mark("k", marks.LookOf(scheme, v), marks.Size{Box: 40, Icon: 24, Text: theme.TextTheme.HeadlineMedium})
		if w == nil {
			t.Errorf("no mark for %s", v)
		}
	}
}

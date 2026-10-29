package cassette

import (
	"strconv"
	"unicode"
	"unicode/utf8"

	"github.com/rm4n0s/radio-tutor/internal/core/problem"
)

// The limits of CASSETTE-FORMAT §7.2.
const (
	// MaxLabelRunes is the most characters of an exercise label.
	MaxLabelRunes = 6
	// MaxPromptRunes is the most characters of an exercise prompt.
	MaxPromptRunes = 2000
	// MaxSolutionRunes is the most characters of a reference solution.
	MaxSolutionRunes = 4000
	// MaxGradingRunes is the most characters of an exercise's grading
	// instructions.
	MaxGradingRunes = 4000
	// MaxAttemptsLimit is the largest value of max_attempts.
	MaxAttemptsLimit = 20
	// DefaultMaxAttempts is the max_attempts of an exercise that does not say.
	DefaultMaxAttempts = 3
	// MaxHints is the most hints an exercise may have.
	MaxHints = 5
	// MaxHintRunes is the most characters of a hint.
	MaxHintRunes = 300
)

// Exercise is an exercise.json (CASSETTE-FORMAT §7.2).
type Exercise struct {
	// ID is the exercise's id (its folder name). It is not in the file.
	ID string `json:"-"`
	// Format is the format version, always FormatVersion.
	Format int `json:"format"`
	// Title is the exercise's title.
	Title string `json:"title"`
	// Label is what the child writes next to the solution. When the file has
	// no label, Load sets it to the exercise's position among all the
	// exercises of the cassette, counting from 1 ("1", "2", ...).
	Label string `json:"label,omitempty"`
	// Prompt is what the child sees and does.
	Prompt string `json:"prompt"`
	// ReferenceSolution is the right answer, with the working if ShowWork.
	ReferenceSolution string `json:"reference_solution"`
	// Grading tells the grader what counts as correct for this exercise.
	Grading string `json:"grading"`
	// ShowWork is true when the working is graded too. Default false.
	ShowWork bool `json:"show_work,omitempty"`
	// MaxAttempts is how many wrong tries a child gets before a parent looks
	// at the exercise; 0 means no limit. DefaultMaxAttempts when the file
	// does not say.
	MaxAttempts int `json:"max_attempts"`
	// Hints are shown one after each wrong try.
	Hints []string `json:"hints,omitempty"`
}

var exerciseSchema = &schema{fields: []field{
	{name: "format", kind: kindInt, required: true},
	{name: "title", kind: kindString, required: true},
	{name: "label", kind: kindString},
	{name: "prompt", kind: kindString, required: true},
	{name: "reference_solution", kind: kindString, required: true},
	{name: "grading", kind: kindString, required: true},
	{name: "show_work", kind: kindBool},
	{name: "max_attempts", kind: kindInt},
	{name: "hints", kind: kindStrings},
}}

// validate applies the field rules of CASSETTE-FORMAT §7.2, with paths
// relative to exercise.json. labelSet says whether the file has a label (an
// empty one is then an error); the default label is not checked here.
func (e *Exercise) validate(l *problem.List, labelSet bool) {
	checkFormat(l, e.Format)
	checkText(l, "title", e.Title, 1, MaxTitleRunes)
	if labelSet && !validLabel(e.Label) {
		l.Errorf("label", "%q is not a label: 1 to %d letters, digits, dots or hyphens", e.Label, MaxLabelRunes)
	}
	checkText(l, "prompt", e.Prompt, 1, MaxPromptRunes)
	checkText(l, "reference_solution", e.ReferenceSolution, 1, MaxSolutionRunes)
	checkText(l, "grading", e.Grading, 1, MaxGradingRunes)
	if e.MaxAttempts < 0 || e.MaxAttempts > MaxAttemptsLimit {
		l.Errorf("max_attempts", "%d is not from 0 to %d", e.MaxAttempts, MaxAttemptsLimit)
	}
	if len(e.Hints) > MaxHints {
		l.Errorf("hints", "has %d hints; at most %d are allowed", len(e.Hints), MaxHints)
	}
	for i, h := range e.Hints {
		checkText(l, problem.Index("hints", i), h, 1, MaxHintRunes)
	}
}

// validLabel reports whether s is 1 to MaxLabelRunes characters, each a
// letter (of any alphabet), a digit, '.' or '-'.
func validLabel(s string) bool {
	n := utf8.RuneCountInString(s)
	if n < 1 || n > MaxLabelRunes {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' && r != '-' {
			return false
		}
	}
	return true
}

// defaultLabel is the label of the exercise at the 1-based position pos among
// all the exercises of the cassette.
func defaultLabel(pos int) string {
	return strconv.Itoa(pos)
}

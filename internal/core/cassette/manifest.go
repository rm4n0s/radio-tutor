package cassette

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/language"

	"github.com/rm4n0s/radio-tutor/internal/core/problem"
)

// The limits of CASSETTE-FORMAT §3.
const (
	// FormatVersion is the only cassette format this Radio Tutor reads.
	FormatVersion = 1
	// MaxTitleRunes is the most characters a cassette, lesson or exercise
	// title may have.
	MaxTitleRunes = 80
	// MaxDescriptionRunes is the most characters of a manifest description.
	MaxDescriptionRunes = 500
	// MaxSubjectRunes is the most characters of a manifest subject.
	MaxSubjectRunes = 40
	// MaxSourceRunes is the most characters of a manifest source.
	MaxSourceRunes = 300
	// MaxGraderInstructionsRunes is the most characters of the manifest's
	// grader instructions.
	MaxGraderInstructionsRunes = 4000
	// MaxItems is the most items a cassette may list.
	MaxItems = 500
	// MinAge is the lowest age a manifest may name.
	MinAge = 3
	// MaxAge is the highest age a manifest may name.
	MaxAge = 18
)

var (
	idPattern         = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	cassetteIDPattern = regexp.MustCompile(`^[a-z0-9]+(\.[a-z0-9][a-z0-9-]*)+$`)
)

// ItemType says what a manifest item is.
type ItemType string

// The types of an Item.
const (
	// ItemLesson is a lesson: lessons/<id>/.
	ItemLesson ItemType = "lesson"
	// ItemExercise is an exercise: exercises/<id>/.
	ItemExercise ItemType = "exercise"
)

// Item is one entry of the manifest's items: a lesson or an exercise, in the
// order the child follows.
type Item struct {
	// Type is ItemLesson or ItemExercise.
	Type ItemType `json:"type"`
	// ID is the id of the lesson or exercise, and the name of its folder.
	ID string `json:"id"`
}

// Manifest is cassette.json (CASSETTE-FORMAT §3).
type Manifest struct {
	// Format is the format version, always FormatVersion.
	Format int `json:"format"`
	// ID is the reverse-domain id of the cassette.
	ID string `json:"id"`
	// Version is the cassette's version.
	Version Version `json:"version"`
	// Title is the cassette's title.
	Title string `json:"title"`
	// Description says what the cassette is about. Optional.
	Description string `json:"description,omitempty"`
	// Language is the BCP 47 tag of the content, such as "en" or "el".
	Language string `json:"language"`
	// Subject is free text such as "math".
	Subject string `json:"subject"`
	// Ages is [min, max], integers from MinAge to MaxAge.
	Ages []int `json:"ages"`
	// Authors lists the authors. Optional.
	Authors []string `json:"authors,omitempty"`
	// Source names the book or material the cassette is based on. Optional.
	Source string `json:"source,omitempty"`
	// GraderInstructions tell the AI grader how to speak and what to accept
	// for every exercise. Optional.
	GraderInstructions string `json:"grader_instructions,omitempty"`
	// Items is the ordered list of lessons and exercises.
	Items []Item `json:"items"`
}

var manifestSchema = &schema{fields: []field{
	{name: "format", kind: kindInt, required: true},
	{name: "id", kind: kindString, required: true},
	{name: "version", kind: kindString, required: true, check: checkVersion},
	{name: "title", kind: kindString, required: true},
	{name: "description", kind: kindString},
	{name: "language", kind: kindString, required: true},
	{name: "subject", kind: kindString, required: true},
	{name: "ages", kind: kindInts, required: true},
	{name: "authors", kind: kindStrings},
	{name: "source", kind: kindString},
	{name: "grader_instructions", kind: kindString},
	{name: "items", kind: kindObjects, required: true, elem: &schema{fields: []field{
		{name: "type", kind: kindString, required: true},
		{name: "id", kind: kindString, required: true},
	}}},
}}

// checkVersion is the schema check of a version string. It is the only value
// rule applied before decoding, because Version cannot hold an invalid value.
func checkVersion(s string) string {
	if _, err := ParseVersion(s); err != nil {
		return err.Error()
	}
	return ""
}

// validate applies the field rules of CASSETTE-FORMAT §3 and §2, with paths
// relative to cassette.json. The folders of the items are checked by Load.
func (m *Manifest) validate(l *problem.List) {
	checkFormat(l, m.Format)
	if !cassetteIDPattern.MatchString(m.ID) {
		l.Errorf("id", "%q is not a cassette id: reverse-domain style such as org.family-smith.math.fractions-1 (lowercase letters, digits and hyphens, at least two dot-separated parts)", m.ID)
	}
	checkText(l, "title", m.Title, 1, MaxTitleRunes)
	checkText(l, "description", m.Description, 0, MaxDescriptionRunes)
	checkLanguage(l, "language", m.Language)
	checkText(l, "subject", m.Subject, 1, MaxSubjectRunes)
	checkAges(l, "ages", m.Ages)
	checkText(l, "source", m.Source, 0, MaxSourceRunes)
	checkText(l, "grader_instructions", m.GraderInstructions, 0, MaxGraderInstructionsRunes)

	if len(m.Items) == 0 || len(m.Items) > MaxItems {
		l.Errorf("items", "has %d items; it needs 1 to %d", len(m.Items), MaxItems)
	}
	used := make(map[string]int, len(m.Items))
	for i, it := range m.Items {
		path := problem.Index("items", i)
		if it.Type != ItemLesson && it.Type != ItemExercise {
			l.Errorf(path+".type", "%q is not lesson or exercise", string(it.Type))
		}
		switch first, dup := used[it.ID]; {
		case !idPattern.MatchString(it.ID):
			l.Errorf(path+".id", "%q is not an id: 1 to 63 characters, lowercase letters, digits and hyphens, not starting with a hyphen", it.ID)
		case dup:
			l.Errorf(path+".id", "%q is already used by items[%d]; ids are unique across lessons and exercises", it.ID, first)
		default:
			used[it.ID] = i
		}
	}
	if len(m.Items) > 0 && m.Items[0].Type == ItemExercise {
		l.Warnf("items[0]", "the first item should be a lesson")
	}
}

// checkFormat reports a format other than FormatVersion.
func checkFormat(l *problem.List, format int) {
	if format != FormatVersion {
		l.Errorf("format", "is %d; it must be %d", format, FormatVersion)
	}
}

// checkText reports a text whose length in characters (not bytes) is outside
// min to max. With min 1 a blank text (only white space) is also reported.
func checkText(l *problem.List, path, s string, min, max int) {
	n := utf8.RuneCountInString(s)
	switch {
	case min > 0 && strings.TrimSpace(s) == "":
		l.Errorf(path, "must not be empty")
	case n > max:
		l.Errorf(path, "has %d characters; at most %d are allowed", n, max)
	}
}

func checkLanguage(l *problem.List, path, s string) {
	if strings.Contains(s, "_") {
		l.Errorf(path, "%q is not a BCP 47 language tag (use a hyphen, as in en-GB)", s)
		return
	}
	if _, err := language.Parse(s); err != nil {
		l.Errorf(path, "%q is not a BCP 47 language tag such as en, el or en-GB", s)
	}
}

func checkAges(l *problem.List, path string, ages []int) {
	if len(ages) != 2 {
		l.Errorf(path, "must be [min, max] (2 numbers), not %d", len(ages))
		return
	}
	ok := true
	for i, a := range ages {
		if a < MinAge || a > MaxAge {
			l.Errorf(problem.Index(path, i), "%d is not an age from %d to %d", a, MinAge, MaxAge)
			ok = false
		}
	}
	if ok && ages[0] > ages[1] {
		l.Errorf(path, "the minimum age (%d) is above the maximum (%d)", ages[0], ages[1])
	}
}

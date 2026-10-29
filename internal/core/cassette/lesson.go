package cassette

import (
	"github.com/rm4n0s/radio-tutor/internal/core/problem"
	"github.com/rm4n0s/radio-tutor/internal/core/timeline"
)

// MaxSummaryRunes is the most characters of a lesson's summary.
const MaxSummaryRunes = 300

// Lesson is a lesson.json (CASSETTE-FORMAT §5.3): the timeline that animates
// the lesson's scene and the captions. The scene and the narration are other
// files, read by other packages.
type Lesson struct {
	// ID is the lesson's id (its folder name). It is not in the file.
	ID string `json:"-"`
	// Format is the format version, always FormatVersion.
	Format int `json:"format"`
	// Title is the lesson's title.
	Title string `json:"title"`
	// Summary is a short description. Optional.
	Summary string `json:"summary,omitempty"`
	// Tracks animate the actors of the scene; at most one per actor.
	Tracks []timeline.Track `json:"tracks"`
	// Captions are the texts shown while the narration plays.
	Captions []timeline.Caption `json:"captions"`
}

// keySchema describes a key: t is required, every property is optional.
var keySchema = &schema{fields: []field{
	{name: "t", kind: kindNumber, required: true},
	{name: "opacity", kind: kindNumber},
	{name: "x", kind: kindNumber},
	{name: "y", kind: kindNumber},
	{name: "scale", kind: kindNumber},
	{name: "rotate", kind: kindNumber},
	{name: "reveal", kind: kindNumber},
	{name: "ease", kind: kindString},
}}

var trackSchema = &schema{fields: []field{
	{name: "actor", kind: kindString, required: true},
	{name: "origin", kind: kindNumbers},
	{name: "reveal_box", kind: kindNumbers},
	{name: "reveal_from", kind: kindString},
	{name: "keys", kind: kindObjects, required: true, elem: keySchema},
}}

var captionSchema = &schema{fields: []field{
	{name: "start", kind: kindNumber, required: true},
	{name: "end", kind: kindNumber, required: true},
	{name: "text", kind: kindString, required: true},
}}

var lessonSchema = &schema{fields: []field{
	{name: "format", kind: kindInt, required: true},
	{name: "title", kind: kindString, required: true},
	{name: "summary", kind: kindString},
	{name: "tracks", kind: kindObjects, required: true, elem: trackSchema},
	{name: "captions", kind: kindObjects, required: true, elem: captionSchema},
}}

// validate applies the field rules of CASSETTE-FORMAT §5.3 to §5.5, with paths
// relative to lesson.json. The rules that need the narration's length are left
// out (timeline.LengthUnknown); the archive adds timeline.CheckLength.
func (le *Lesson) validate(l *problem.List) {
	checkFormat(l, le.Format)
	checkText(l, "title", le.Title, 1, MaxTitleRunes)
	checkText(l, "summary", le.Summary, 0, MaxSummaryRunes)
	l.Add(timeline.Validate(le.Tracks, le.Captions, timeline.LengthUnknown)...)
}

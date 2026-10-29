package grading

import (
	"bytes"
	_ "embed" // for the system prompt template
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

//go:embed system_prompt.tmpl
var systemPromptSource string

// systemPromptTemplate is read-only after package initialization; executing a
// template is safe for concurrent use.
var systemPromptTemplate = template.Must(template.New("system_prompt").Option("missingkey=error").Parse(systemPromptSource))

// systemPromptData is what the system prompt template is filled with. Both
// values come from the app (ages are integers, the language is a name from a
// table or a quoted tag), never from free text.
type systemPromptData struct {
	// Ages is "8 to 10", or "8" when both ends are equal.
	Ages string
	// Language is the English name of the content language and its tag, such
	// as "Greek (el)".
	Language string
}

// BuildPrompt returns the system message (SPEC §11.3) and the text of the user
// message (SPEC §11.4) for the request. The photos are not part of the text;
// the provider adds them after it. Every value of the cassette in the user
// message is a JSON string, so it cannot break out of its line, and a literal
// "</cassette>" inside a value is neutralized.
func BuildPrompt(req Request) (system, user string) {
	var sb bytes.Buffer
	data := systemPromptData{Ages: agesText(req.Ages), Language: LanguageName(req.Language)}
	if err := systemPromptTemplate.Execute(&sb, data); err != nil {
		// Unreachable: the template only prints two strings (a golden test
		// renders it), so a failure here is a programming error.
		panic("grading: system prompt template: " + err.Error())
	}
	return sb.String(), userMessage(req)
}

// agesText formats the age range for the system prompt.
func agesText(ages [2]int) string {
	if ages[0] == ages[1] {
		return strconv.Itoa(ages[0])
	}
	return strconv.Itoa(ages[0]) + " to " + strconv.Itoa(ages[1])
}

// LanguageName returns the English name of the language with the BCP 47 tag
// followed by the tag in parentheses, such as "Greek (el)" or "British English
// (en-GB)". A tag that cannot be parsed, or that has no name, is returned as a
// JSON string.
func LanguageName(tag string) string {
	t, err := language.Parse(tag)
	if err != nil {
		return quote(tag)
	}
	name := display.English.Languages().Name(t)
	if name == "" || strings.HasPrefix(name, "Unknown") {
		return quote(tag)
	}
	return name + " (" + t.String() + ")"
}

// userMessage renders the exercise data block of SPEC §11.4. Ages, show_work
// and the number of photos come from the app and are written bare; every text
// is JSON-quoted.
func userMessage(req Request) string {
	var b strings.Builder
	b.WriteString("<cassette>\n")
	b.WriteString("title: " + quote(req.Title) + "\n")
	b.WriteString("language: " + quote(req.Language) + "\n")
	b.WriteString("ages: " + strconv.Itoa(req.Ages[0]) + "-" + strconv.Itoa(req.Ages[1]) + "\n")
	b.WriteString("grader_instructions: " + quote(req.GraderInstructions) + "\n")
	b.WriteString("exercises:\n")
	for _, ex := range req.Exercises {
		b.WriteString("- label: " + quote(ex.Label) + "\n")
		b.WriteString("  id: " + quote(ex.ID) + "\n")
		b.WriteString("  title: " + quote(ex.Title) + "\n")
		b.WriteString("  prompt: " + quote(ex.Prompt) + "\n")
		b.WriteString("  reference_solution: " + quote(ex.ReferenceSolution) + "\n")
		b.WriteString("  grading: " + quote(ex.Grading) + "\n")
		b.WriteString("  show_work: " + strconv.FormatBool(ex.ShowWork) + "\n")
	}
	b.WriteString("</cassette>\n")
	b.WriteString("Photos: " + strconv.Itoa(len(req.Photos)) + " (in order).")
	return b.String()
}

// jsonLT is the JSON escape of "<": a backslash, "u003c".
const jsonLT = "\x5c" + "u003c"

// tagLike matches the start of a "<cassette>" or "</cassette>" tag, in any
// case and with white space inside.
var tagLike = regexp.MustCompile(`(?i)<(\s*/?\s*cassette)`)

// quote returns s as a JSON string (not Go syntax: strconv.Quote writes \x
// escapes that JSON does not have), with "<" written as the JSON escape for U+003C where it would
// start a cassette tag. "<", ">" and "&" are otherwise left as they are, so
// that mathematics stays readable. The result is one line.
func quote(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		// Unreachable: a string always encodes (invalid UTF-8 becomes U+FFFD).
		panic("grading: quote: " + err.Error())
	}
	out := strings.TrimSuffix(buf.String(), "\n")
	return tagLike.ReplaceAllString(out, jsonLT+"$1")
}

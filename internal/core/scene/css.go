package scene

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// cssNormalize prepares CSS (a style attribute, the text of a <style> element
// or a presentation attribute) for the forbidden-content scans: comments become
// a space, escapes (\61 or \a) are resolved, and the text is lower-cased, so
// that "anim\61tion" and "ANIMATION" are found like "animation".
func cssNormalize(s string) string {
	if !strings.ContainsAny(s, `\/`) {
		return strings.ToLower(s)
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		switch {
		case strings.HasPrefix(s[i:], "/*"):
			end := strings.Index(s[i+2:], "*/")
			b.WriteByte(' ')
			if end < 0 {
				i = len(s)
			} else {
				i += 2 + end + 2
			}
		case s[i] == '\\':
			i = writeEscape(&b, s, i+1)
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return strings.ToLower(b.String())
}

// writeEscape resolves the escape whose text starts at s[i] (just after the
// backslash), writes the result and returns the index after the escape.
func writeEscape(b *strings.Builder, s string, i int) int {
	if i >= len(s) {
		return i
	}
	j := i
	for j < len(s) && j-i < 6 && isHex(s[j]) {
		j++
	}
	if j > i {
		code, err := strconv.ParseUint(s[i:j], 16, 32)
		r := rune(code)
		if err != nil || r == 0 || !utf8.ValidRune(r) {
			r = utf8.RuneError
		}
		b.WriteRune(r)
		// One white space after a hex escape belongs to the escape.
		if j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\f') {
			j++
		} else if strings.HasPrefix(s[j:], "\r\n") {
			j += 2
		}
		return j
	}
	if s[i] == '\n' {
		return i + 1 // a line continuation
	}
	r, size := utf8.DecodeRuneInString(s[i:])
	b.WriteRune(r)
	return i + size
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

var (
	// animationProperty matches a declaration of an animation or transition
	// property (also the vendor-prefixed and longhand ones). A property name
	// follows the start of the text, "{", ";" or white space, which keeps a
	// selector such as ".transition:hover" from matching.
	animationProperty = regexp.MustCompile(`(^|[{;\s])(-[a-z]+-)?(animation|transition)(-[a-z-]*)?\s*:`)
	// keyframesRule matches @keyframes (also vendor-prefixed).
	keyframesRule = regexp.MustCompile(`@(-[a-z]+-)?keyframes`)
	// verticalWritingMode matches the writing-mode declarations that set
	// vertical text.
	verticalWritingMode = regexp.MustCompile(`writing-mode\s*:\s*(vertical-[a-z]+|tb-rl|tb)\b`)
)

// cssProblems scans normalized CSS and returns a message for each kind of
// forbidden content in it, at most one per kind.
func cssProblems(css string) []string {
	var out []string
	if animationProperty.MatchString(css) {
		out = append(out, "CSS animations and transitions are not allowed; animate with the timeline in lesson.json")
	}
	if keyframesRule.MatchString(css) {
		out = append(out, "@keyframes is not allowed; animate with the timeline in lesson.json")
	}
	if verticalWritingMode.MatchString(css) {
		out = append(out, "vertical text (writing-mode) is not allowed")
	}
	if strings.Contains(css, "@import") {
		out = append(out, "@import is not allowed: a scene cannot load other files")
	}
	if msg := urlProblem(css); msg != "" {
		out = append(out, msg)
	}
	return out
}

// urlProblem scans normalized text for url(...) references and returns a
// message for the first one that points outside the document: only "#id" and
// data:image/png or data:image/jpeg URLs are allowed.
func urlProblem(norm string) string {
	rest := norm
	for {
		i := strings.Index(rest, "url(")
		if i < 0 {
			return ""
		}
		rest = rest[i+len("url("):]
		target := rest
		trimmed := strings.TrimLeft(rest, " \t\r\n\f")
		if trimmed != "" && (trimmed[0] == '"' || trimmed[0] == '\'') {
			q := trimmed[0]
			trimmed = trimmed[1:]
			if end := strings.IndexByte(trimmed, q); end >= 0 {
				target = trimmed[:end]
			} else {
				target = trimmed
			}
		} else if end := strings.IndexByte(rest, ')'); end >= 0 {
			target = rest[:end]
		}
		target = strings.TrimSpace(target)
		if !allowedReference(target) {
			return "url(" + shorten(target) + ") points outside the document; only url(#id) and data:image/png or data:image/jpeg URLs are allowed"
		}
	}
}

// allowedReference reports whether a reference target is a fragment of this
// document ("#id") or a data:image/png or data:image/jpeg URL.
func allowedReference(target string) bool {
	return strings.HasPrefix(target, "#") || isDataImage(target)
}

// isDataImage reports whether s is a data URL of a PNG or JPEG image.
func isDataImage(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	for _, p := range []string{"data:image/png", "data:image/jpeg"} {
		if len(lower) > len(p) && strings.HasPrefix(lower, p) && (lower[len(p)] == ';' || lower[len(p)] == ',') {
			return true
		}
	}
	return false
}

// shorten cuts a long text for a message.
func shorten(s string) string {
	const max = 40
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "..."
}

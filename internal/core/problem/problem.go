// Package problem is the small shared vocabulary of every validator of the
// core (cassette, scene, timeline, audio, archive): a Problem says where
// something is wrong and what is wrong, and is printed as "path: message"
// (SPEC §6, CASSETTE-FORMAT §10).
//
// Validators collect every problem they find instead of stopping at the first,
// so they return a []Problem (or a List, which is the same slice with adders).
// A Problem is either an error or a warning: a cassette is valid when it has
// no errors, and warnings are shown but do not fail it.
//
// Paths are built by the validator that finds the problem, relative to the
// thing it validates (for example "tracks[2].keys[5].opacity" or
// "captions[1].end"). The caller that knows which file the thing came from
// prefixes them with InFile (for example
// "lessons/l1/lesson.json#tracks[2].keys[5].opacity"), and a caller that
// nests validators prefixes them with Prefixed.
package problem

import (
	"fmt"
	"strings"
)

// Problem is one finding of a validator.
type Problem struct {
	// Path says where the problem is: a file, a field path, or both (see the
	// package documentation). It may be empty for a problem about the whole
	// thing that was validated.
	Path string `json:"path"`
	// Message says what is wrong, in plain words for a cassette author.
	Message string `json:"message"`
	// Warning is true for a finding that does not make the cassette invalid.
	Warning bool `json:"warning,omitempty"`
}

// String formats the problem as "path: message" (just the message when the
// path is empty). A warning is prefixed with "warning: " so that it is told
// apart in plain text lists.
func (p Problem) String() string {
	s := p.Message
	if p.Path != "" {
		s = p.Path + ": " + p.Message
	}
	if p.Warning {
		return "warning: " + s
	}
	return s
}

// Errorf returns an error Problem with a formatted message.
func Errorf(path, format string, args ...any) Problem {
	return Problem{Path: path, Message: fmt.Sprintf(format, args...)}
}

// Warnf returns a warning Problem with a formatted message.
func Warnf(path, format string, args ...any) Problem {
	return Problem{Path: path, Message: fmt.Sprintf(format, args...), Warning: true}
}

// Join appends the field path child to the path parent: they are separated by
// a dot, except that an index such as "[2]" is appended directly and an empty
// side is dropped. Join("tracks", "[2]") is "tracks[2]" and
// Join("tracks[2]", "keys") is "tracks[2].keys".
func Join(parent, child string) string {
	switch {
	case parent == "":
		return child
	case child == "":
		return parent
	case strings.HasPrefix(child, "["):
		return parent + child
	default:
		return parent + "." + child
	}
}

// Index returns the path of element i of the list at path parent, for example
// Index("tracks", 2) is "tracks[2]".
func Index(parent string, i int) string {
	return fmt.Sprintf("%s[%d]", parent, i)
}

// List is a slice of Problems with the adders validators use. The zero value is
// an empty list that is ready to use. A List is a []Problem, so it is returned
// from functions declared to return []Problem without conversion.
type List []Problem

// Add appends problems to the list.
func (l *List) Add(ps ...Problem) {
	*l = append(*l, ps...)
}

// Errorf appends an error with a formatted message.
func (l *List) Errorf(path, format string, args ...any) {
	l.Add(Errorf(path, format, args...))
}

// Warnf appends a warning with a formatted message.
func (l *List) Warnf(path, format string, args ...any) {
	l.Add(Warnf(path, format, args...))
}

// HasErrors reports whether the list holds at least one error (a warning does
// not count).
func (l List) HasErrors() bool {
	return HasErrors(l)
}

// HasErrors reports whether ps holds at least one error (a warning does not
// count). A cassette is valid exactly when this is false.
func HasErrors(ps []Problem) bool {
	for _, p := range ps {
		if !p.Warning {
			return true
		}
	}
	return false
}

// Errors returns the errors of ps, in order, as a new slice.
func Errors(ps []Problem) []Problem {
	return filter(ps, false)
}

// Warnings returns the warnings of ps, in order, as a new slice.
func Warnings(ps []Problem) []Problem {
	return filter(ps, true)
}

func filter(ps []Problem, warning bool) []Problem {
	var out []Problem
	for _, p := range ps {
		if p.Warning == warning {
			out = append(out, p)
		}
	}
	return out
}

// Prefixed returns a copy of ps in which every path is prefixed with prefix
// using Join, for a caller that validates a part of a bigger thing (the
// lesson's timeline inside a lesson, say). ps is not changed.
func Prefixed(prefix string, ps []Problem) []Problem {
	out := make([]Problem, len(ps))
	for i, p := range ps {
		p.Path = Join(prefix, p.Path)
		out[i] = p
	}
	return out
}

// InFile returns a copy of ps in which every path is prefixed with the name of
// the file the validated thing was read from, separated by "#": the path
// "tracks[2].keys[5].opacity" of "lessons/l1/lesson.json" becomes
// "lessons/l1/lesson.json#tracks[2].keys[5].opacity". A problem with an empty
// path gets the file name alone. ps is not changed.
func InFile(file string, ps []Problem) []Problem {
	out := make([]Problem, len(ps))
	for i, p := range ps {
		if p.Path == "" {
			p.Path = file
		} else {
			p.Path = file + "#" + p.Path
		}
		out[i] = p
	}
	return out
}

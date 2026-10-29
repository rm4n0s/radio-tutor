package timeline

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/rm4n0s/radio-tutor/internal/core/problem"
)

// The limits of CASSETTE-FORMAT §5.3 to §5.5.
const (
	// MaxKeys is the most keys a track may have.
	MaxKeys = 1000
	// MaxScale is the largest value of the scale property.
	MaxScale = 20
	// MaxCaptionRunes is the most characters a caption's text may have.
	MaxCaptionRunes = 200
	// MaxCaptionLines is the most lines a caption's text may have.
	MaxCaptionLines = 2
	// LengthTolerance is how many seconds a key's t or a caption's end may be
	// past the narration's length (CASSETTE-FORMAT §6).
	LengthTolerance = 0.05
)

// LengthUnknown is the narration length to pass to Validate when the audio has
// not been read yet: the rules that compare times with the length are skipped.
// Any length that is not above 0 means the same (a narration is never empty).
const LengthUnknown = 0

// Validate checks the rules of CASSETTE-FORMAT §5.3 to §5.5 that need nothing
// but the timeline and the narration's length (in seconds), and returns every
// problem it finds (all errors), with paths such as "tracks[2].keys[5].opacity"
// and "captions[1].end".
//
// With length LengthUnknown (or any length that is not above 0) the rules that
// compare a time with the length are skipped. The cassette package validates a
// lesson that way, because the narration is read later; the caller that knows
// the length then adds CheckLength.
//
// Rules for tracks: the actor is not empty and has at most one track; origin
// has two numbers and is present when a key sets scale or rotate; reveal_box
// has four numbers (positive width and height) and is present when a key sets
// reveal; reveal_from is one of the sides; there are 1 to MaxKeys keys with
// strictly increasing t, each at least 0 and at most length + LengthTolerance;
// opacity and reveal are within 0 to 1, scale is above 0 and at most MaxScale,
// x, y and rotate are finite; ease is one of the easings.
//
// Rules for captions: at least one; text of 1 to MaxCaptionRunes characters
// and at most MaxCaptionLines lines; start at least 0 and before end; end at
// most length + LengthTolerance; in order and not overlapping (a caption may
// start exactly when the previous one ends).
//
// That the actor exists in the scene and that reveal_box lies inside the view
// box need the scene and are checked by the scene package.
func Validate(tracks []Track, captions []Caption, length float64) []problem.Problem {
	var l problem.List
	limit := limitOf(length)

	seen := make(map[string]int, len(tracks))
	for i, tr := range tracks {
		validateTrack(&l, problem.Index("tracks", i), tr, limit)
		if tr.Actor != "" {
			if first, dup := seen[tr.Actor]; dup {
				l.Errorf(problem.Index("tracks", i)+".actor", "actor %q already has track %d; use one track per actor", tr.Actor, first)
			} else {
				seen[tr.Actor] = i
			}
		}
	}

	validateCaptions(&l, captions, limit)
	return l
}

// CheckLength checks only the rules of Validate that compare a time with the
// narration's length: every key's t and every caption's end is at most length +
// LengthTolerance. Its problems have the same paths and messages as Validate's,
// so Validate(..., LengthUnknown) followed by CheckLength reports exactly what
// Validate(..., length) does. With an unknown length it reports nothing.
func CheckLength(tracks []Track, captions []Caption, length float64) []problem.Problem {
	var l problem.List
	limit := limitOf(length)
	for i, tr := range tracks {
		for j, k := range tr.Keys {
			if finite(k.T) && k.T > limit {
				l.Errorf(problem.Index(problem.Index("tracks", i)+".keys", j)+".t", "%s is after the end of the narration (%s)", num(k.T), num(limit-LengthTolerance))
			}
		}
	}
	for i, c := range captions {
		if finite(c.End) && c.End > limit {
			l.Errorf(problem.Index("captions", i)+".end", "%s is after the end of the narration (%s)", num(c.End), num(limit-LengthTolerance))
		}
	}
	return l
}

// limitOf returns the latest time a key or a caption end may have for a
// narration of the given length: +Inf when the length is unknown.
func limitOf(length float64) float64 {
	if !(length > 0) {
		return math.Inf(1)
	}
	return length + LengthTolerance
}

func validateTrack(l *problem.List, path string, tr Track, limit float64) {
	if tr.Actor == "" {
		l.Errorf(path+".actor", "is required")
	}
	if !tr.RevealFrom.Valid() {
		l.Errorf(path+".reveal_from", "%q is not left, right, top or bottom", string(tr.RevealFrom))
	}

	if tr.Origin != nil {
		checkNumbers(l, path+".origin", tr.Origin, 2, "[x, y]")
	}
	if tr.RevealBox != nil && checkNumbers(l, path+".reveal_box", tr.RevealBox, 4, "[x, y, width, height]") &&
		(tr.RevealBox[2] <= 0 || tr.RevealBox[3] <= 0) {
		l.Errorf(path+".reveal_box", "width and height must be greater than 0")
	}

	if len(tr.Keys) == 0 {
		l.Errorf(path+".keys", "needs at least one key")
	}
	if len(tr.Keys) > MaxKeys {
		l.Errorf(path+".keys", "has %d keys; at most %d are allowed", len(tr.Keys), MaxKeys)
	}

	needsOrigin, needsBox := false, false
	for i, k := range tr.Keys {
		kp := path + ".keys" + problem.Index("", i)
		validateKey(l, kp, k, limit)
		if i > 0 && finite(k.T) && finite(tr.Keys[i-1].T) && k.T <= tr.Keys[i-1].T {
			l.Errorf(kp+".t", "%s is not after the previous key's t (%s); keys must be in strictly increasing order", num(k.T), num(tr.Keys[i-1].T))
		}
		needsOrigin = needsOrigin || k.Scale != nil || k.Rotate != nil
		needsBox = needsBox || k.Reveal != nil
	}
	if needsOrigin && tr.Origin == nil {
		l.Errorf(path+".origin", "is required when a key sets scale or rotate")
	}
	if needsBox && tr.RevealBox == nil {
		l.Errorf(path+".reveal_box", "is required when a key sets reveal")
	}
}

func validateKey(l *problem.List, path string, k Key, limit float64) {
	switch {
	case !finite(k.T):
		l.Errorf(path+".t", "must be a finite number")
	case k.T < 0:
		l.Errorf(path+".t", "%s is negative", num(k.T))
	case k.T > limit:
		l.Errorf(path+".t", "%s is after the end of the narration (%s)", num(k.T), num(limit-LengthTolerance))
	}

	checkRange(l, path+".opacity", k.Opacity, 0, 1)
	checkRange(l, path+".reveal", k.Reveal, 0, 1)
	if v := k.Scale; v != nil && (!finite(*v) || *v <= 0 || *v > MaxScale) {
		l.Errorf(path+".scale", "%s must be greater than 0 and at most %d", num(*v), MaxScale)
	}
	for _, f := range []struct {
		name string
		v    *float64
	}{{"x", k.X}, {"y", k.Y}, {"rotate", k.Rotate}} {
		if f.v != nil && !finite(*f.v) {
			l.Errorf(path+"."+f.name, "must be a finite number")
		}
	}
	if !k.Ease.Valid() {
		l.Errorf(path+".ease", "%q is not linear, in, out, in-out or step", string(k.Ease))
	}
}

func validateCaptions(l *problem.List, captions []Caption, limit float64) {
	if len(captions) == 0 {
		l.Errorf("captions", "needs at least one caption")
	}
	for i, c := range captions {
		path := problem.Index("captions", i)
		validateCaption(l, path, c, limit)
		if i == 0 {
			continue
		}
		prev := captions[i-1]
		switch {
		case !finite(c.Start) || !finite(prev.End):
		case c.Start < prev.Start:
			l.Errorf(path+".start", "%s is before the previous caption's start (%s); captions must be in order", num(c.Start), num(prev.Start))
		case c.Start < prev.End:
			l.Errorf(path+".start", "%s is before the previous caption's end (%s); captions must not overlap", num(c.Start), num(prev.End))
		}
	}
}

func validateCaption(l *problem.List, path string, c Caption, limit float64) {
	switch n := utf8.RuneCountInString(c.Text); {
	case strings.TrimSpace(c.Text) == "":
		l.Errorf(path+".text", "must not be empty")
	case n > MaxCaptionRunes:
		l.Errorf(path+".text", "has %d characters; at most %d are allowed", n, MaxCaptionRunes)
	case strings.Count(c.Text, "\n")+1 > MaxCaptionLines:
		l.Errorf(path+".text", "has %d lines; at most %d are allowed", strings.Count(c.Text, "\n")+1, MaxCaptionLines)
	}

	if !finite(c.Start) || c.Start < 0 {
		l.Errorf(path+".start", "must be a number that is at least 0")
	}
	switch {
	case !finite(c.End):
		l.Errorf(path+".end", "must be a finite number")
	case finite(c.Start) && c.End <= c.Start:
		l.Errorf(path+".end", "%s is not after the start (%s)", num(c.End), num(c.Start))
	case c.End > limit:
		l.Errorf(path+".end", "%s is after the end of the narration (%s)", num(c.End), num(limit-LengthTolerance))
	}
}

// checkNumbers reports a problem when v is not want finite numbers and returns
// whether it is.
func checkNumbers(l *problem.List, path string, v []float64, want int, shape string) bool {
	if len(v) != want {
		l.Errorf(path, "must be %s (%d numbers), not %d", shape, want, len(v))
		return false
	}
	for _, x := range v {
		if !finite(x) {
			l.Errorf(path, "must hold finite numbers")
			return false
		}
	}
	return true
}

func checkRange(l *problem.List, path string, v *float64, lo, hi float64) {
	if v != nil && (!finite(*v) || *v < lo || *v > hi) {
		l.Errorf(path, "%s must be between %s and %s", num(*v), num(lo), num(hi))
	}
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// num formats a number for a message without a trailing ".000000".
func num(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

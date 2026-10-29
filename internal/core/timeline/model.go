// Package timeline is the animation model of a lesson (CASSETTE-FORMAT §5.3 to
// §5.5, SPEC §8): the tracks that animate the actors of a scene, the captions,
// the evaluation of the tracks at a time t, and the validation of the rules
// that need nothing but the timeline itself.
//
// The types carry JSON tags but this package does not read JSON: the cassette
// package decodes lesson.json (rejecting unknown fields) into them. The rules
// that need the scene (the actor exists, reveal_box inside the view box) or
// the audio (the narration's length, passed to Validate) live in the packages
// that own those; this package never imports them.
//
// Required numbers (Key.T, Caption.Start, Caption.End) are plain float64:
// encoding/json cannot tell an absent field from 0 for them, so a lesson that
// leaves out "start" or the first "t" is read as 0. The optional key
// properties are pointers so that "not set" and 0 differ.
package timeline

// Ease is how the properties a key sets arrive from the previous key that set
// them (CASSETTE-FORMAT §5.4). The empty Ease means EaseLinear.
type Ease string

// The easings of CASSETTE-FORMAT §5.4.
const (
	// EaseLinear moves at a constant speed. It is the default.
	EaseLinear Ease = "linear"
	// EaseIn starts slowly (cubic).
	EaseIn Ease = "in"
	// EaseOut ends slowly (cubic).
	EaseOut Ease = "out"
	// EaseInOut starts and ends slowly (cubic).
	EaseInOut Ease = "in-out"
	// EaseStep keeps the previous value until the key's time and jumps to the
	// key's value there.
	EaseStep Ease = "step"
)

// Valid reports whether e is one of the easings or empty (the default).
func (e Ease) Valid() bool {
	switch e {
	case "", EaseLinear, EaseIn, EaseOut, EaseInOut, EaseStep:
		return true
	}
	return false
}

// RevealFrom is the side of a track's reveal_box a reveal starts from. The
// empty RevealFrom means RevealLeft.
type RevealFrom string

// The sides of CASSETTE-FORMAT §5.4.
const (
	// RevealLeft reveals from the left side. It is the default.
	RevealLeft RevealFrom = "left"
	// RevealRight reveals from the right side.
	RevealRight RevealFrom = "right"
	// RevealTop reveals from the top.
	RevealTop RevealFrom = "top"
	// RevealBottom reveals from the bottom.
	RevealBottom RevealFrom = "bottom"
)

// Valid reports whether r is one of the sides or empty (the default).
func (r RevealFrom) Valid() bool {
	switch r {
	case "", RevealLeft, RevealRight, RevealTop, RevealBottom:
		return true
	}
	return false
}

// Or returns r, or RevealLeft when r is empty.
func (r RevealFrom) Or() RevealFrom {
	if r == "" {
		return RevealLeft
	}
	return r
}

// Track animates one actor, a top-level group of the scene.
type Track struct {
	// Actor is the id of the actor.
	Actor string `json:"actor"`
	// Origin is [x, y] in view box units: the point the actor rotates and
	// scales around. Required when a key sets scale or rotate.
	Origin []float64 `json:"origin,omitempty"`
	// RevealBox is [x, y, width, height] in view box units: the area the
	// reveal wipes over. Required when a key sets reveal.
	RevealBox []float64 `json:"reveal_box,omitempty"`
	// RevealFrom is the side the reveal starts from. Empty means left.
	RevealFrom RevealFrom `json:"reveal_from,omitempty"`
	// Keys are the keyframes, with strictly increasing T.
	Keys []Key `json:"keys"`
}

// Key is a keyframe of a Track: a time and the properties that have a value
// from then on. A nil property is not set by this key.
type Key struct {
	// T is the time in seconds.
	T float64 `json:"t"`
	// Opacity is 0 (invisible) to 1 (opaque). Default 1.
	Opacity *float64 `json:"opacity,omitempty"`
	// X moves the actor right by this many view box units. Default 0.
	X *float64 `json:"x,omitempty"`
	// Y moves the actor down by this many view box units. Default 0.
	Y *float64 `json:"y,omitempty"`
	// Scale scales uniformly around the origin; greater than 0, at most 20.
	// Default 1.
	Scale *float64 `json:"scale,omitempty"`
	// Rotate turns clockwise, in degrees, around the origin. Default 0.
	Rotate *float64 `json:"rotate,omitempty"`
	// Reveal is the fraction of the reveal box shown, 0 to 1. Default 1.
	Reveal *float64 `json:"reveal,omitempty"`
	// Ease is how the properties this key sets arrive from the previous key
	// that set them. Empty means linear.
	Ease Ease `json:"ease,omitempty"`
}

// Caption is a text shown from Start (inclusive) to End (exclusive), in
// seconds.
type Caption struct {
	// Start is the time the caption appears.
	Start float64 `json:"start"`
	// End is the time the caption disappears.
	End float64 `json:"end"`
	// Text is the caption, at most two lines.
	Text string `json:"text"`
}

// Float returns a pointer to v, for building keys: Key{Opacity: Float(0)}.
func Float(v float64) *float64 {
	return &v
}

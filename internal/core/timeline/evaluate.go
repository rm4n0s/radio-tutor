package timeline

// ActorState is what the player applies to an actor at one moment: opacity,
// a translation, a uniform scale and a rotation (degrees, clockwise) around
// the track's origin, and the fraction of the reveal box shown. The transforms
// apply in this order: scale and rotate around the origin, then move by X, Y.
type ActorState struct {
	Opacity float64
	X, Y    float64
	Scale   float64
	Rotate  float64
	Reveal  float64
}

// DefaultState is the state of an actor no key sets any property of:
// opaque, not moved, scale 1, not rotated, fully revealed.
func DefaultState() ActorState {
	return ActorState{Opacity: 1, Scale: 1, Reveal: 1}
}

// The properties of a key, in the order of ActorState.
const (
	propOpacity = iota
	propX
	propY
	propScale
	propRotate
	propReveal
	propCount
)

var defaultValues = [propCount]float64{propOpacity: 1, propScale: 1, propReveal: 1}

// point is one keyframe of one property: the key's time, the value it sets
// and the key's ease.
type point struct {
	t, v float64
	ease Ease
}

type compiledTrack struct {
	actor string
	props [propCount][]point
}

// Evaluator evaluates the tracks of a lesson at any time t. It is built once
// (NewEvaluator) and then asked for the states of all the actors every frame
// without allocating: the keys are split per property when it is built, and
// the state of a frame is written into a slice the caller owns and reuses.
// This matters because the player evaluates up to 64 actors at 60 frames per
// second.
//
// An Evaluator is immutable after NewEvaluator, so it can be shared by
// goroutines as long as each uses its own destination slice. It expects
// validated tracks (keys with strictly increasing t, see Validate); with
// unordered keys the result is unspecified but never panics.
type Evaluator struct {
	tracks []compiledTrack
}

// NewEvaluator builds an Evaluator for tracks. The result is independent of
// tracks: changing them afterwards does not change the Evaluator.
func NewEvaluator(tracks []Track) *Evaluator {
	e := &Evaluator{tracks: make([]compiledTrack, len(tracks))}
	for i, tr := range tracks {
		ct := &e.tracks[i]
		ct.actor = tr.Actor
		for _, k := range tr.Keys {
			add := func(prop int, v *float64) {
				if v != nil {
					ct.props[prop] = append(ct.props[prop], point{t: k.T, v: *v, ease: k.Ease})
				}
			}
			add(propOpacity, k.Opacity)
			add(propX, k.X)
			add(propY, k.Y)
			add(propScale, k.Scale)
			add(propRotate, k.Rotate)
			add(propReveal, k.Reveal)
		}
	}
	return e
}

// Len returns the number of tracks, which is the number of states Evaluate
// writes.
func (e *Evaluator) Len() int {
	return len(e.tracks)
}

// Actor returns the actor id of track i.
func (e *Evaluator) Actor(i int) string {
	return e.tracks[i].actor
}

// Evaluate writes the state of every track at time t (seconds) into dst, in
// track order, and returns dst[:Len()]. When dst is too short a new slice is
// allocated; a caller that keeps a slice of Len() states and passes it every
// frame never allocates.
//
// Each property is animated on its own (CASSETTE-FORMAT §5.4): before the
// first key that sets it, it has that key's value; between two keys that set
// it, it moves from the first value to the second with the second key's ease;
// after the last it keeps the last value; a property no key sets has its
// default.
func (e *Evaluator) Evaluate(t float64, dst []ActorState) []ActorState {
	if cap(dst) < len(e.tracks) {
		dst = make([]ActorState, len(e.tracks))
	}
	dst = dst[:len(e.tracks)]
	for i := range e.tracks {
		p := &e.tracks[i].props
		dst[i] = ActorState{
			Opacity: sample(p[propOpacity], t, defaultValues[propOpacity]),
			X:       sample(p[propX], t, defaultValues[propX]),
			Y:       sample(p[propY], t, defaultValues[propY]),
			Scale:   sample(p[propScale], t, defaultValues[propScale]),
			Rotate:  sample(p[propRotate], t, defaultValues[propRotate]),
			Reveal:  sample(p[propReveal], t, defaultValues[propReveal]),
		}
	}
	return dst
}

// Evaluate is a convenience for one-off use (tests, tools): it builds an
// Evaluator for tracks and returns the states at t in track order. The player
// builds an Evaluator once and reuses it.
func Evaluate(tracks []Track, t float64) []ActorState {
	return NewEvaluator(tracks).Evaluate(t, nil)
}

// sample returns the value of one property at time t.
func sample(pts []point, t, def float64) float64 {
	n := len(pts)
	if n == 0 {
		return def
	}
	if t <= pts[0].t {
		return pts[0].v
	}
	if t >= pts[n-1].t {
		return pts[n-1].v
	}
	// Binary search for the first point after t; it exists and is not the
	// first because of the two checks above.
	lo, hi := 1, n-1
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if pts[mid].t > t {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	a, b := pts[lo-1], pts[lo]
	span := b.t - a.t
	if span <= 0 {
		return b.v
	}
	return a.v + (b.v-a.v)*b.ease.Apply((t-a.t)/span)
}

// CaptionAt returns the caption shown at time t: the one with
// Start <= t < End. captions must be in order and not overlapping (see
// Validate); the search is a binary search. The second result is false when
// no caption is shown at t.
func CaptionAt(captions []Caption, t float64) (Caption, bool) {
	lo, hi := 0, len(captions)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if captions[mid].Start > t {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	// lo is the first caption starting after t; the candidate is before it.
	if lo == 0 {
		return Caption{}, false
	}
	c := captions[lo-1]
	if t < c.End {
		return c, true
	}
	return Caption{}, false
}

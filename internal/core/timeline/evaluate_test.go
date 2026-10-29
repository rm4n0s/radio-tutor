package timeline

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func statesNear(a, b ActorState) bool {
	return near(a.Opacity, b.Opacity) && near(a.X, b.X) && near(a.Y, b.Y) &&
		near(a.Scale, b.Scale) && near(a.Rotate, b.Rotate) && near(a.Reveal, b.Reveal)
}

func TestEaseApply(t *testing.T) {
	tests := []struct {
		name string
		ease Ease
		u    float64
		want float64
	}{
		{"empty is linear", "", 0.3, 0.3},
		{"linear", EaseLinear, 0.25, 0.25},
		{"unknown is linear", Ease("zigzag"), 0.6, 0.6},
		{"in start", EaseIn, 0, 0},
		{"in half", EaseIn, 0.5, 0.125},
		{"in end", EaseIn, 1, 1},
		{"out half", EaseOut, 0.5, 0.875},
		{"out quarter", EaseOut, 0.25, 1 - 0.75*0.75*0.75},
		{"in-out first half", EaseInOut, 0.25, 4 * 0.25 * 0.25 * 0.25},
		{"in-out middle", EaseInOut, 0.5, 0.5},
		{"in-out second half", EaseInOut, 0.75, 0.9375},
		{"step before", EaseStep, 0.999, 0},
		{"step at end", EaseStep, 1, 1},
		{"below zero is clamped", EaseOut, -3, 0},
		{"above one is clamped", EaseIn, 3, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ease.Apply(tt.u)
			if !near(got, tt.want) {
				t.Errorf("%q.Apply(%v) = %v, want %v", tt.ease, tt.u, got, tt.want)
			}
		})
	}
}

func TestEaseMonotonic(t *testing.T) {
	for _, e := range []Ease{EaseLinear, EaseIn, EaseOut, EaseInOut, EaseStep} {
		prev := e.Apply(0)
		for i := 1; i <= 100; i++ {
			v := e.Apply(float64(i) / 100)
			if v < prev {
				t.Errorf("%q is not monotonic at %d: %v < %v", e, i, v, prev)
			}
			prev = v
		}
	}
}

func TestEvaluate(t *testing.T) {
	def := DefaultState()
	tests := []struct {
		name   string
		tracks []Track
		t      float64
		want   []ActorState
	}{
		{"no tracks", nil, 3, []ActorState{}},
		{"track without keys setting anything", []Track{{Actor: "a", Keys: []Key{{T: 1}}}}, 5, []ActorState{def}},
		{
			"before the first key",
			[]Track{{Actor: "a", Keys: []Key{{T: 2, Opacity: Float(0.2)}, {T: 4, Opacity: Float(1)}}}},
			0.5,
			[]ActorState{{Opacity: 0.2, Scale: 1, Reveal: 1}},
		},
		{
			"exactly at the first key",
			[]Track{{Actor: "a", Keys: []Key{{T: 2, Opacity: Float(0.2)}, {T: 4, Opacity: Float(1)}}}},
			2,
			[]ActorState{{Opacity: 0.2, Scale: 1, Reveal: 1}},
		},
		{
			"linear between keys",
			[]Track{{Actor: "a", Keys: []Key{{T: 2, Opacity: Float(0)}, {T: 4, Opacity: Float(1)}}}},
			3,
			[]ActorState{{Opacity: 0.5, Scale: 1, Reveal: 1}},
		},
		{
			"ease of the later key: in",
			[]Track{{Actor: "a", Keys: []Key{{T: 0, X: Float(0)}, {T: 2, X: Float(80), Ease: EaseIn}}}},
			1,
			[]ActorState{{Opacity: 1, X: 10, Scale: 1, Reveal: 1}},
		},
		{
			"ease of the later key: out",
			[]Track{{Actor: "a", Keys: []Key{{T: 0, X: Float(0)}, {T: 2, X: Float(80), Ease: EaseOut}}}},
			1,
			[]ActorState{{Opacity: 1, X: 70, Scale: 1, Reveal: 1}},
		},
		{
			"ease of the earlier key is not used",
			[]Track{{Actor: "a", Keys: []Key{{T: 0, X: Float(0), Ease: EaseStep}, {T: 2, X: Float(80)}}}},
			1,
			[]ActorState{{Opacity: 1, X: 40, Scale: 1, Reveal: 1}},
		},
		{
			"in-out",
			[]Track{{Actor: "a", Keys: []Key{{T: 0, Y: Float(0)}, {T: 4, Y: Float(100), Ease: EaseInOut}}}},
			1,
			[]ActorState{{Opacity: 1, Y: 100 * 4 * 0.25 * 0.25 * 0.25, Scale: 1, Reveal: 1}},
		},
		{
			"step holds the old value before the key",
			[]Track{{Actor: "a", Keys: []Key{{T: 0, Opacity: Float(0)}, {T: 5, Opacity: Float(1), Ease: EaseStep}}}},
			4.999,
			[]ActorState{{Opacity: 0, Scale: 1, Reveal: 1}},
		},
		{
			"step jumps at the key",
			[]Track{{Actor: "a", Keys: []Key{{T: 0, Opacity: Float(0)}, {T: 5, Opacity: Float(1), Ease: EaseStep}}}},
			5,
			[]ActorState{{Opacity: 1, Scale: 1, Reveal: 1}},
		},
		{
			"after the last key",
			[]Track{{Actor: "a", Keys: []Key{{T: 0, Rotate: Float(0)}, {T: 2, Rotate: Float(15), Ease: EaseInOut}}}},
			99,
			[]ActorState{{Opacity: 1, Rotate: 15, Scale: 1, Reveal: 1}},
		},
		{
			"properties set by different keys are independent",
			[]Track{{Actor: "a", Keys: []Key{
				{T: 0, Opacity: Float(0)},
				{T: 2, X: Float(100)},
				{T: 4, Opacity: Float(1), X: Float(200)},
				{T: 8, Scale: Float(3)},
			}}},
			3,
			// opacity: 0 -> 1 between t=0 and t=4 (0.75); x: 100 -> 200 between 2 and 4 (150);
			// scale is only set by the last key, so it has that value before it.
			[]ActorState{{Opacity: 0.75, X: 150, Scale: 3, Reveal: 1}},
		},
		{
			"a key without a property does not interrupt the interpolation",
			[]Track{{Actor: "a", Keys: []Key{{T: 0, Opacity: Float(0)}, {T: 1, X: Float(5)}, {T: 4, Opacity: Float(1)}}}},
			2,
			[]ActorState{{Opacity: 0.5, X: 5, Scale: 1, Reveal: 1}},
		},
		{
			"zero is a value, not absent",
			[]Track{{Actor: "a", Keys: []Key{{T: 0, Opacity: Float(0), Scale: Float(2)}}}},
			1,
			[]ActorState{{Opacity: 0, Scale: 2, Reveal: 1}},
		},
		{
			"several tracks in track order",
			[]Track{
				{Actor: "a", Keys: []Key{{T: 0, Reveal: Float(0)}, {T: 2, Reveal: Float(1)}}},
				{Actor: "b", Keys: []Key{{T: 1, Opacity: Float(0.5)}}},
			},
			1,
			[]ActorState{{Opacity: 1, Scale: 1, Reveal: 0.5}, {Opacity: 0.5, Scale: 1, Reveal: 1}},
		},
		{
			"all six properties",
			[]Track{{Actor: "a", Keys: []Key{
				{T: 0, Opacity: Float(0), X: Float(0), Y: Float(0), Scale: Float(1), Rotate: Float(0), Reveal: Float(0)},
				{T: 2, Opacity: Float(1), X: Float(10), Y: Float(20), Scale: Float(3), Rotate: Float(90), Reveal: Float(1)},
			}}},
			1,
			[]ActorState{{Opacity: 0.5, X: 5, Y: 10, Scale: 2, Rotate: 45, Reveal: 0.5}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(tt.tracks, tt.t)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d states, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if !statesNear(got[i], tt.want[i]) {
					t.Errorf("state %d at t=%v = %+v, want %+v", i, tt.t, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestEvaluatorReusesDestination(t *testing.T) {
	tracks := []Track{
		{Actor: "a", Keys: []Key{{T: 0, Opacity: Float(0)}, {T: 10, Opacity: Float(1)}}},
		{Actor: "b", Keys: []Key{{T: 0, X: Float(0)}, {T: 10, X: Float(100)}}},
	}
	ev := NewEvaluator(tracks)
	if ev.Len() != 2 || ev.Actor(0) != "a" || ev.Actor(1) != "b" {
		t.Fatalf("Len/Actor wrong: %d %q %q", ev.Len(), ev.Actor(0), ev.Actor(1))
	}

	// The evaluator is a snapshot: changing the tracks afterwards changes nothing.
	tracks[0].Keys[1].Opacity = Float(0.1)

	dst := make([]ActorState, ev.Len())
	allocs := testing.AllocsPerRun(100, func() {
		dst = ev.Evaluate(5, dst)
	})
	if allocs != 0 {
		t.Errorf("Evaluate allocated %v times with a big enough destination", allocs)
	}
	if !near(dst[0].Opacity, 0.5) || !near(dst[1].X, 50) {
		t.Errorf("states = %+v", dst)
	}

	// A too short destination is replaced; a longer one is cut.
	if got := ev.Evaluate(5, nil); len(got) != 2 {
		t.Errorf("nil destination gave %d states", len(got))
	}
	if got := ev.Evaluate(5, make([]ActorState, 5)); len(got) != 2 {
		t.Errorf("long destination gave %d states", len(got))
	}
}

func TestEvaluateManyKeys(t *testing.T) {
	// The binary search must find the right segment among many keys.
	keys := make([]Key, 1000)
	for i := range keys {
		keys[i] = Key{T: float64(i), X: Float(float64(i * 2))}
	}
	ev := NewEvaluator([]Track{{Actor: "a", Keys: keys}})
	for _, at := range []float64{0, 0.5, 1, 499.25, 998.5, 999, 1500} {
		want := math.Min(at, 999) * 2
		if got := ev.Evaluate(at, nil)[0].X; !near(got, want) {
			t.Errorf("X at %v = %v, want %v", at, got, want)
		}
	}
}

func TestEvaluateUnorderedKeysDoNotPanic(t *testing.T) {
	tracks := []Track{{Actor: "a", Keys: []Key{
		{T: 5, X: Float(1)}, {T: 5, X: Float(2), Ease: EaseIn}, {T: 1, X: Float(3)},
	}}}
	for _, at := range []float64{0, 1, 3, 5, 6} {
		_ = Evaluate(tracks, at)
	}
}

func TestCaptionAt(t *testing.T) {
	caps := []Caption{
		{Start: 0, End: 5.6, Text: "one"},
		{Start: 5.8, End: 8.6, Text: "two"},
		{Start: 8.6, End: 12, Text: "three"},
	}
	tests := []struct {
		name     string
		captions []Caption
		t        float64
		want     string
		ok       bool
	}{
		{"no captions", nil, 1, "", false},
		{"before everything", []Caption{{Start: 1, End: 2, Text: "x"}}, 0.5, "", false},
		{"at the first start", caps, 0, "one", true},
		{"inside", caps, 3, "one", true},
		{"just before the end", caps, 5.599, "one", true},
		{"at an end (exclusive)", caps[:2], 5.6, "", false},
		{"in a gap", caps, 5.7, "", false},
		{"at a start after a gap", caps, 5.8, "two", true},
		{"adjacent captions: the next one wins at the boundary", caps, 8.6, "three", true},
		{"just before the boundary", caps, 8.599, "two", true},
		{"at the last end", caps, 12, "", false},
		{"after everything", caps, 99, "", false},
		{"negative time", caps, -1, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, ok := CaptionAt(tt.captions, tt.t)
			if ok != tt.ok || c.Text != tt.want {
				t.Errorf("CaptionAt(%v) = %q, %v; want %q, %v", tt.t, c.Text, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestValidators(t *testing.T) {
	if !EaseStep.Valid() || Ease("bounce").Valid() || !Ease("").Valid() {
		t.Error("Ease.Valid")
	}
	if !RevealBottom.Valid() || RevealFrom("middle").Valid() || !RevealFrom("").Valid() {
		t.Error("RevealFrom.Valid")
	}
	if RevealFrom("").Or() != RevealLeft || RevealTop.Or() != RevealTop {
		t.Error("RevealFrom.Or")
	}
}

package timeline

import (
	"math"
	"strings"
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/problem"
)

// validTimeline returns the example of CASSETTE-FORMAT §5.3 (12 s long).
func validTimeline() ([]Track, []Caption, float64) {
	tracks := []Track{
		{Actor: "pizza", Keys: []Key{{T: 0, Opacity: Float(0)}, {T: 1.5, Opacity: Float(1), Ease: EaseOut}}},
		{Actor: "slice", Origin: []float64{800, 450}, Keys: []Key{
			{T: 6, X: Float(0), Rotate: Float(0)},
			{T: 8, X: Float(240), Rotate: Float(15), Ease: EaseInOut},
		}},
		{Actor: "label-half", RevealBox: []float64{650, 720, 300, 110}, RevealFrom: RevealLeft, Keys: []Key{
			{T: 9, Reveal: Float(0)}, {T: 10.5, Reveal: Float(1)},
		}},
	}
	captions := []Caption{
		{Start: 0, End: 5.6, Text: "Here is a pizza. Let's share it fairly."},
		{Start: 5.8, End: 8.6, Text: "We cut it into two equal pieces."},
		{Start: 8.8, End: 12, Text: "Each piece is one half: 1/2."},
	}
	return tracks, captions, 12
}

func TestValidateValid(t *testing.T) {
	tracks, captions, length := validTimeline()
	if ps := Validate(tracks, captions, length); len(ps) != 0 {
		t.Fatalf("valid timeline has problems: %v", ps)
	}
	// Times up to the tolerance past the length are fine.
	captions[2].End = 12.05
	tracks[0].Keys[1].T = 12.05
	if ps := Validate(tracks, captions, length); len(ps) != 0 {
		t.Fatalf("tolerance not applied: %v", ps)
	}
	// No tracks is fine.
	if ps := Validate(nil, captions[:1], length); len(ps) != 0 {
		t.Fatalf("no tracks: %v", ps)
	}
}

func TestValidateRules(t *testing.T) {
	long := strings.Repeat("a", MaxCaptionRunes+1)
	tests := []struct {
		name string
		mut  func(tracks []Track, captions []Caption) ([]Track, []Caption)
		path string // the path of the (single) expected problem
		msg  string // a part of its message
	}{
		{"empty actor", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[0].Actor = ""
			return tr, c
		}, "tracks[0].actor", "required"},
		{"duplicate actor", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[2].Actor = "pizza"
			return tr, c
		}, "tracks[2].actor", "one track per actor"},
		{"bad reveal_from", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[2].RevealFrom = "middle"
			return tr, c
		}, "tracks[2].reveal_from", "left, right, top or bottom"},
		{"origin wrong length", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[1].Origin = []float64{1, 2, 3}
			return tr, c
		}, "tracks[1].origin", "2 numbers"},
		{"origin not finite", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[1].Origin = []float64{math.NaN(), 2}
			return tr, c
		}, "tracks[1].origin", "finite"},
		{"origin missing for scale", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[0].Keys[1].Scale = Float(2)
			return tr, c
		}, "tracks[0].origin", "scale or rotate"},
		{"origin missing for rotate", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[1].Origin = nil
			return tr, c
		}, "tracks[1].origin", "scale or rotate"},
		{"reveal_box wrong length", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[2].RevealBox = []float64{1, 2, 3}
			return tr, c
		}, "tracks[2].reveal_box", "4 numbers"},
		{"reveal_box zero width", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[2].RevealBox = []float64{1, 2, 0, 5}
			return tr, c
		}, "tracks[2].reveal_box", "greater than 0"},
		{"reveal_box negative height", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[2].RevealBox = []float64{1, 2, 5, -1}
			return tr, c
		}, "tracks[2].reveal_box", "greater than 0"},
		{"reveal_box missing", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[2].RevealBox = nil
			return tr, c
		}, "tracks[2].reveal_box", "sets reveal"},
		{"no keys", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[0].Keys = nil
			return tr, c
		}, "tracks[0].keys", "at least one key"},
		{"too many keys", func(tr []Track, c []Caption) ([]Track, []Caption) {
			keys := make([]Key, MaxKeys+1)
			for i := range keys {
				keys[i] = Key{T: float64(i) / 1000, X: Float(1)}
			}
			tr[0].Keys = keys
			return tr, c
		}, "tracks[0].keys", "at most 1000"},
		{"negative t", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[0].Keys[0].T = -1
			return tr, c
		}, "tracks[0].keys[0].t", "negative"},
		{"t past the narration", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[0].Keys[1].T = 12.06
			return tr, c
		}, "tracks[0].keys[1].t", "after the end of the narration (12)"},
		{"t not finite", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[0].Keys[1].T = math.Inf(1)
			return tr, c
		}, "tracks[0].keys[1].t", "finite"},
		{"t equal to the previous", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[1].Keys[1].T = 6
			return tr, c
		}, "tracks[1].keys[1].t", "strictly increasing"},
		{"t before the previous", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[2].Keys[1].T = 8
			return tr, c
		}, "tracks[2].keys[1].t", "strictly increasing"},
		{"opacity above 1", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[0].Keys[1].Opacity = Float(1.5)
			return tr, c
		}, "tracks[0].keys[1].opacity", "between 0 and 1"},
		{"opacity below 0", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[0].Keys[0].Opacity = Float(-0.1)
			return tr, c
		}, "tracks[0].keys[0].opacity", "between 0 and 1"},
		{"reveal above 1", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[2].Keys[1].Reveal = Float(2)
			return tr, c
		}, "tracks[2].keys[1].reveal", "between 0 and 1"},
		{"scale zero", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[1].Keys[0].Scale = Float(0)
			return tr, c
		}, "tracks[1].keys[0].scale", "greater than 0"},
		{"scale too big", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[1].Keys[0].Scale = Float(20.5)
			return tr, c
		}, "tracks[1].keys[0].scale", "at most 20"},
		{"x not finite", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[1].Keys[0].X = Float(math.NaN())
			return tr, c
		}, "tracks[1].keys[0].x", "finite"},
		{"y not finite", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[1].Keys[0].Y = Float(math.Inf(-1))
			return tr, c
		}, "tracks[1].keys[0].y", "finite"},
		{"rotate not finite", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[1].Keys[1].Rotate = Float(math.Inf(1))
			return tr, c
		}, "tracks[1].keys[1].rotate", "finite"},
		{"bad ease", func(tr []Track, c []Caption) ([]Track, []Caption) {
			tr[0].Keys[1].Ease = "bounce"
			return tr, c
		}, "tracks[0].keys[1].ease", "linear, in, out, in-out or step"},
		{"no captions", func(tr []Track, c []Caption) ([]Track, []Caption) {
			return tr, nil
		}, "captions", "at least one caption"},
		{"empty caption text", func(tr []Track, c []Caption) ([]Track, []Caption) {
			c[1].Text = ""
			return tr, c
		}, "captions[1].text", "must not be empty"},
		{"blank caption text", func(tr []Track, c []Caption) ([]Track, []Caption) {
			c[1].Text = " \n "
			return tr, c
		}, "captions[1].text", "must not be empty"},
		{"caption text too long", func(tr []Track, c []Caption) ([]Track, []Caption) {
			c[0].Text = long
			return tr, c
		}, "captions[0].text", "201 characters"},
		{"caption text counts characters", func(tr []Track, c []Caption) ([]Track, []Caption) {
			c[0].Text = strings.Repeat("é", MaxCaptionRunes)
			return tr, c
		}, "", ""},
		{"caption text with three lines", func(tr []Track, c []Caption) ([]Track, []Caption) {
			c[0].Text = "a\nb\nc"
			return tr, c
		}, "captions[0].text", "3 lines"},
		{"caption start negative", func(tr []Track, c []Caption) ([]Track, []Caption) {
			c[0].Start = -0.5
			return tr, c
		}, "captions[0].start", "at least 0"},
		{"caption end equals start", func(tr []Track, c []Caption) ([]Track, []Caption) {
			c[1].End = c[1].Start
			return tr, c
		}, "captions[1].end", "not after the start"},
		{"caption end before start", func(tr []Track, c []Caption) ([]Track, []Caption) {
			c[1].End = 5
			return tr, c
		}, "captions[1].end", "not after the start"},
		{"caption end past the narration", func(tr []Track, c []Caption) ([]Track, []Caption) {
			c[2].End = 12.06
			return tr, c
		}, "captions[2].end", "after the end of the narration"},
		{"caption end not finite", func(tr []Track, c []Caption) ([]Track, []Caption) {
			c[2].End = math.NaN()
			return tr, c
		}, "captions[2].end", "finite"},
		{"captions overlap", func(tr []Track, c []Caption) ([]Track, []Caption) {
			c[1].Start = 5.5
			return tr, c
		}, "captions[1].start", "must not overlap"},
		{"captions touching are fine", func(tr []Track, c []Caption) ([]Track, []Caption) {
			c[1].Start = c[0].End
			return tr, c
		}, "", ""},
		{"captions out of order", func(tr []Track, c []Caption) ([]Track, []Caption) {
			c[0], c[1] = c[1], c[0]
			return tr, c
		}, "captions[1].start", "in order"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracks, captions, length := validTimeline()
			tracks, captions = tt.mut(tracks, captions)
			ps := Validate(tracks, captions, length)
			if tt.path == "" {
				if len(ps) != 0 {
					t.Fatalf("expected no problems, got %v", ps)
				}
				return
			}
			if len(ps) != 1 {
				t.Fatalf("expected one problem, got %d: %v", len(ps), ps)
			}
			if ps[0].Path != tt.path || !strings.Contains(ps[0].Message, tt.msg) || ps[0].Warning {
				t.Errorf("got %v, want path %q and message containing %q", ps[0], tt.path, tt.msg)
			}
		})
	}
}

func TestValidateCollectsEveryProblem(t *testing.T) {
	tracks := []Track{
		{Actor: "a", Keys: []Key{{T: 1, Opacity: Float(2)}, {T: 1, Scale: Float(-1)}}},
		{Actor: "a", Keys: []Key{{T: -1}}},
	}
	captions := []Caption{{Start: 3, End: 2, Text: ""}}
	ps := Validate(tracks, captions, 10)
	var paths []string
	for _, p := range ps {
		paths = append(paths, p.Path)
	}
	want := []string{
		"tracks[0].keys[0].opacity",
		"tracks[0].keys[1].scale",
		"tracks[0].keys[1].t",
		"tracks[0].origin",
		"tracks[1].keys[0].t",
		"tracks[1].actor",
		"captions[0].text",
		"captions[0].end",
	}
	if strings.Join(paths, " ") != strings.Join(want, " ") {
		t.Errorf("paths = %v\nwant %v", paths, want)
	}
	if !problem.HasErrors(ps) {
		t.Error("HasErrors is false")
	}
}

func TestValidateTimeBoundWithZeroLength(t *testing.T) {
	// With a narration of length 0 only t <= 0.05 is allowed.
	tracks := []Track{{Actor: "a", Keys: []Key{{T: 0.05, X: Float(1)}}}}
	captions := []Caption{{Start: 0, End: 0.05, Text: "x"}}
	if ps := Validate(tracks, captions, 0); len(ps) != 0 {
		t.Errorf("unexpected problems: %v", ps)
	}
	tracks[0].Keys[0].T = 0.06
	if ps := Validate(tracks, captions, 0); len(ps) != 1 {
		t.Errorf("expected one problem, got %v", ps)
	}
}

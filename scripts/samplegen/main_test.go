package main

import (
	"encoding/json"
	"testing"

	"github.com/rm4n0s/errors"
)

func TestPlan(t *testing.T) {
	pause := 1.0
	sc := script{Silence: 0.4, Lead: 0.5, Tail: 2.0}
	sentences := []sentence{{Text: "a"}, {Text: "b", Pause: &pause}, {Text: "c"}}
	spans, total := plan(sc, sentences, []float64{2, 3, 1})
	want := []span{{0.5, 2.5}, {2.9, 5.9}, {6.9, 7.9}}
	for i := range want {
		if spans[i] != want[i] {
			t.Errorf("span %d = %+v, want %+v", i, spans[i], want[i])
		}
	}
	if total != 9.9 {
		t.Errorf("total = %v, want 9.9", total)
	}
}

func TestResolve(t *testing.T) {
	spans := []span{{0.5, 2.5}, {2.9, 5.9}}
	tests := []struct {
		anchor string
		want   float64
		route  string
	}{
		{anchor: "s1", want: 0.5},
		{anchor: "e1", want: 2.5},
		{anchor: "s2+0.25", want: 3.15},
		{anchor: "e2-0.9", want: 5},
		{anchor: "end", want: 8},
		{anchor: "end-1", want: 7},
		{anchor: "s3", route: "resolve.SampleGenAnchorSentence"},
		{anchor: "s0", route: "resolve.SampleGenAnchorSentence"},
		{anchor: "x1", route: "resolve.SampleGenAnchorInvalid"},
		{anchor: "s1+", route: "resolve.SampleGenAnchorInvalid"},
	}
	for _, tt := range tests {
		t.Run(tt.anchor, func(t *testing.T) {
			got, err := resolve(tt.anchor, spans, 8)
			if tt.route != "" {
				appErr, ok := errors.FromError(err)
				if !ok {
					t.Fatalf("err = %v, want an *errors.Error", err)
				}
				if !appErr.HasRoute(tt.route) {
					t.Errorf("route = %s, want %s", appErr.Route(), tt.route)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("resolve(%q) = %v, %v; want %v", tt.anchor, got, err, tt.want)
			}
		})
	}
}

func TestLessonJSON(t *testing.T) {
	one := 1.0
	zero := 0.0
	tm := template{Title: "T", Tracks: []tmplTrack{{Actor: "a", Keys: []tmplKey{
		{At: "s1", Opacity: &zero},
		{At: "e1", Opacity: &one, Ease: "out"},
	}}}}
	it := item{ID: "l", Sentences: []sentence{{Text: "Hello."}}}
	spans := []span{{0.5, 2.5}}
	data, err := lessonJSON(tm, it, spans, 4)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Tracks []struct {
			Keys []struct {
				T       float64
				Opacity float64
				Ease    string
			}
		}
		Captions []struct {
			Start, End float64
			Text       string
		}
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, data)
	}
	if k := got.Tracks[0].Keys; k[0].T != 0.5 || k[1].T != 2.5 || k[1].Ease != "out" {
		t.Errorf("keys = %+v", k)
	}
	if c := got.Captions[0]; c.Start != 0.5 || c.End != 2.5 || c.Text != "Hello." {
		t.Errorf("caption = %+v", c)
	}

	late := template{Tracks: []tmplTrack{{Actor: "a", Keys: []tmplKey{{At: "end+1"}}}}}
	_, err = lessonJSON(late, it, spans, 4)
	appErr, ok := errors.FromError(err)
	if !ok || !appErr.HasRoute("lessonJSON.SampleGenKeyAfterEnd") {
		t.Errorf("a key after the end: err = %v", err)
	}
}

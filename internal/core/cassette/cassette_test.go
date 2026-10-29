package cassette

import (
	"reflect"
	"testing"
)

func TestWorksheets(t *testing.T) {
	item := func(typ ItemType, id string) Item { return Item{Type: typ, ID: id} }
	l, e := ItemLesson, ItemExercise
	tests := []struct {
		name  string
		items []Item
		want  []Worksheet
	}{
		{"none", []Item{item(l, "a"), item(l, "b")}, nil},
		{"one run", []Item{item(l, "a"), item(e, "x"), item(e, "y"), item(l, "b")},
			[]Worksheet{{Index: 0, Start: 1, ExerciseIDs: []string{"x", "y"}}}},
		{"a run per lesson", []Item{item(l, "a"), item(e, "x"), item(l, "b"), item(e, "y"), item(e, "z")},
			[]Worksheet{{Index: 0, Start: 1, ExerciseIDs: []string{"x"}}, {Index: 1, Start: 3, ExerciseIDs: []string{"y", "z"}}}},
		{"starting and ending with exercises", []Item{item(e, "x"), item(l, "a"), item(e, "y")},
			[]Worksheet{{Index: 0, Start: 0, ExerciseIDs: []string{"x"}}, {Index: 1, Start: 2, ExerciseIDs: []string{"y"}}}},
		{"only exercises", []Item{item(e, "x"), item(e, "y"), item(e, "z")},
			[]Worksheet{{Index: 0, Start: 0, ExerciseIDs: []string{"x", "y", "z"}}}},
		{"no items", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cassette{Manifest: Manifest{Items: tt.items}}
			if got := c.Worksheets(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Worksheets() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestWorksheetOf(t *testing.T) {
	c, ps := load(t, minimalFS(t))
	if len(ps) != 0 {
		t.Fatal(ps)
	}
	tests := []struct {
		id    string
		index int
		ok    bool
	}{
		{"shade-the-half", 0, true},
		{"name-the-fraction", 0, true},
		{"write-the-sum", 1, true},
		{"what-is-a-fraction", 0, false}, // a lesson
		{"nope", 0, false},
	}
	for _, tt := range tests {
		w, ok := c.WorksheetOf(tt.id)
		if ok != tt.ok || (ok && w.Index != tt.index) {
			t.Errorf("WorksheetOf(%q) = %+v, %v; want index %d, %v", tt.id, w, ok, tt.index, tt.ok)
		}
	}
	w, _ := c.WorksheetOf("shade-the-half")
	if !reflect.DeepEqual(w.ExerciseIDs, []string{"shade-the-half", "name-the-fraction"}) || w.Start != 1 {
		t.Errorf("worksheet = %+v", w)
	}
}

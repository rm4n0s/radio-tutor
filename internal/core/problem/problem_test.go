package problem

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestProblemString(t *testing.T) {
	tests := []struct {
		name string
		p    Problem
		want string
	}{
		{"error", Problem{Path: "captions[1].end", Message: "must be after start"}, "captions[1].end: must be after start"},
		{"no path", Problem{Message: "at least one caption"}, "at least one caption"},
		{"warning", Problem{Path: "items[0]", Message: "should be a lesson", Warning: true}, "warning: items[0]: should be a lesson"},
		{"warning no path", Problem{Message: "odd", Warning: true}, "warning: odd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJoinAndIndex(t *testing.T) {
	tests := []struct {
		name          string
		parent, child string
		want          string
	}{
		{"both empty", "", "", ""},
		{"empty parent", "", "tracks", "tracks"},
		{"empty child", "tracks", "", "tracks"},
		{"field", "tracks[2]", "keys", "tracks[2].keys"},
		{"index", "tracks", "[2]", "tracks[2]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Join(tt.parent, tt.child); got != tt.want {
				t.Errorf("Join(%q, %q) = %q, want %q", tt.parent, tt.child, got, tt.want)
			}
		})
	}
	if got := Index("tracks", 2); got != "tracks[2]" {
		t.Errorf("Index = %q", got)
	}
}

func TestListAndFilters(t *testing.T) {
	var l List
	if l.HasErrors() {
		t.Fatal("empty list has errors")
	}
	l.Warnf("a", "warn %d", 1)
	if l.HasErrors() || !reflect.DeepEqual(Errors(l), []Problem(nil)) {
		t.Fatalf("a warning must not count as an error: %v", l)
	}
	l.Errorf("b", "err %s", "x")
	l.Add(Problem{Path: "c", Message: "m"})
	if !l.HasErrors() || !HasErrors(l) {
		t.Fatal("expected errors")
	}
	wantErrs := []Problem{{Path: "b", Message: "err x"}, {Path: "c", Message: "m"}}
	if got := Errors(l); !reflect.DeepEqual(got, wantErrs) {
		t.Errorf("Errors = %v, want %v", got, wantErrs)
	}
	wantWarns := []Problem{{Path: "a", Message: "warn 1", Warning: true}}
	if got := Warnings(l); !reflect.DeepEqual(got, wantWarns) {
		t.Errorf("Warnings = %v, want %v", got, wantWarns)
	}
}

func TestPrefixedAndInFile(t *testing.T) {
	in := []Problem{
		{Path: "tracks[2].keys[5].opacity", Message: "m1"},
		{Path: "", Message: "m2", Warning: true},
		{Path: "[0]", Message: "m3"},
	}
	orig := append([]Problem(nil), in...)

	gotP := Prefixed("timeline", in)
	wantP := []Problem{
		{Path: "timeline.tracks[2].keys[5].opacity", Message: "m1"},
		{Path: "timeline", Message: "m2", Warning: true},
		{Path: "timeline[0]", Message: "m3"},
	}
	if !reflect.DeepEqual(gotP, wantP) {
		t.Errorf("Prefixed = %v, want %v", gotP, wantP)
	}

	gotF := InFile("lessons/l1/lesson.json", in)
	wantF := []Problem{
		{Path: "lessons/l1/lesson.json#tracks[2].keys[5].opacity", Message: "m1"},
		{Path: "lessons/l1/lesson.json", Message: "m2", Warning: true},
		{Path: "lessons/l1/lesson.json#[0]", Message: "m3"},
	}
	if !reflect.DeepEqual(gotF, wantF) {
		t.Errorf("InFile = %v, want %v", gotF, wantF)
	}
	if !reflect.DeepEqual(in, orig) {
		t.Errorf("input was changed: %v", in)
	}
}

func TestJSON(t *testing.T) {
	b, err := json.Marshal([]Problem{{Path: "a", Message: "m"}, {Path: "b", Message: "n", Warning: true}})
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"path":"a","message":"m"},{"path":"b","message":"n","warning":true}]`
	if string(b) != want {
		t.Errorf("json = %s, want %s", b, want)
	}
}

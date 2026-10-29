package cassette

import (
	"strings"
	"testing"
	"testing/fstest"
)

func lessonEdit(fn func(m map[string]any)) func(t *testing.T, fsys fstest.MapFS) {
	return fileEdit(lesson1, fn)
}

func exerciseEdit(name string, fn func(m map[string]any)) func(t *testing.T, fsys fstest.MapFS) {
	return fileEdit(name, fn)
}

func TestLoadLessonRules(t *testing.T) {
	const l1 = "lessons/what-is-a-fraction/lesson.json#"
	long := func(n int) string { return strings.Repeat("a", n) }
	tests := []struct {
		name string
		mut  func(t *testing.T, fsys fstest.MapFS)
		want []want
	}{
		{"format 0", lessonEdit(func(m map[string]any) { m["format"] = 0 }), []want{{l1 + "format", "must be 1", false}}},
		{"format missing", lessonEdit(func(m map[string]any) { delete(m, "format") }), []want{{l1 + "format", "is required", false}}},
		{"title empty", lessonEdit(func(m map[string]any) { m["title"] = "" }), []want{{l1 + "title", "must not be empty", false}}},
		{"title 81", lessonEdit(func(m map[string]any) { m["title"] = long(81) }), []want{{l1 + "title", "81 characters", false}}},
		{"title missing", lessonEdit(func(m map[string]any) { delete(m, "title") }), []want{{l1 + "title", "is required", false}}},
		{"summary 301", lessonEdit(func(m map[string]any) { m["summary"] = long(301) }), []want{{l1 + "summary", "301 characters", false}}},
		{"summary is optional", lessonEdit(func(m map[string]any) { delete(m, "summary") }), nil},
		{"unknown field", lessonEdit(func(m map[string]any) { m["duration"] = 12 }), []want{{l1 + "duration", "unknown field", false}}},
		{"tracks missing", lessonEdit(func(m map[string]any) { delete(m, "tracks") }), []want{{l1 + "tracks", "is required", false}}},
		{"tracks empty is fine", lessonEdit(func(m map[string]any) { m["tracks"] = []any{} }), nil},
		{"tracks is null", lessonEdit(func(m map[string]any) { m["tracks"] = nil }), []want{{l1 + "tracks", "must be an array, not null", false}}},
		{"captions missing", lessonEdit(func(m map[string]any) { delete(m, "captions") }), []want{{l1 + "captions", "is required", false}}},
		{"captions empty", lessonEdit(func(m map[string]any) { m["captions"] = []any{} }), []want{{l1 + "captions", "at least one caption", false}}},
		{"track without actor", lessonEdit(func(m map[string]any) { delete(obj(m, "tracks", 0), "actor") }), []want{{l1 + "tracks[0].actor", "is required", false}}},
		{"track without keys", lessonEdit(func(m map[string]any) { delete(obj(m, "tracks", 0), "keys") }), []want{{l1 + "tracks[0].keys", "is required", false}}},
		{"track with an empty key list", lessonEdit(func(m map[string]any) { obj(m, "tracks", 0)["keys"] = []any{} }),
			[]want{{l1 + "tracks[0].keys", "at least one key", false}}},
		{"track with an unknown field", lessonEdit(func(m map[string]any) { obj(m, "tracks", 1)["pivot"] = []any{1, 2} }),
			[]want{{l1 + "tracks[1].pivot", "unknown field", false}}},
		{"origin with a string", lessonEdit(func(m map[string]any) { obj(m, "tracks", 1)["origin"] = []any{"a", 2} }),
			[]want{{l1 + "tracks[1].origin[0]", "must be a number, not a string", false}}},
		{"origin with one number", lessonEdit(func(m map[string]any) { obj(m, "tracks", 1)["origin"] = []any{5} }),
			[]want{{l1 + "tracks[1].origin", "2 numbers", false}}},
		{"origin missing for rotate", lessonEdit(func(m map[string]any) { delete(obj(m, "tracks", 1), "origin") }),
			[]want{{l1 + "tracks[1].origin", "scale or rotate", false}}},
		{"reveal_box missing", lessonEdit(func(m map[string]any) { delete(obj(m, "tracks", 2), "reveal_box") }),
			[]want{{l1 + "tracks[2].reveal_box", "sets reveal", false}}},
		{"reveal_from invalid", lessonEdit(func(m map[string]any) { obj(m, "tracks", 2)["reveal_from"] = "centre" }),
			[]want{{l1 + "tracks[2].reveal_from", "left, right, top or bottom", false}}},
		{"two tracks for one actor", lessonEdit(func(m map[string]any) { obj(m, "tracks", 2)["actor"] = "pizza" }),
			[]want{{l1 + "tracks[2].actor", "one track per actor", false}}},
		{"key without t", lessonEdit(func(m map[string]any) { delete(obj(m, "tracks", 0, "keys", 0), "t") }),
			[]want{{l1 + "tracks[0].keys[0].t", "is required", false}}},
		{"key with t null", lessonEdit(func(m map[string]any) { obj(m, "tracks", 0, "keys", 0)["t"] = nil }),
			[]want{{l1 + "tracks[0].keys[0].t", "must be a number, not null", false}}},
		{"key with t a string", lessonEdit(func(m map[string]any) { obj(m, "tracks", 0, "keys", 0)["t"] = "0" }),
			[]want{{l1 + "tracks[0].keys[0].t", "must be a number, not a string", false}}},
		{"key with t 0 is fine", lessonEdit(func(m map[string]any) { obj(m, "tracks", 0, "keys", 0)["t"] = 0 }), nil},
		{"key with an unknown property", lessonEdit(func(m map[string]any) { obj(m, "tracks", 0, "keys", 1)["alpha"] = 1 }),
			[]want{{l1 + "tracks[0].keys[1].alpha", "unknown field", false}}},
		{"key with opacity a string", lessonEdit(func(m map[string]any) { obj(m, "tracks", 0, "keys", 1)["opacity"] = "1" }),
			[]want{{l1 + "tracks[0].keys[1].opacity", "must be a number", false}}},
		{"key with opacity above 1", lessonEdit(func(m map[string]any) { obj(m, "tracks", 0, "keys", 1)["opacity"] = 2 }),
			[]want{{l1 + "tracks[0].keys[1].opacity", "between 0 and 1", false}}},
		{"key with a bad ease", lessonEdit(func(m map[string]any) { obj(m, "tracks", 0, "keys", 1)["ease"] = "bounce" }),
			[]want{{l1 + "tracks[0].keys[1].ease", "linear, in, out, in-out or step", false}}},
		{"keys not increasing", lessonEdit(func(m map[string]any) { obj(m, "tracks", 0, "keys", 1)["t"] = 0 }),
			[]want{{l1 + "tracks[0].keys[1].t", "strictly increasing", false}}},
		{"keys past any length are fine without the narration", lessonEdit(func(m map[string]any) {
			obj(m, "tracks", 0, "keys", 1)["t"] = 5000
			obj(m, "captions", 2)["end"] = 6000
		}), nil},
		{"caption without start", lessonEdit(func(m map[string]any) { delete(obj(m, "captions", 0), "start") }),
			[]want{{l1 + "captions[0].start", "is required", false}}},
		{"caption without end", lessonEdit(func(m map[string]any) { delete(obj(m, "captions", 1), "end") }),
			[]want{{l1 + "captions[1].end", "is required", false}}},
		{"caption without text", lessonEdit(func(m map[string]any) { delete(obj(m, "captions", 1), "text") }),
			[]want{{l1 + "captions[1].text", "is required", false}}},
		{"caption with an unknown field", lessonEdit(func(m map[string]any) { obj(m, "captions", 1)["speaker"] = "x" }),
			[]want{{l1 + "captions[1].speaker", "unknown field", false}}},
		{"caption text empty", lessonEdit(func(m map[string]any) { obj(m, "captions", 1)["text"] = "" }),
			[]want{{l1 + "captions[1].text", "must not be empty", false}}},
		{"caption text 201 characters", lessonEdit(func(m map[string]any) { obj(m, "captions", 1)["text"] = long(201) }),
			[]want{{l1 + "captions[1].text", "201 characters", false}}},
		{"caption text of 200 Greek letters is fine", lessonEdit(func(m map[string]any) { obj(m, "captions", 1)["text"] = strings.Repeat("ή", 200) }), nil},
		{"caption text with three lines", lessonEdit(func(m map[string]any) { obj(m, "captions", 1)["text"] = "a\nb\nc" }),
			[]want{{l1 + "captions[1].text", "3 lines", false}}},
		{"caption end not after start", lessonEdit(func(m map[string]any) { obj(m, "captions", 1)["end"] = 5.8 }),
			[]want{{l1 + "captions[1].end", "not after the start", false}}},
		{"captions overlap", lessonEdit(func(m map[string]any) { obj(m, "captions", 1)["start"] = 5 }),
			[]want{{l1 + "captions[1].start", "must not overlap", false}}},
		{"second lesson has its own path", fileEdit(lesson2, func(m map[string]any) { m["title"] = "" }),
			[]want{{"lessons/adding-fractions/lesson.json#title", "must not be empty", false}}},
		{"several structure problems at once", lessonEdit(func(m map[string]any) {
			delete(obj(m, "tracks", 0, "keys", 0), "t")
			delete(obj(m, "captions", 2), "start")
			m["extra"] = 1
		}), []want{{l1 + "tracks[0].keys[0].t", "is required", false}, {l1 + "captions[2].start", "is required", false}, {l1 + "extra", "unknown field", false}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := minimalFS(t)
			tt.mut(t, fsys)
			_, ps := load(t, fsys)
			checkProblems(t, ps, tt.want...)
		})
	}
}

func TestLoadExerciseRules(t *testing.T) {
	const e1 = "exercises/shade-the-half/exercise.json#"
	const e2 = "exercises/name-the-fraction/exercise.json#"
	long := func(n int) string { return strings.Repeat("a", n) }
	tests := []struct {
		name string
		mut  func(t *testing.T, fsys fstest.MapFS)
		want []want
	}{
		{"format 0", exerciseEdit(ex1, func(m map[string]any) { m["format"] = 0 }), []want{{e1 + "format", "must be 1", false}}},
		{"title missing", exerciseEdit(ex1, func(m map[string]any) { delete(m, "title") }), []want{{e1 + "title", "is required", false}}},
		{"title 81", exerciseEdit(ex1, func(m map[string]any) { m["title"] = long(81) }), []want{{e1 + "title", "81 characters", false}}},
		{"prompt missing", exerciseEdit(ex1, func(m map[string]any) { delete(m, "prompt") }), []want{{e1 + "prompt", "is required", false}}},
		{"prompt empty", exerciseEdit(ex1, func(m map[string]any) { m["prompt"] = "" }), []want{{e1 + "prompt", "must not be empty", false}}},
		{"prompt 2001", exerciseEdit(ex1, func(m map[string]any) { m["prompt"] = long(2001) }), []want{{e1 + "prompt", "2001 characters", false}}},
		{"prompt 2000 multi-byte characters is fine", exerciseEdit(ex1, func(m map[string]any) { m["prompt"] = strings.Repeat("π", 2000) }), nil},
		{"reference_solution missing", exerciseEdit(ex1, func(m map[string]any) { delete(m, "reference_solution") }),
			[]want{{e1 + "reference_solution", "is required", false}}},
		{"reference_solution 4001", exerciseEdit(ex1, func(m map[string]any) { m["reference_solution"] = long(4001) }),
			[]want{{e1 + "reference_solution", "4001 characters", false}}},
		{"grading missing", exerciseEdit(ex1, func(m map[string]any) { delete(m, "grading") }), []want{{e1 + "grading", "is required", false}}},
		{"grading 4001", exerciseEdit(ex1, func(m map[string]any) { m["grading"] = long(4001) }), []want{{e1 + "grading", "4001 characters", false}}},
		{"show_work not a boolean", exerciseEdit(ex1, func(m map[string]any) { m["show_work"] = "yes" }),
			[]want{{e1 + "show_work", "must be a boolean, not a string", false}}},
		{"show_work is optional", exerciseEdit(ex2, func(m map[string]any) { delete(m, "show_work") }), nil},
		{"max_attempts negative", exerciseEdit(ex1, func(m map[string]any) { m["max_attempts"] = -1 }), []want{{e1 + "max_attempts", "not from 0 to 20", false}}},
		{"max_attempts 21", exerciseEdit(ex1, func(m map[string]any) { m["max_attempts"] = 21 }), []want{{e1 + "max_attempts", "not from 0 to 20", false}}},
		{"max_attempts 20", exerciseEdit(ex1, func(m map[string]any) { m["max_attempts"] = 20 }), nil},
		{"max_attempts not whole", exerciseEdit(ex1, func(m map[string]any) { m["max_attempts"] = 2.5 }), []want{{e1 + "max_attempts", "whole number", false}}},
		{"max_attempts huge", exerciseEdit(ex1, func(m map[string]any) { m["max_attempts"] = 1e12 }), []want{{e1 + "max_attempts", "out of range", false}}},
		{"hints 6", exerciseEdit(ex1, func(m map[string]any) { m["hints"] = []any{"a", "b", "c", "d", "e", "f"} }), []want{{e1 + "hints", "6 hints", false}}},
		{"hints 5", exerciseEdit(ex1, func(m map[string]any) { m["hints"] = []any{"a", "b", "c", "d", "e"} }), nil},
		{"hint 301", exerciseEdit(ex1, func(m map[string]any) { m["hints"] = []any{"a", long(301)} }), []want{{e1 + "hints[1]", "301 characters", false}}},
		{"hint empty", exerciseEdit(ex1, func(m map[string]any) { m["hints"] = []any{""} }), []want{{e1 + "hints[0]", "must not be empty", false}}},
		{"hint not a string", exerciseEdit(ex1, func(m map[string]any) { m["hints"] = []any{3} }), []want{{e1 + "hints[0]", "must be a string", false}}},
		{"unknown field", exerciseEdit(ex1, func(m map[string]any) { m["answer"] = "x" }), []want{{e1 + "answer", "unknown field", false}}},

		// labels
		{"label empty", exerciseEdit(ex1, func(m map[string]any) { m["label"] = "" }), []want{{e1 + "label", "not a label", false}}},
		{"label 7 characters", exerciseEdit(ex1, func(m map[string]any) { m["label"] = "abcdefg" }), []want{{e1 + "label", "not a label", false}}},
		{"label 6 characters", exerciseEdit(ex1, func(m map[string]any) { m["label"] = "a.b-c9" }), nil},
		{"label with a space", exerciseEdit(ex1, func(m map[string]any) { m["label"] = "a b" }), []want{{e1 + "label", "not a label", false}}},
		{"label with a slash", exerciseEdit(ex1, func(m map[string]any) { m["label"] = "1/2" }), []want{{e1 + "label", "not a label", false}}},
		{"label with an emoji", exerciseEdit(ex1, func(m map[string]any) { m["label"] = "★" }), []want{{e1 + "label", "not a label", false}}},
		{"label counts characters not bytes", exerciseEdit(ex1, func(m map[string]any) { m["label"] = "αβγδεζ" }), nil},
		{"label not a string", exerciseEdit(ex1, func(m map[string]any) { m["label"] = 3 }), []want{{e1 + "label", "must be a string", false}}},
		{"explicit labels are unique", exerciseEdit(ex1, func(m map[string]any) { m["label"] = "B2" }),
			[]want{{e2 + "label", `"B2" is already the label of exercise "shade-the-half"`, false}}},
		{"a default label collides with an explicit one", exerciseEdit(ex2, func(m map[string]any) { m["label"] = "1" }),
			[]want{{e2 + "label", `"1" is already the label of exercise "shade-the-half"`, false}}},
		{"an explicit label collides with a later default", exerciseEdit(ex2, func(m map[string]any) { m["label"] = "3" }),
			[]want{{"exercises/write-the-sum/exercise.json#label", `default label "3" (the exercise's position) is already the label of exercise "name-the-fraction"`, false}}},
		{"labels differ only by case are different", exerciseEdit(ex2, func(m map[string]any) { m["label"] = "A" }), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := minimalFS(t)
			tt.mut(t, fsys)
			_, ps := load(t, fsys)
			checkProblems(t, ps, tt.want...)
		})
	}
}

// The default label is the exercise's position among all the exercises of the
// cassette, also when an earlier exercise has an explicit label or its file
// is broken.
func TestLoadDefaultLabels(t *testing.T) {
	fsys := minimalFS(t)
	edit(t, fsys, ex1, func(m map[string]any) { m["label"] = "first" })
	removeFile(fsys, ex2) // the second exercise has no file; the third is still the third
	c, ps := load(t, fsys)
	checkProblems(t, ps, want{"exercises/name-the-fraction/exercise.json", "is missing", false})
	if got := c.Exercises["write-the-sum"].Label; got != "3" {
		t.Errorf("label of the third exercise = %q, want 3", got)
	}
	if got := c.Exercises["shade-the-half"].Label; got != "first" {
		t.Errorf("label = %q", got)
	}
	if _, ok := c.Exercises["name-the-fraction"]; ok {
		t.Error("the exercise with no file is in the cassette")
	}
}

func TestLoadFolders(t *testing.T) {
	tests := []struct {
		name string
		mut  func(t *testing.T, fsys fstest.MapFS)
		want []want
	}{
		{"lesson folder missing", func(t *testing.T, fsys fstest.MapFS) { removeFile(fsys, lesson1) },
			[]want{{"lessons/what-is-a-fraction/lesson.json", "is missing", false}}},
		{"lesson folder missing entirely", func(t *testing.T, fsys fstest.MapFS) {
			for name := range fsys {
				if strings.HasPrefix(name, "lessons/") {
					delete(fsys, name)
				}
			}
		}, []want{
			{"cassette.json#items[0].id", `lesson "what-is-a-fraction" has no folder lessons/what-is-a-fraction`, false},
			{"cassette.json#items[3].id", `lesson "adding-fractions" has no folder lessons/adding-fractions`, false},
		}},
		{"exercise folder missing entirely", func(t *testing.T, fsys fstest.MapFS) {
			for name := range fsys {
				if strings.HasPrefix(name, "exercises/") {
					delete(fsys, name)
				}
			}
		}, []want{
			{"cassette.json#items[1].id", `exercise "shade-the-half" has no folder exercises/shade-the-half`, false},
			{"cassette.json#items[2].id", "has no folder exercises/name-the-fraction", false},
			{"cassette.json#items[4].id", "has no folder exercises/write-the-sum", false},
		}},
		{"exercise file missing", func(t *testing.T, fsys fstest.MapFS) { removeFile(fsys, ex3) },
			[]want{{"exercises/write-the-sum/exercise.json", "is missing", false}}},
		{"folder for a lesson that is an exercise item", func(t *testing.T, fsys fstest.MapFS) {
			fsys["exercises/what-is-a-fraction/exercise.json"] = fsys[ex1]
			delete(fsys, ex1)
			edit(t, fsys, manifest, func(m map[string]any) { obj(m, "items", 1)["id"] = "what-is-a-fraction" })
		}, []want{
			{"cassette.json#items[1].id", "already used by items[0]", false},
			{"exercises/what-is-a-fraction", "not listed", false},
		}},
		{"lesson folder not listed", func(t *testing.T, fsys fstest.MapFS) {
			fsys["lessons/extra/lesson.json"] = fsys[lesson1]
		}, []want{{"lessons/extra", "not listed", false}}},
		{"exercise folder not listed", func(t *testing.T, fsys fstest.MapFS) {
			fsys["exercises/extra/exercise.json"] = fsys[ex1]
		}, []want{{"exercises/extra", "not listed", false}}},
		{"exercise folder listed as a lesson only", func(t *testing.T, fsys fstest.MapFS) {
			fsys["lessons/shade-the-half/lesson.json"] = fsys[lesson1]
		}, []want{{"lessons/shade-the-half", "not listed", false}}},
		{"a file in lessons is not this step's concern", func(t *testing.T, fsys fstest.MapFS) {
			fsys["lessons/notes.txt"] = &fstest.MapFile{Data: []byte("x")}
			fsys["exercises/readme.md"] = &fstest.MapFile{Data: []byte("x")}
		}, nil},
		{"a file where a lesson folder should be", func(t *testing.T, fsys fstest.MapFS) {
			for name := range fsys {
				if strings.HasPrefix(name, "lessons/adding-fractions/") {
					delete(fsys, name)
				}
			}
			fsys["lessons/adding-fractions"] = &fstest.MapFile{Data: []byte("x")}
		}, []want{{"lessons/adding-fractions", "must be a folder, not a file", false}}},
		{"lessons is a file", func(t *testing.T, fsys fstest.MapFS) {
			for name := range fsys {
				if strings.HasPrefix(name, "lessons/") {
					delete(fsys, name)
				}
			}
			fsys["lessons"] = &fstest.MapFile{Data: []byte("x")}
		}, []want{{"lessons", "must be a folder, not a file", false},
			{"cassette.json#items[0].id", "has no folder", false}, {"cassette.json#items[3].id", "has no folder", false}}},
		{"lesson.json is a folder", func(t *testing.T, fsys fstest.MapFS) {
			delete(fsys, lesson2)
			fsys[lesson2+"/x"] = &fstest.MapFile{Data: []byte("x")}
		}, []want{{"lessons/adding-fractions/lesson.json", "must be a file, not a folder", false}}},
		{"no cassette.json at all", func(t *testing.T, fsys fstest.MapFS) { delete(fsys, manifest) },
			[]want{{"cassette.json", "is missing", false}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := minimalFS(t)
			tt.mut(t, fsys)
			_, ps := load(t, fsys)
			checkProblems(t, ps, tt.want...)
		})
	}
}

func TestLoadReturnsNilCassetteOnlyWhenTheManifestIsUnusable(t *testing.T) {
	tests := []struct {
		name    string
		mut     func(t *testing.T, fsys fstest.MapFS)
		wantNil bool
	}{
		{"valid", func(t *testing.T, fsys fstest.MapFS) {}, false},
		{"manifest missing", func(t *testing.T, fsys fstest.MapFS) { delete(fsys, manifest) }, true},
		{"manifest structure wrong", manifestEdit(func(m map[string]any) { delete(m, "items") }), true},
		{"manifest newer format", manifestEdit(func(m map[string]any) { m["format"] = 2 }), true},
		{"manifest field rule broken", manifestEdit(func(m map[string]any) { m["title"] = "" }), false},
		{"lesson broken", lessonEdit(func(m map[string]any) { delete(m, "title") }), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := minimalFS(t)
			tt.mut(t, fsys)
			c, _ := load(t, fsys)
			if (c == nil) != tt.wantNil {
				t.Errorf("cassette nil = %v, want %v", c == nil, tt.wantNil)
			}
		})
	}
}

func TestLoadPartialCassette(t *testing.T) {
	fsys := minimalFS(t)
	edit(t, fsys, lesson1, func(m map[string]any) { delete(m, "title") }) // structure: not kept
	edit(t, fsys, ex3, func(m map[string]any) { m["prompt"] = "" })       // field rule: kept
	c, ps := load(t, fsys)
	checkProblems(t, ps,
		want{"lessons/what-is-a-fraction/lesson.json#title", "is required", false},
		want{"exercises/write-the-sum/exercise.json#prompt", "must not be empty", false})
	if _, ok := c.Lessons["what-is-a-fraction"]; ok {
		t.Error("lesson with a wrong structure is in the cassette")
	}
	if _, ok := c.Lessons["adding-fractions"]; !ok {
		t.Error("good lesson is missing")
	}
	if _, ok := c.Exercises["write-the-sum"]; !ok {
		t.Error("exercise with a broken field rule is missing")
	}
}

func TestLoadCollectsProblemsOfEveryFile(t *testing.T) {
	fsys := minimalFS(t)
	edit(t, fsys, manifest, func(m map[string]any) { m["subject"] = "" })
	edit(t, fsys, lesson2, func(m map[string]any) { m["title"] = "" })
	edit(t, fsys, ex1, func(m map[string]any) { m["grading"] = "" })
	removeFile(fsys, ex3)
	fsys["lessons/extra/lesson.json"] = fsys[lesson1]
	_, ps := load(t, fsys)
	checkProblems(t, ps,
		want{"cassette.json#subject", "must not be empty", false},
		want{"lessons/adding-fractions/lesson.json#title", "must not be empty", false},
		want{"exercises/shade-the-half/exercise.json#grading", "must not be empty", false},
		want{"exercises/write-the-sum/exercise.json", "is missing", false},
		want{"lessons/extra", "not listed", false})
}

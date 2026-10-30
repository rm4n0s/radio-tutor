package cassette

import (
	"context"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

const (
	manifest = "cassette.json"
	lesson1  = "lessons/what-is-a-fraction/lesson.json"
	lesson2  = "lessons/adding-fractions/lesson.json"
	ex1      = "exercises/shade-the-half/exercise.json"
	ex2      = "exercises/name-the-fraction/exercise.json"
	ex3      = "exercises/write-the-sum/exercise.json"
)

func TestLoadValid(t *testing.T) {
	c, ps := load(t, minimalFS(t))
	if len(ps) != 0 {
		t.Fatalf("problems: %v", ps)
	}
	m := c.Manifest
	if m.ID != "org.example.minimal-1" || m.Version != (Version{1, 0, 0}) || m.Title != "Fractions, part 1" ||
		m.Language != "en" || m.Subject != "math" || len(m.Ages) != 2 || m.Ages[1] != 10 || len(m.Items) != 5 {
		t.Errorf("manifest = %+v", m)
	}
	if len(c.Lessons) != 2 || len(c.Exercises) != 3 {
		t.Fatalf("lessons %d, exercises %d", len(c.Lessons), len(c.Exercises))
	}
	le := c.Lessons["what-is-a-fraction"]
	if le.ID != "what-is-a-fraction" || le.Title != "What is a fraction?" || len(le.Tracks) != 3 || len(le.Captions) != 3 {
		t.Errorf("lesson = %+v", le)
	}
	if k := le.Tracks[1].Keys[1]; k.T != 8 || k.X == nil || *k.X != 240 || k.Y != nil || k.Ease != "in-out" {
		t.Errorf("key = %+v", k)
	}
	if got := c.Lessons["adding-fractions"].Captions[0].Text; got != "When the bottom numbers are the same,\nwe add the top numbers." {
		t.Errorf("caption = %q", got)
	}

	exercises := []struct {
		id       string
		label    string
		attempts int
		showWork bool
		hints    int
	}{
		{"shade-the-half", "1", DefaultMaxAttempts, false, 0},
		{"name-the-fraction", "B2", 0, true, 1},
		{"write-the-sum", "3", 5, false, 2},
	}
	for _, w := range exercises {
		e := c.Exercises[w.id]
		if e == nil {
			t.Fatalf("no exercise %s", w.id)
		}
		if e.ID != w.id || e.Label != w.label || e.MaxAttempts != w.attempts || e.ShowWork != w.showWork || len(e.Hints) != w.hints {
			t.Errorf("%s = %+v", w.id, e)
		}
	}
	if e := c.Exercises["write-the-sum"]; e.Prompt != "Add the fractions and write the result:\n1/4 + 2/4 = ?" {
		t.Errorf("prompt = %q", e.Prompt)
	}
}

func TestLoadFromDirFS(t *testing.T) {
	c, ps, err := Load(context.Background(), fsysDir(t))
	if err != nil || len(ps) != 0 || c == nil || len(c.Exercises) != 3 {
		t.Fatalf("c=%v ps=%v err=%v", c, ps, err)
	}
}

// The tests of the rules: each breaks the valid cassette in one place and
// expects exactly the listed problems.
func TestLoadRules(t *testing.T) {
	long := func(n int) string { return strings.Repeat("a", n) }
	tests := []struct {
		name string
		mut  func(t *testing.T, fsys fstest.MapFS)
		want []want
	}{
		// --- the manifest's fields
		{"format 0", manifestEdit(func(m map[string]any) { m["format"] = 0 }),
			[]want{{"cassette.json#format", "must be 1", false}}},
		{"format is a string", manifestEdit(func(m map[string]any) { m["format"] = "1" }),
			[]want{{"cassette.json#format", "must be a number, not a string", false}}},
		{"format is not whole", manifestEdit(func(m map[string]any) { m["format"] = 1.5 }),
			[]want{{"cassette.json#format", "whole number", false}}},
		{"format missing", manifestEdit(func(m map[string]any) { delete(m, "format") }),
			[]want{{"cassette.json#format", "is required", false}}},
		{"id missing", manifestEdit(func(m map[string]any) { delete(m, "id") }),
			[]want{{"cassette.json#id", "is required", false}}},
		{"id without a dot", manifestEdit(func(m map[string]any) { m["id"] = "fractions" }),
			[]want{{"cassette.json#id", "not a cassette id", false}}},
		{"id with capitals", manifestEdit(func(m map[string]any) { m["id"] = "org.Example.x" }),
			[]want{{"cassette.json#id", "not a cassette id", false}}},
		{"id starts with a reserved Windows name", manifestEdit(func(m map[string]any) { m["id"] = "con.example.x" }),
			[]want{{"cassette.json#id", "reserves for a device", false}}},
		{"id part starts with a hyphen", manifestEdit(func(m map[string]any) { m["id"] = "org.-x" }),
			[]want{{"cassette.json#id", "not a cassette id", false}}},
		{"version missing", manifestEdit(func(m map[string]any) { delete(m, "version") }),
			[]want{{"cassette.json#version", "is required", false}}},
		{"version with two numbers", manifestEdit(func(m map[string]any) { m["version"] = "1.0" }),
			[]want{{"cassette.json#version", "three numbers", false}}},
		{"version with a suffix", manifestEdit(func(m map[string]any) { m["version"] = "1.0.0-beta" }),
			[]want{{"cassette.json#version", "digits", false}}},
		{"version with a leading zero", manifestEdit(func(m map[string]any) { m["version"] = "1.02.0" }),
			[]want{{"cassette.json#version", "leading zeros", false}}},
		{"version is a number", manifestEdit(func(m map[string]any) { m["version"] = 1 }),
			[]want{{"cassette.json#version", "must be a string", false}}},
		{"title empty", manifestEdit(func(m map[string]any) { m["title"] = "" }),
			[]want{{"cassette.json#title", "must not be empty", false}}},
		{"title blank", manifestEdit(func(m map[string]any) { m["title"] = " \t" }),
			[]want{{"cassette.json#title", "must not be empty", false}}},
		{"title 81 characters", manifestEdit(func(m map[string]any) { m["title"] = long(81) }),
			[]want{{"cassette.json#title", "81 characters", false}}},
		{"title of 80 two-byte characters is fine", manifestEdit(func(m map[string]any) { m["title"] = strings.Repeat("é", 80) }), nil},
		{"title of 81 Greek letters", manifestEdit(func(m map[string]any) { m["title"] = strings.Repeat("ά", 81) }),
			[]want{{"cassette.json#title", "81 characters", false}}},
		{"description 501", manifestEdit(func(m map[string]any) { m["description"] = long(501) }),
			[]want{{"cassette.json#description", "501 characters", false}}},
		{"description 500 Greek letters is fine", manifestEdit(func(m map[string]any) { m["description"] = strings.Repeat("ω", 500) }), nil},
		{"description is optional", manifestEdit(func(m map[string]any) { delete(m, "description") }), nil},
		{"language with an underscore", manifestEdit(func(m map[string]any) { m["language"] = "en_GB" }),
			[]want{{"cassette.json#language", "BCP 47", false}}},
		{"language not a tag", manifestEdit(func(m map[string]any) { m["language"] = "english language" }),
			[]want{{"cassette.json#language", "BCP 47", false}}},
		{"language empty", manifestEdit(func(m map[string]any) { m["language"] = "" }),
			[]want{{"cassette.json#language", "BCP 47", false}}},
		{"language with a region is fine", manifestEdit(func(m map[string]any) { m["language"] = "en-GB" }), nil},
		{"language Greek is fine", manifestEdit(func(m map[string]any) { m["language"] = "el" }), nil},
		{"language missing", manifestEdit(func(m map[string]any) { delete(m, "language") }),
			[]want{{"cassette.json#language", "is required", false}}},
		{"subject empty", manifestEdit(func(m map[string]any) { m["subject"] = "" }),
			[]want{{"cassette.json#subject", "must not be empty", false}}},
		{"subject 41", manifestEdit(func(m map[string]any) { m["subject"] = long(41) }),
			[]want{{"cassette.json#subject", "41 characters", false}}},
		{"ages one number", manifestEdit(func(m map[string]any) { m["ages"] = []any{8} }),
			[]want{{"cassette.json#ages", "2 numbers", false}}},
		{"ages three numbers", manifestEdit(func(m map[string]any) { m["ages"] = []any{8, 9, 10} }),
			[]want{{"cassette.json#ages", "2 numbers", false}}},
		{"ages too young", manifestEdit(func(m map[string]any) { m["ages"] = []any{2, 10} }),
			[]want{{"cassette.json#ages[0]", "not an age from 3 to 18", false}}},
		{"ages too old", manifestEdit(func(m map[string]any) { m["ages"] = []any{8, 19} }),
			[]want{{"cassette.json#ages[1]", "not an age from 3 to 18", false}}},
		{"ages reversed", manifestEdit(func(m map[string]any) { m["ages"] = []any{10, 8} }),
			[]want{{"cassette.json#ages", "above the maximum", false}}},
		{"ages equal is fine", manifestEdit(func(m map[string]any) { m["ages"] = []any{8, 8} }), nil},
		{"ages not whole", manifestEdit(func(m map[string]any) { m["ages"] = []any{8, 9.5} }),
			[]want{{"cassette.json#ages[1]", "whole number", false}}},
		{"ages are strings", manifestEdit(func(m map[string]any) { m["ages"] = []any{"8", "10"} }),
			[]want{{"cassette.json#ages[0]", "must be a number", false}, {"cassette.json#ages[1]", "must be a number", false}}},
		{"ages missing", manifestEdit(func(m map[string]any) { delete(m, "ages") }),
			[]want{{"cassette.json#ages", "is required", false}}},
		{"authors not strings", manifestEdit(func(m map[string]any) { m["authors"] = []any{"a", 2} }),
			[]want{{"cassette.json#authors[1]", "must be a string", false}}},
		{"authors not a list", manifestEdit(func(m map[string]any) { m["authors"] = "me" }),
			[]want{{"cassette.json#authors", "must be an array", false}}},
		{"source 301", manifestEdit(func(m map[string]any) { m["source"] = long(301) }),
			[]want{{"cassette.json#source", "301 characters", false}}},
		{"grader instructions 4001", manifestEdit(func(m map[string]any) { m["grader_instructions"] = long(4001) }),
			[]want{{"cassette.json#grader_instructions", "4001 characters", false}}},
		{"grader instructions 4000", manifestEdit(func(m map[string]any) { m["grader_instructions"] = long(4000) }), nil},
		{"unknown field", manifestEdit(func(m map[string]any) { m["colour"] = "red" }),
			[]want{{"cassette.json#colour", "unknown field", false}}},
		{"field of the wrong case is unknown and the right one missing", manifestEdit(func(m map[string]any) {
			m["Title"] = m["title"]
			delete(m, "title")
		}), []want{{"cassette.json#Title", "unknown field", false}, {"cassette.json#title", "is required", false}}},

		// --- the manifest's items
		{"items missing", manifestEdit(func(m map[string]any) { delete(m, "items") }),
			[]want{{"cassette.json#items", "is required", false}}},
		{"items empty", manifestEdit(func(m map[string]any) { m["items"] = []any{} }),
			[]want{{"cassette.json#items", "0 items", false}, {"lessons/adding-fractions", "not listed", false}, {"lessons/what-is-a-fraction", "not listed", false},
				{"exercises/name-the-fraction", "not listed", false}, {"exercises/shade-the-half", "not listed", false}, {"exercises/write-the-sum", "not listed", false}}},
		{"item not an object", manifestEdit(func(m map[string]any) { m["items"].([]any)[1] = "x" }),
			[]want{{"cassette.json#items[1]", "must be a JSON object", false}}},
		{"item type unknown", manifestEdit(func(m map[string]any) { obj(m, "items", 1)["type"] = "video" }),
			[]want{{"cassette.json#items[1].type", `"video" is not lesson or exercise`, false}, {"exercises/shade-the-half", "not listed", false}}},
		{"item type missing", manifestEdit(func(m map[string]any) { delete(obj(m, "items", 1), "type") }),
			[]want{{"cassette.json#items[1].type", "is required", false}}},
		{"item id invalid", manifestEdit(func(m map[string]any) { obj(m, "items", 1)["id"] = "Shade_The_Half" }),
			[]want{{"cassette.json#items[1].id", "is not an id", false}, {"exercises/shade-the-half", "not listed", false}}},
		{"item id is a reserved Windows name", manifestEdit(func(m map[string]any) { obj(m, "items", 1)["id"] = "nul" }),
			[]want{{"cassette.json#items[1].id", "reserves for a device", false}, {"exercises/shade-the-half", "not listed", false}}},
		{"item id starts with a hyphen", manifestEdit(func(m map[string]any) { obj(m, "items", 1)["id"] = "-x" }),
			[]want{{"cassette.json#items[1].id", "is not an id", false}, {"exercises/shade-the-half", "not listed", false}}},
		{"item id of 64 characters", manifestEdit(func(m map[string]any) { obj(m, "items", 1)["id"] = long(64) }),
			[]want{{"cassette.json#items[1].id", "is not an id", false}, {"exercises/shade-the-half", "not listed", false}}},
		{"item id missing", manifestEdit(func(m map[string]any) { delete(obj(m, "items", 1), "id") }),
			[]want{{"cassette.json#items[1].id", "is required", false}}},
		{"item with an unknown field", manifestEdit(func(m map[string]any) { obj(m, "items", 1)["title"] = "x" }),
			[]want{{"cassette.json#items[1].title", "unknown field", false}}},
		{"duplicate item id", manifestEdit(func(m map[string]any) { obj(m, "items", 2)["id"] = "shade-the-half" }),
			[]want{{"cassette.json#items[2].id", `"shade-the-half" is already used by items[1]`, false}, {"exercises/name-the-fraction", "not listed", false}}},
		{"a lesson and an exercise share an id", manifestEdit(func(m map[string]any) { obj(m, "items", 4)["id"] = "adding-fractions" }),
			[]want{{"cassette.json#items[4].id", "already used by items[3]", false}, {"exercises/write-the-sum", "not listed", false}}},
		{"first item is an exercise", manifestEdit(func(m map[string]any) {
			items := m["items"].([]any)
			items[0], items[1] = items[1], items[0]
		}), []want{{"cassette.json#items[0]", "first item should be a lesson", true}}},
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

// manifestEdit makes a mutation that edits cassette.json.
func manifestEdit(fn func(m map[string]any)) func(t *testing.T, fsys fstest.MapFS) {
	return fileEdit(manifest, fn)
}

func fileEdit(name string, fn func(m map[string]any)) func(t *testing.T, fsys fstest.MapFS) {
	return func(t *testing.T, fsys fstest.MapFS) {
		t.Helper()
		edit(t, fsys, name, fn)
	}
}

func fsysDir(t *testing.T) fs.FS {
	t.Helper()
	return os.DirFS("testdata/minimal")
}

func TestLoadTooManyItems(t *testing.T) {
	fsys := minimalFS(t)
	edit(t, fsys, manifest, func(m map[string]any) {
		items := m["items"].([]any)
		for i := 0; len(items) < MaxItems+1; i++ {
			items = append(items, map[string]any{"type": "exercise", "id": "x" + strconv.Itoa(i)})
		}
		m["items"] = items
	})
	_, ps := load(t, fsys)
	found := false
	for _, p := range ps {
		found = found || (p.Path == "cassette.json#items" && strings.Contains(p.Message, "501 items"))
	}
	if !found {
		t.Errorf("no problem about 501 items in %d problems", len(ps))
	}
}

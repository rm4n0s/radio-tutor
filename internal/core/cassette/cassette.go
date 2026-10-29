// Package cassette reads and checks the JSON documents of a cassette
// (CASSETTE-FORMAT §2, §3, §5.3 to §5.5, §7, §8 and the checks 2 to 4 of §10):
// the manifest cassette.json, each lesson.json and each exercise.json. It
// reads from an fs.FS, so the same code reads a folder (os.DirFS), a zip and
// the store. The scene, the pictures, the audio and the archive limits belong
// to the scene, audio and archive packages.
//
// Load collects every problem it finds instead of stopping at the first. A
// problem's path names the file and, after a '#', the field inside it:
// "lessons/l1/lesson.json#captions[2].end". The checks are the ones that need
// only the documents; the rules that need the narration's length are left to
// the archive (timeline.CheckLength).
//
// Required fields are detected even where JSON's zero value would be valid (a
// key without "t", a caption without "start"): each file is first read into a
// tree that remembers which fields exist, checked against a schema (unknown
// fields, missing required fields, wrong types, duplicate fields), and only
// then decoded into the Go types, so the timeline types stay plain.
package cassette

// Cassette is a cassette's documents: the manifest and the lessons and
// exercises it lists, by id.
type Cassette struct {
	// Manifest is cassette.json.
	Manifest Manifest
	// Lessons are the parsed lesson.json files, by lesson id.
	Lessons map[string]*Lesson
	// Exercises are the parsed exercise.json files, by exercise id, with the
	// default labels applied.
	Exercises map[string]*Exercise
}

// Worksheet is a maximal run of consecutive exercise items (SPEC §7.1): the
// exercises the child sees together, solves on the same paper and photographs
// together.
type Worksheet struct {
	// Index is the worksheet's position among the cassette's worksheets,
	// counting from 0.
	Index int
	// Start is the index in Manifest.Items of the first exercise.
	Start int
	// ExerciseIDs are the ids of the exercises, in order.
	ExerciseIDs []string
}

// Worksheets returns the cassette's worksheets in order. They are computed
// from the manifest's items.
func (c *Cassette) Worksheets() []Worksheet {
	var out []Worksheet
	for i, it := range c.Manifest.Items {
		if it.Type != ItemExercise {
			continue
		}
		if i > 0 && c.Manifest.Items[i-1].Type == ItemExercise {
			w := &out[len(out)-1]
			w.ExerciseIDs = append(w.ExerciseIDs, it.ID)
			continue
		}
		out = append(out, Worksheet{Index: len(out), Start: i, ExerciseIDs: []string{it.ID}})
	}
	return out
}

// WorksheetOf returns the worksheet that holds the exercise, and false when no
// exercise item has that id.
func (c *Cassette) WorksheetOf(exerciseID string) (Worksheet, bool) {
	for _, w := range c.Worksheets() {
		for _, id := range w.ExerciseIDs {
			if id == exerciseID {
				return w, true
			}
		}
	}
	return Worksheet{}, false
}

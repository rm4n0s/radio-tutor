package cassette

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/problem"
)

// MaxJSONBytes is the size limit of cassette.json, lesson.json and
// exercise.json (CASSETTE-FORMAT §9): 1 MiB.
const MaxJSONBytes = 1 << 20

// File names inside a cassette (CASSETTE-FORMAT §1).
const (
	manifestFile = "cassette.json"
	lessonFile   = "lesson.json"
	exerciseFile = "exercise.json"
	lessonsDir   = "lessons"
	exercisesDir = "exercises"
)

const utf8BOM = "\xef\xbb\xbf"

// Load reads the documents of the cassette in fsys (its root is the cassette's
// root: cassette.json, lessons/, exercises/) and checks every rule that needs
// only those documents. It returns every problem it finds; a cassette is valid
// when problem.HasErrors is false for them.
//
// The returned error is for failures that are not about the cassette's
// content: the context was cancelled or reading fsys failed. A missing file, a
// bad JSON file or a broken rule is a problem.
//
// The returned Cassette is nil when cassette.json is missing, is not valid
// JSON of the right shape, or has a newer format. Otherwise it holds the
// manifest and every lesson and exercise whose file could be read and has the
// right shape, even when a field rule failed: use it only when the problems
// have no errors.
func Load(ctx context.Context, fsys fs.FS) (*Cassette, []problem.Problem, error) {
	ld := &loader{ctx: ctx, fsys: fsys}
	var l problem.List

	var m Manifest
	root, ps, err := ld.document(manifestFile, manifestSchema, &m)
	if err != nil {
		return nil, nil, err
	}
	l.Add(ps...)
	if root == nil {
		return nil, l, nil
	}
	var mp problem.List
	m.validate(&mp)
	l.Add(problem.InFile(manifestFile, mp)...)

	c := &Cassette{Manifest: m, Lessons: map[string]*Lesson{}, Exercises: map[string]*Exercise{}}
	lessonFolders, ps, err := ld.folders(lessonsDir)
	if err != nil {
		return nil, nil, err
	}
	l.Add(ps...)
	exerciseFolders, ps, err := ld.folders(exercisesDir)
	if err != nil {
		return nil, nil, err
	}
	l.Add(ps...)

	listed := map[ItemType]map[string]bool{ItemLesson: {}, ItemExercise: {}}
	labels := map[string]string{} // label -> id of the exercise that has it
	position := 0                 // of the exercise among all the exercises
	for i, it := range m.Items {
		if it.Type == ItemExercise {
			position++
		}
		if (it.Type != ItemLesson && it.Type != ItemExercise) || !idPattern.MatchString(it.ID) || listed[ItemLesson][it.ID] || listed[ItemExercise][it.ID] {
			continue // reported by Manifest.validate
		}
		listed[it.Type][it.ID] = true

		dirs, kind, base := lessonFolders, "lesson", lessonsDir
		if it.Type == ItemExercise {
			dirs, kind, base = exerciseFolders, "exercise", exercisesDir
		}
		isDir, exists := dirs[it.ID]
		switch {
		case !exists:
			l.Errorf(manifestFile+"#"+problem.Index("items", i)+".id", "%s %q has no folder %s/%s", kind, it.ID, base, it.ID)
			continue
		case !isDir:
			l.Errorf(base+"/"+it.ID, "must be a folder, not a file")
			continue
		}

		if it.Type == ItemLesson {
			if err := ld.loadLesson(c, &l, it.ID); err != nil {
				return nil, nil, err
			}
			continue
		}
		ex, err := ld.loadExercise(&l, it.ID, position, labels)
		if err != nil {
			return nil, nil, err
		}
		if ex != nil {
			c.Exercises[it.ID] = ex
		}
	}

	l.Add(unlisted(lessonsDir, lessonFolders, listed[ItemLesson])...)
	l.Add(unlisted(exercisesDir, exerciseFolders, listed[ItemExercise])...)
	return c, l, nil
}

// LoadManifest reads only cassette.json of the cassette in fsys and applies
// the manifest's own field rules (not the folders of its items). It is what
// the store uses to list installed cassettes without reading every lesson. The
// returned Manifest is nil when the file is missing, is not valid JSON of the
// right shape, or has a newer format; otherwise it is returned even when a
// field rule failed, so use it only when the problems have no errors. The
// error is for failures that are not about the content (see Load).
func LoadManifest(ctx context.Context, fsys fs.FS) (*Manifest, []problem.Problem, error) {
	ld := &loader{ctx: ctx, fsys: fsys}
	var m Manifest
	root, ps, err := ld.document(manifestFile, manifestSchema, &m)
	if err != nil {
		return nil, nil, err
	}
	if root == nil {
		return nil, ps, nil
	}
	var mp problem.List
	m.validate(&mp)
	return &m, append(ps, problem.InFile(manifestFile, mp)...), nil
}

// loader reads the files of one cassette from an fs.FS.
type loader struct {
	ctx  context.Context
	fsys fs.FS
}

func (ld *loader) loadLesson(c *Cassette, l *problem.List, id string) error {
	name := lessonsDir + "/" + id + "/" + lessonFile
	var le Lesson
	root, ps, err := ld.document(name, lessonSchema, &le)
	if err != nil {
		return err
	}
	l.Add(ps...)
	if root == nil {
		return nil
	}
	le.ID = id
	var lp problem.List
	le.validate(&lp)
	l.Add(problem.InFile(name, lp)...)
	c.Lessons[id] = &le
	return nil
}

// loadExercise reads an exercise, applies its defaults and checks its label
// against the labels of the exercises before it. It returns nil when the file
// could not be read into an Exercise.
func (ld *loader) loadExercise(l *problem.List, id string, position int, labels map[string]string) (*Exercise, error) {
	name := exercisesDir + "/" + id + "/" + exerciseFile
	var ex Exercise
	root, ps, err := ld.document(name, exerciseSchema, &ex)
	if err != nil {
		return nil, err
	}
	l.Add(ps...)
	if root == nil {
		return nil, nil
	}
	ex.ID = id
	if root.field("max_attempts") == nil {
		ex.MaxAttempts = DefaultMaxAttempts
	}
	labelSet := root.field("label") != nil
	var ep problem.List
	ex.validate(&ep, labelSet)

	if !labelSet {
		ex.Label = defaultLabel(position)
	}
	if labelSet && !validLabel(ex.Label) {
		l.Add(problem.InFile(name, ep)...)
		return &ex, nil
	}
	if first, dup := labels[ex.Label]; dup {
		if labelSet {
			ep.Errorf("label", "%q is already the label of exercise %q; labels are unique in a cassette", ex.Label, first)
		} else {
			ep.Errorf("label", "the default label %q (the exercise's position) is already the label of exercise %q; give one of them a label", ex.Label, first)
		}
	} else {
		labels[ex.Label] = id
	}
	l.Add(problem.InFile(name, ep)...)
	return &ex, nil
}

// unlisted reports the folders of dir that no item names.
func unlisted(dir string, folders map[string]bool, listed map[string]bool) []problem.Problem {
	var l problem.List
	for name, isDir := range folders {
		if isDir && !listed[name] {
			l.Errorf(dir+"/"+name, "the folder is not listed in the items of %s", manifestFile)
		}
	}
	sortByPath(l)
	return l
}

// folders lists the entries directly inside dir as name -> is a folder. A
// missing dir is empty.
func (ld *loader) folders(dir string) (map[string]bool, []problem.Problem, error) {
	entries, err := fs.ReadDir(ld.fsys, dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, nil
		}
		if info, serr := fs.Stat(ld.fsys, dir); serr == nil && !info.IsDir() {
			return nil, problem.List{problem.Errorf(dir, "must be a folder, not a file")}, nil
		}
		return nil, nil, errors.NewErr("CassetteFolderReadFailed", err).SetMetadata("path", dir)
	}
	out := make(map[string]bool, len(entries))
	for _, e := range entries {
		out[e.Name()] = e.IsDir()
	}
	return out, nil, nil
}

// readFile reads the file name, at most MaxJSONBytes. A file that is missing,
// is a folder or is too large is reported as a problem message (and nil
// data); only a failure to read is an error.
func (ld *loader) readFile(name string) (data []byte, msg string, err error) {
	if err := ld.ctx.Err(); err != nil {
		return nil, "", errors.NewErr("CassetteLoadCanceled", err).SetMetadata("path", name)
	}
	f, err := ld.fsys.Open(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "is missing", nil
		}
		return nil, "", errors.NewErr("CassetteFileOpenFailed", err).SetMetadata("path", name)
	}
	defer func() { _ = f.Close() }() // read only; a failed close loses nothing
	info, err := f.Stat()
	if err != nil {
		return nil, "", errors.NewErr("CassetteFileStatFailed", err).SetMetadata("path", name)
	}
	if info.IsDir() {
		return nil, "must be a file, not a folder", nil
	}
	data, err = io.ReadAll(io.LimitReader(f, MaxJSONBytes+1))
	if err != nil {
		return nil, "", errors.NewErr("CassetteFileReadFailed", err).SetMetadata("path", name)
	}
	if len(data) > MaxJSONBytes {
		return nil, "is larger than 1 MiB", nil
	}
	return data, "", nil
}

// document reads the JSON file name, checks it against s and decodes it into
// dst (a pointer to the matching struct). The returned problems already carry
// the file name. The returned node is nil when dst was not filled: the file is
// missing, is not UTF-8 JSON, does not have the right shape, or needs a newer
// Radio Tutor; the problems say which. The field rules (lengths, ranges) are
// not applied here.
func (ld *loader) document(name string, s *schema, dst any) (*node, []problem.Problem, error) {
	data, msg, err := ld.readFile(name)
	if err != nil {
		return nil, nil, err
	}
	if msg != "" {
		return nil, problem.List{problem.Errorf(name, "%s", msg)}, nil
	}
	root, ps := decode(data, s, dst)
	return root, problem.InFile(name, ps), nil
}

// decode checks data against s and decodes it into dst. It returns a nil node
// with the problems when data is not UTF-8 without a byte order mark, is not
// JSON, does not match s, or declares a newer format.
func decode(data []byte, s *schema, dst any) (*node, problem.List) {
	var l problem.List
	if bytes.HasPrefix(data, []byte(utf8BOM)) {
		l.Errorf("", "starts with a byte order mark; save the file as UTF-8 without BOM")
		return nil, l
	}
	if !utf8.Valid(data) {
		off := 0
		for off < len(data) {
			r, size := utf8.DecodeRune(data[off:])
			if r == utf8.RuneError && size == 1 {
				break
			}
			off += size
		}
		l.Errorf("", "is not valid UTF-8 (first bad byte on line %d)", 1+bytes.Count(data[:off], []byte("\n")))
		return nil, l
	}

	root, jp := parseJSON(data)
	l.Add(jp...)
	if root == nil {
		return nil, l
	}
	if root.kind == nodeObject {
		if n := root.fields["format"]; n != nil && n.kind == nodeNumber {
			if v, err := strconv.ParseInt(n.str, 10, 32); err == nil && v > FormatVersion {
				var newer problem.List
				newer.Errorf("format", "This cassette needs a newer Radio Tutor (it has format %d; this one reads format %d)", v, FormatVersion)
				return nil, newer
			}
		}
	}
	s.check(&l, "", root)
	if l.HasErrors() {
		return nil, l
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		// The schema mirrors the structs, so this is a bug in the schema.
		l.Errorf("", "cannot be read: %s", err.Error())
		return nil, l
	}
	return root, l
}

// sortByPath orders problems by path, so that output does not depend on map
// iteration.
func sortByPath(l problem.List) {
	slices.SortFunc(l, func(a, b problem.Problem) int { return strings.Compare(a.Path, b.Path) })
}

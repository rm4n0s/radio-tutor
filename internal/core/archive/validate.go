package archive

import (
	"context"
	"io"
	"io/fs"
	"os"
	"strconv"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/audio"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/problem"
	"github.com/rm4n0s/radio-tutor/internal/core/scene"
	"github.com/rm4n0s/radio-tutor/internal/core/timeline"
)

// RenderChecker checks that Radio Tutor's renderer can draw an SVG
// (CASSETTE-FORMAT §10 item 5). The core does not know the renderer, so the
// program passes one in: internal/gui/svgcheck implements it with Glutter's
// SVG parser. A nil RenderChecker skips the check.
type RenderChecker interface {
	// CheckSVG draws data, an SVG document, and returns an error that says
	// why when it cannot. name says where the document is from, for the log.
	// An error is reported as a problem of the cassette, unless ctx is done,
	// which cancels the validation.
	CheckSVG(ctx context.Context, name string, data []byte) error
}

// Report is the result of Validate.
type Report struct {
	// Problems are all the problems found, errors and warnings, in a stable
	// order. A path is a file name, optionally followed by '#' and a field
	// path.
	Problems []problem.Problem
	// Cassette holds the documents of the cassette when cassette.json could be
	// read; it is nil otherwise. It may be partial or wrong when Problems has
	// errors, use it only when Valid is true.
	Cassette *cassette.Cassette
}

// Valid reports whether the cassette has no errors (warnings do not count).
func (r Report) Valid() bool { return !problem.HasErrors(r.Problems) }

// Validate runs every check of CASSETTE-FORMAT §10 on the cassette in fsys,
// whose root is the cassette's root, and reports every problem it finds.
//
//   - The names, the layout and the sizes of CASSETTE-FORMAT §9 and §1 (the
//     folder rules of scanFS: only the files of §1, no symbolic links, at most
//     2000 entries, 200 MiB a file, 1 GiB in all). Files that fail are not
//     read.
//   - cassette.Load: the manifest, the lessons and the exercises.
//   - For every lesson: scene.svg (scene.Split, then the renderer for every
//     layer, then scene.CheckTracks against the lesson's tracks) and
//     narration.ogg (audio.Probe, audio.CheckNarration, then
//     timeline.CheckLength with the narration's real length).
//   - cover.svg (scene.CheckCover and the renderer).
//   - For every exercise: prompt.svg and prompt.ogg when present.
//
// The error is for failures that are not about the cassette: a failure to read
// fsys, or a done ctx. Validate does not read what is not listed in the
// manifest.
func Validate(ctx context.Context, fsys fs.FS, render RenderChecker) (Report, error) {
	view, ps, _, err := scanFS(ctx, fsys)
	if err != nil {
		return Report{}, err
	}
	return validateView(ctx, view, ps, render)
}

// ValidatePath validates the cassette at path, a .cassette file or a folder,
// with the problems of opening it (OpenZip or the folder rules) first. It is
// what `radio-tutor validate` runs.
func ValidatePath(ctx context.Context, path string, render RenderChecker) (Report, error) {
	src, err := openPath(ctx, path)
	if err != nil {
		return Report{}, err
	}
	defer func() { _ = src.closer.Close() }() // read only
	if src.fatal {
		return Report{Problems: src.problems}, nil
	}
	return validateView(ctx, src.fsys, src.problems, render)
}

// source is an opened zip or folder.
type source struct {
	fsys     *treeFS
	closer   io.Closer
	problems []problem.Problem
	// fatal is true when the whole thing was refused and the view is empty.
	fatal bool
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// openPath opens a .cassette file with OpenZip, or a folder with the folder
// rules.
func openPath(ctx context.Context, path string) (source, error) {
	info, err := os.Stat(path)
	if err != nil {
		return source{}, errors.NewErr("ArchivePathStatFailed", err).SetMetadata("path", path)
	}
	if info.IsDir() {
		t, ps, fatal, err := scanFS(ctx, os.DirFS(path))
		if err != nil {
			return source{}, err
		}
		return source{fsys: t, closer: nopCloser{}, problems: ps, fatal: fatal}, nil
	}
	t, closer, ps, fatal, err := openZip(ctx, path)
	if err != nil {
		return source{}, err
	}
	return source{fsys: t, closer: closer, problems: ps, fatal: fatal}, nil
}

// validateView runs the checks after the entries were scanned.
func validateView(ctx context.Context, view *treeFS, scanProblems []problem.Problem, render RenderChecker) (Report, error) {
	v := &validator{ctx: ctx, fsys: view, render: render}
	v.ps.Add(scanProblems...)
	c, ps, err := cassette.Load(ctx, view)
	if err != nil {
		return Report{}, err
	}
	v.ps.Add(ps...)
	if c != nil {
		if err := v.items(c); err != nil {
			return Report{}, err
		}
	}
	if err := v.cover(); err != nil {
		return Report{}, err
	}
	return Report{Problems: v.ps, Cassette: c}, nil
}

// validator holds the state of one Validate.
type validator struct {
	ctx    context.Context
	fsys   *treeFS
	render RenderChecker
	ps     problem.List
}

// items checks the files of every lesson and exercise the manifest lists, in
// the manifest's order.
func (v *validator) items(c *cassette.Cassette) error {
	done := map[string]bool{}
	for _, it := range c.Manifest.Items {
		key := string(it.Type) + "/" + it.ID
		if !cassette.ValidID(it.ID) || done[key] {
			continue
		}
		done[key] = true
		var err error
		switch it.Type {
		case cassette.ItemLesson:
			err = v.lesson(it.ID, c.Lessons[it.ID])
		case cassette.ItemExercise:
			err = v.exercise(it.ID)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// lesson checks scene.svg and narration.ogg of a lesson folder against the
// lesson's document le, which is nil when lesson.json could not be read.
func (v *validator) lesson(id string, le *cassette.Lesson) error {
	dir := lessonsDir + "/" + id
	if info, err := fs.Stat(v.fsys, dir); err != nil || !info.IsDir() {
		return nil // reported by cassette.Load
	}
	lessonName := dir + "/" + lessonFile

	sceneName := dir + "/" + sceneFile
	data, ok, err := v.readSVG(sceneName, false)
	if err != nil {
		return err
	}
	if ok {
		sc, sps, err := scene.Split(data)
		if err != nil {
			return err
		}
		v.ps.Add(problem.InFile(sceneName, sps)...)
		if sc != nil && !problem.HasErrors(sps) {
			if err := v.renderLayers(sceneName, sc); err != nil {
				return err
			}
		}
		if le != nil {
			v.ps.Add(problem.InFile(lessonName, scene.CheckTracks(sc, le.Tracks))...)
		}
	}

	info, ok, err := v.probe(dir+"/"+narrationFile, audio.MaxNarrationSeconds, false)
	if err != nil {
		return err
	}
	if ok && le != nil {
		v.ps.Add(problem.InFile(lessonName, timeline.CheckLength(le.Tracks, le.Captions, info.Seconds()))...)
	}
	return nil
}

// exercise checks the optional prompt.svg and prompt.ogg of an exercise.
func (v *validator) exercise(id string) error {
	dir := exercisesDir + "/" + id
	if info, err := fs.Stat(v.fsys, dir); err != nil || !info.IsDir() {
		return nil // reported by cassette.Load
	}
	name := dir + "/" + promptSVGFile
	data, ok, err := v.readSVG(name, true)
	if err != nil {
		return err
	}
	if ok {
		v.ps.Add(problem.InFile(name, scene.CheckPicture(data))...)
		if err := v.renderOne(name, data); err != nil {
			return err
		}
	}
	_, _, err = v.probe(dir+"/"+promptAudioOgg, audio.MaxPromptSeconds, true)
	return err
}

// cover checks cover.svg.
func (v *validator) cover() error {
	data, ok, err := v.readSVG(coverFile, false)
	if err != nil || !ok {
		return err
	}
	v.ps.Add(problem.InFile(coverFile, scene.CheckCover(data))...)
	return v.renderOne(coverFile, data)
}

// renderLayers gives every layer of a scene to the renderer.
func (v *validator) renderLayers(file string, sc *scene.Scene) error {
	static := 0
	for _, l := range sc.Layers {
		label := file + "#svg/g[@id=" + l.ActorID + "]"
		if l.ActorID == "" {
			static++
			label = file + "#static layer " + strconv.Itoa(static)
		}
		if err := v.renderOne(label, l.SVG); err != nil {
			return err
		}
	}
	return nil
}

// renderOne gives one document to the renderer, if there is one.
func (v *validator) renderOne(name string, data []byte) error {
	if v.render == nil {
		return nil
	}
	if err := cancelCheck(v.ctx, "rendering "+name); err != nil {
		return err
	}
	if err := v.render.CheckSVG(v.ctx, name, data); err != nil {
		if cerr := cancelCheck(v.ctx, "rendering "+name); cerr != nil {
			return cerr
		}
		v.ps.Errorf(name, "Radio Tutor cannot draw it: %s", err.Error())
	}
	return nil
}

// stat finds the file name for a check: ok is false, with the problem
// recorded, when it is missing (unless optional) or is a folder.
func (v *validator) stat(name string, optional bool) (fs.FileInfo, bool, error) {
	if err := cancelCheck(v.ctx, "checking "+name); err != nil {
		return nil, false, err
	}
	info, err := fs.Stat(v.fsys, name)
	switch {
	case err == nil:
	case errors.Is(err, fs.ErrNotExist):
		if !optional {
			v.ps.Errorf(name, "is missing")
		}
		return nil, false, nil
	default:
		return nil, false, err // from the view: already tagged
	}
	if info.IsDir() {
		v.ps.Errorf(name, "must be a file, not a folder")
		return nil, false, nil
	}
	return info, true, nil
}

// readSVG reads an SVG file of at most scene.MaxBytes.
func (v *validator) readSVG(name string, optional bool) ([]byte, bool, error) {
	info, ok, err := v.stat(name, optional)
	if err != nil || !ok {
		return nil, false, err
	}
	if info.Size() > scene.MaxBytes {
		v.ps.Errorf(name, "the file is %d bytes; the limit is %d bytes (5 MiB)", info.Size(), scene.MaxBytes)
		return nil, false, nil
	}
	data, err := fs.ReadFile(v.fsys, name)
	if err != nil {
		return nil, false, err // from the view: already tagged
	}
	return data, true, nil
}

// probe decodes an Ogg Vorbis file and applies audio.CheckNarration with the
// given limit. ok is true when the length is known.
func (v *validator) probe(name string, maxSeconds float64, optional bool) (audio.Info, bool, error) {
	info, ok, err := v.stat(name, optional)
	if err != nil || !ok {
		return audio.Info{}, false, err
	}
	if info.Size() > audio.MaxBytes {
		v.ps.Errorf(name, "the file is %d bytes; the limit is %d bytes (20 MiB)", info.Size(), audio.MaxBytes)
		return audio.Info{}, false, nil
	}
	f, err := v.fsys.Open(name)
	if err != nil {
		return audio.Info{}, false, err // from the view: already tagged
	}
	defer func() { _ = f.Close() }() // read only
	got, err := audio.Probe(v.ctx, f)
	if err != nil {
		if appErr, ok := errors.FromError(err); ok && appErr.Tag != "AudioCanceled" && appErr.Tag != "AudioReadFailed" {
			v.ps.Errorf(name, "cannot be decoded as Ogg Vorbis: %s", appErr.Message)
			return audio.Info{}, false, nil
		}
		return audio.Info{}, false, err
	}
	v.ps.Add(problem.InFile(name, audio.CheckNarration(got, maxSeconds))...)
	return got, true, nil
}

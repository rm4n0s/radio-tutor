// Package scene reads the SVG files of a cassette (CASSETTE-FORMAT §4, §5.1,
// §5.2, SPEC §8): it splits a lesson's scene.svg into layers (one per actor
// and one per run of static elements), checks the SVG rules (forbidden
// elements and attributes, external references, view box, actor ids), checks
// the pictures prompt.svg and cover.svg, and cross-checks a scene with the
// tracks of its lesson.
//
// # Layers
//
// The player draws each layer as a picture of its own and moves the pictures
// of the actors, so every Layer is a complete standalone SVG document: the
// root's start tag exactly as it was written (with its namespaces and viewBox,
// so all layers align), the top-level <defs> and <style> elements (copied into
// every layer, so that url(#id) and <use href="#id"> resolve), and the layer's
// elements. The markup of the elements is copied byte for byte from the
// input (not re-serialized), so prefixes such as xlink:href and entities
// such as &amp; survive. Top-level <title>, <desc>, <metadata> and elements of
// other namespaces (an editor's own data) are not drawn and are left out.
//
// # Paths
//
// Problems have paths that are relative to the SVG file; the caller that knows
// the file applies problem.InFile. A problem about an element has the path of
// the element: "svg" for the root, then one step per level, each the element's
// name followed by its id in brackets when it has one and its position among
// the siblings of the same name otherwise: "svg/g[@id=pizza]/rect[2]". A
// problem about an attribute adds "/@name": "svg/g[@id=pizza]/@onclick". The
// message ends with "(line N)", the line where the element's start tag ends.
// A problem about the file as a whole (not well-formed XML) has the path
// "line N" or an empty path.
package scene

import (
	"bytes"

	"github.com/rm4n0s/radio-tutor/internal/core/problem"
	"github.com/rm4n0s/radio-tutor/internal/core/timeline"
)

// The limits of CASSETTE-FORMAT §5.1 and §9.
const (
	// MaxBytes is the largest SVG file: 5 MiB.
	MaxBytes = 5 << 20
	// MaxActors is the most actors a scene may have.
	MaxActors = 64
	// MaxDepth is the deepest nesting of elements.
	MaxDepth = 128
	// MaxLayerBytes is the most bytes all the layers of a scene may have
	// together. The shared <defs> and <style> are copied into every layer, so
	// this bounds the memory a scene with big defs and many actors can use.
	MaxLayerBytes = 32 << 20
	// MaxProblems is the most problems one file reports before it says that
	// there are more.
	MaxProblems = 100
)

// Layer is one picture of the scene: an actor, or a run of static elements.
type Layer struct {
	// ActorID is the id of the actor, or empty for a static layer.
	ActorID string
	// SVG is a complete SVG document that draws the layer on its own.
	SVG []byte
}

// Scene is a scene.svg split into layers.
type Scene struct {
	// ViewBox is the root's viewBox, the coordinate system of the timeline.
	ViewBox ViewBox
	// Layers are in drawing order (the first is drawn first, at the bottom).
	Layers []Layer
	// Actors are the actor ids in document order.
	Actors []string
}

// HasActor reports whether the scene has an actor with the given id.
func (s *Scene) HasActor(id string) bool {
	for _, a := range s.Actors {
		if a == id {
			return true
		}
	}
	return false
}

// Split reads a scene.svg and splits it into layers, reporting every problem
// of CASSETTE-FORMAT §5.1 and §5.2 it finds (paths: see the package
// documentation).
//
// The returned Scene is nil when the document cannot be used at all: it is too
// big, not well-formed XML, not an <svg> in the SVG namespace, or without a
// valid viewBox. Otherwise it is returned even when there are errors (for
// example a forbidden element), so that further checks can run; the layers of
// a scene with errors must not be used. The error is for failures that are not
// the author's fault; reading a byte slice has none today, so it is nil.
func Split(data []byte) (*Scene, []problem.Problem, error) {
	p, ok := parse(data, true)
	if !ok {
		return nil, p.ps, nil
	}
	if !aspectInRange(p.viewBox.Aspect()) {
		p.ps.Errorf("svg/@viewBox", "the view box is %s x %s (aspect ratio %.2f:1); a scene needs an aspect ratio between 4:3 (1.33) and 21:9 (2.33), such as 0 0 1600 900",
			formatNumber(p.viewBox.Width), formatNumber(p.viewBox.Height), p.viewBox.Aspect())
	}
	s := &Scene{ViewBox: p.viewBox, Actors: p.actors}
	s.Layers = p.buildLayers()
	return s, p.ps, nil
}

// CheckPicture checks a prompt.svg (CASSETTE-FORMAT §7.2): the SVG rules of
// §5.2 and a valid viewBox of any aspect ratio. There are no actors: top-level
// groups are not special. The paths are those of Split.
func CheckPicture(data []byte) []problem.Problem {
	p, _ := parse(data, false)
	return p.ps
}

// CheckCover checks a cover.svg (CASSETTE-FORMAT §4): what CheckPicture
// checks, and a warning when the view box's aspect ratio is not 3:4.
func CheckCover(data []byte) []problem.Problem {
	p, ok := parse(data, false)
	if ok && !aspectIsCover(p.viewBox.Aspect()) {
		p.ps.Warnf("svg/@viewBox", "the cover's view box is %s x %s; the shelf expects an aspect ratio of 3:4, such as 0 0 300 400",
			formatNumber(p.viewBox.Width), formatNumber(p.viewBox.Height))
	}
	return p.ps
}

// parse runs the parser. ok is false when the file cannot be used at all (see
// Split).
func parse(data []byte, isScene bool) (*parser, bool) {
	p := newParser(data, isScene)
	if len(data) > MaxBytes {
		p.ps.Errorf("", "the file is %d bytes; the limit is %d bytes (5 MiB)", len(data), MaxBytes)
		return p, false
	}
	p.run()
	return p, !p.incomplete && p.hasView && p.rootDone
}

// buildLayers assembles the layers from the collected markup. When the layers
// together would be bigger than MaxLayerBytes it reports an error and returns
// none.
func (p *parser) buildLayers() []Layer {
	if !p.isScene || len(p.layers) == 0 {
		return nil
	}
	// A root without children has no layers, so its start tag does not end
	// with "/>" here.
	open := p.rootOpen
	closing := append([]byte("</"), rootName(open)...)
	closing = append(closing, '>', '\n')

	sharedLen := 0
	for _, s := range p.shared {
		sharedLen += len(s) + 1
	}
	total := 0
	for _, l := range p.layers {
		total += len(open) + 1 + sharedLen + len(closing)
		for _, part := range l.parts {
			total += len(part) + 1
		}
	}
	if total > MaxLayerBytes {
		p.ps.Errorf("svg", "the layers together would be %d bytes; the limit is %d bytes (32 MiB). The <defs> and <style> (%d bytes) are copied into every layer: make them smaller or use fewer actors",
			total, MaxLayerBytes, sharedLen)
		return nil
	}

	layers := make([]Layer, 0, len(p.layers))
	for _, l := range p.layers {
		var b bytes.Buffer
		b.Grow(total / len(p.layers))
		b.Write(open)
		b.WriteByte('\n')
		for _, s := range p.shared {
			b.Write(s)
			b.WriteByte('\n')
		}
		for _, part := range l.parts {
			b.Write(part)
			b.WriteByte('\n')
		}
		b.Write(closing)
		layers = append(layers, Layer{ActorID: l.actorID, SVG: b.Bytes()})
	}
	return layers
}

// rootName returns the name of an element from its start tag as written
// (with its prefix, if any): "<svg:svg xmlns:svg=...>" gives "svg:svg".
func rootName(open []byte) []byte {
	name := open[1:]
	if i := bytes.IndexAny(name, " \t\r\n/>"); i >= 0 {
		name = name[:i]
	}
	return name
}

// CheckTracks cross-checks the tracks of a lesson with its scene
// (CASSETTE-FORMAT §5.4, §10 item 6): every track's actor must be an actor of
// the scene, and a track's reveal_box must lie inside the scene's view box. The
// paths are those of the lesson file: "tracks[2].actor" and
// "tracks[2].reveal_box". The rules that need only the timeline (a reveal_box
// present when a key sets reveal, four numbers) are timeline.Validate's. A nil
// scene gives no problems.
func CheckTracks(s *Scene, tracks []timeline.Track) []problem.Problem {
	if s == nil {
		return nil
	}
	var ps problem.List
	vb := s.ViewBox
	const eps = 1e-9
	for i, t := range tracks {
		tp := problem.Index("tracks", i)
		if !s.HasActor(t.Actor) {
			ps.Errorf(problem.Join(tp, "actor"), "the scene has no actor \"%s\"; an actor is a top-level <g id=\"%s\"> in scene.svg", t.Actor, t.Actor)
		}
		if len(t.RevealBox) != 4 {
			continue
		}
		x, y, w, h := t.RevealBox[0], t.RevealBox[1], t.RevealBox[2], t.RevealBox[3]
		if x < vb.X-eps || y < vb.Y-eps || x+w > vb.X+vb.Width+eps || y+h > vb.Y+vb.Height+eps {
			ps.Errorf(problem.Join(tp, "reveal_box"), "the box [%s, %s, %s, %s] is not inside the view box (x from %s to %s, y from %s to %s)",
				formatNumber(x), formatNumber(y), formatNumber(w), formatNumber(h),
				formatNumber(vb.X), formatNumber(vb.X+vb.Width), formatNumber(vb.Y), formatNumber(vb.Y+vb.Height))
		}
	}
	return ps
}

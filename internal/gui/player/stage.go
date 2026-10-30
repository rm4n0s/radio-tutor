package player

import (
	"math"

	"github.com/rm4n0s/glutter/painting"
	"github.com/rm4n0s/glutter/rendering"
	"github.com/rm4n0s/glutter/svg"
	"github.com/rm4n0s/glutter/ui"
	"github.com/rm4n0s/glutter/widgets"
	"github.com/rm4n0s/radio-tutor/internal/core/scene"
	"github.com/rm4n0s/radio-tutor/internal/core/timeline"
)

// stage draws a scene (SPEC §8): the layers of scene.svg, each a picture of
// its own, stacked in a box that has the aspect ratio of the view box and is
// centered (contain fit, letterboxed). The pictures are built once; every frame
// only the wrappers of the actors change.
type stage struct {
	view   scene.ViewBox
	layers []layer
}

// layer is one picture of the scene and, for an actor that has a track, the
// index of the track in the evaluator.
type layer struct {
	picture widgets.Widget
	track   int // -1 for a static layer and for an actor no track animates
	origin  []float64
	box     []float64
	from    timeline.RevealFrom
}

// newStage makes the pictures of the layers and finds the track of each actor.
func newStage(sc *scene.Scene, tracks []timeline.Track) *stage {
	byActor := make(map[string]int, len(tracks))
	for i, t := range tracks {
		byActor[t.Actor] = i
	}
	st := &stage{view: sc.ViewBox, layers: make([]layer, len(sc.Layers))}
	for i, l := range sc.Layers {
		ly := layer{
			picture: svg.NewSvgPictureMemory(l.SVG, svg.SvgPictureOpts{
				Fit: painting.BoxFitContain,
				// A scene is meant to be drawn inside its view box only.
				ExcludeFromSemantics: true,
			}),
			track: -1,
		}
		if idx, ok := byActor[l.ActorID]; ok && l.ActorID != "" {
			t := tracks[idx]
			ly.track, ly.origin, ly.box, ly.from = idx, t.Origin, t.RevealBox, t.RevealFrom.Or()
		}
		st.layers[i] = ly
	}
	return st
}

// fit returns the size of the stage inside the available space: the view box's
// aspect ratio, as large as fits.
func (s *stage) fit(maxWidth, maxHeight float64) ui.Size {
	k := math.Min(maxWidth/s.view.Width, maxHeight/s.view.Height)
	if k <= 0 || math.IsInf(k, 0) || math.IsNaN(k) {
		return ui.Size{}
	}
	return ui.Size{Width: s.view.Width * k, Height: s.view.Height * k}
}

// widget builds the stage at the states of the actors (the evaluator's output,
// in track order) inside the available space.
func (s *stage) widget(maxWidth, maxHeight float64, states []timeline.ActorState) widgets.Widget {
	size := s.fit(maxWidth, maxHeight)
	if size.Width <= 0 || size.Height <= 0 {
		return widgets.NewSizedBox(nil)
	}
	k := size.Width / s.view.Width
	children := make([]widgets.Widget, len(s.layers))
	for i := range s.layers {
		ly := &s.layers[i]
		if ly.track < 0 || ly.track >= len(states) {
			children[i] = ly.picture
			continue
		}
		children[i] = s.actor(ly, states[ly.track], k)
	}
	return widgets.NewSizedBox(
		widgets.NewStack(children, widgets.StackOpts{Fit: rendering.StackFitExpand, ClipBehavior: ui.ClipHardEdge}),
		widgets.SizedBoxOpts{Width: someF(size.Width), Height: someF(size.Height)},
	)
}

// actor wraps the picture of an actor: opacity outside, then the transform, and
// inside it the reveal clip, so that the clip moves and turns with the actor
// (CASSETTE-FORMAT §5.4: the reveal box is in the actor's own coordinates).
func (s *stage) actor(ly *layer, st timeline.ActorState, k float64) widgets.Widget {
	w := ly.picture
	if len(ly.box) == 4 {
		w = widgets.NewClipRect(w, widgets.ClipRectOpts{
			Clipper:      newRevealClipper(s.view, ly.box, ly.from, st.Reveal, k),
			ClipBehavior: ui.ClipHardEdge,
		})
	}
	return widgets.NewOpacity(clamp01(st.Opacity), widgets.NewTransform(actorMatrix(st, ly.origin, s.view, k), w))
}

// actorMatrix is the transform of an actor in stage pixels (k pixels per view
// box unit): scale and rotate around the track's origin, then move by x and y
// (CASSETTE-FORMAT §5.4), that is T(x,y) · T(origin) · R · S · T(-origin).
func actorMatrix(st timeline.ActorState, origin []float64, vb scene.ViewBox, k float64) *ui.Matrix4 {
	m := ui.NewMatrix4Identity()
	m.Translate(float32(st.X*k), float32(st.Y*k), 0)
	if len(origin) == 2 {
		ox, oy := (origin[0]-vb.X)*k, (origin[1]-vb.Y)*k
		m.Translate(float32(ox), float32(oy), 0)
		m.RotateZ(st.Rotate * math.Pi / 180)
		m.Scale(float32(st.Scale), float32(st.Scale), 1)
		m.Translate(float32(-ox), float32(-oy), 0)
	}
	return m
}

// revealRect is the part of the reveal box (view box units) that is shown when
// the fraction reveal of it is revealed, starting from the side from, in stage
// pixels. Reveal 0 gives an empty rectangle.
func revealRect(vb scene.ViewBox, box []float64, from timeline.RevealFrom, reveal, k float64) ui.Rect {
	r := clamp01(reveal)
	x, y, w, h := (box[0]-vb.X)*k, (box[1]-vb.Y)*k, box[2]*k, box[3]*k
	switch from.Or() {
	case timeline.RevealRight:
		return ui.NewRectFromLTWH(x+w*(1-r), y, w*r, h)
	case timeline.RevealTop:
		return ui.NewRectFromLTWH(x, y, w, h*r)
	case timeline.RevealBottom:
		return ui.NewRectFromLTWH(x, y+h*(1-r), w, h*r)
	}
	return ui.NewRectFromLTWH(x, y, w*r, h)
}

// revealClipper clips an actor to the revealed part of its box; a fully
// revealed actor is not clipped at all, so that strokes at the edge of the box
// are never cut.
type revealClipper struct {
	rendering.CustomClipperBase[ui.Rect]
	rect ui.Rect
	full bool
}

func newRevealClipper(vb scene.ViewBox, box []float64, from timeline.RevealFrom, reveal, k float64) *revealClipper {
	if reveal >= 1 {
		return &revealClipper{full: true}
	}
	return &revealClipper{rect: revealRect(vb, box, from, reveal, k)}
}

// GetClip returns the revealed rectangle, or the whole box.
func (c *revealClipper) GetClip(size ui.Size) ui.Rect {
	if c.full {
		return ui.OffsetZero.And(size)
	}
	return c.rect
}

// ShouldReclip is true when the revealed part changed.
func (c *revealClipper) ShouldReclip(old rendering.CustomClipper[ui.Rect]) bool {
	o, ok := old.(*revealClipper)
	return !ok || o.full != c.full || o.rect != c.rect
}

func clamp01(v float64) float64 { return math.Min(1, math.Max(0, v)) }

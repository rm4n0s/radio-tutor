package player

import (
	"context"
	"math"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/glutter/ui"
	"github.com/rm4n0s/radio-tutor/internal/core/audio"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/scene"
	"github.com/rm4n0s/radio-tutor/internal/core/timeline"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/student"
)

func offset(dx, dy float64) ui.Offset { return ui.Offset{Dx: dx, Dy: dy} }

func testFailure() error { return errors.New("TestPlayFailure", "the sound server refused") }

var viewBox = scene.ViewBox{Width: 1600, Height: 900}

func TestActorMatrix(t *testing.T) {
	tests := []struct {
		name   string
		state  timeline.ActorState
		origin []float64
		vb     scene.ViewBox
		k      float64
		// from and want are points in stage pixels.
		from, want ui.Offset
	}{
		{"no change", timeline.DefaultState(), nil, viewBox, 0.5, offset(300, 200), offset(300, 200)},
		{"move by view box units", timeline.ActorState{Opacity: 1, Scale: 1, Reveal: 1, X: 400, Y: -100}, nil, viewBox, 0.5, offset(300, 200), offset(500, 150)},
		{"rotate 90 degrees clockwise around the origin", timeline.ActorState{Opacity: 1, Scale: 1, Reveal: 1, Rotate: 90}, []float64{800, 450}, viewBox, 0.5, offset(500, 225), offset(400, 325)},
		{"scale around the origin", timeline.ActorState{Opacity: 1, Scale: 2, Reveal: 1}, []float64{800, 450}, viewBox, 0.5, offset(500, 225), offset(600, 225)},
		{
			// Scale and rotate around the origin first, then move: the point is
			// 200 units right of the origin; scaled 2 it is 400, turned 90
			// degrees it is 400 below the origin, moved by 100 it is at (900,850).
			"scale and rotate, then move", timeline.ActorState{Opacity: 1, Scale: 2, Rotate: 90, X: 100, Reveal: 1},
			[]float64{800, 450}, viewBox, 0.5, offset(500, 225), offset(450, 425),
		},
		{
			"a view box that does not start at 0", timeline.ActorState{Opacity: 1, Scale: 1, Rotate: 180, Reveal: 1},
			[]float64{100, 100}, scene.ViewBox{X: 0, Y: 0, Width: 200, Height: 200}, 2, offset(300, 200), offset(100, 200),
		},
		{
			"origin relative to the view box's corner", timeline.ActorState{Opacity: 1, Scale: 1, Rotate: 180, Reveal: 1},
			[]float64{-50, -50}, scene.ViewBox{X: -100, Y: -100, Width: 200, Height: 200}, 2, offset(300, 200), offset(-100, 0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := actorMatrix(tt.state, tt.origin, tt.vb, tt.k).TransformPoint(tt.from)
			if math.Abs(got.Dx-tt.want.Dx) > 1e-3 || math.Abs(got.Dy-tt.want.Dy) > 1e-3 {
				t.Errorf("%v maps to %v, want %v", tt.from, got, tt.want)
			}
		})
	}
}

func TestRevealRect(t *testing.T) {
	box := []float64{100, 200, 400, 100}
	tests := []struct {
		name   string
		from   timeline.RevealFrom
		reveal float64
		vb     scene.ViewBox
		k      float64
		want   ui.Rect
	}{
		{"left, a quarter", timeline.RevealLeft, 0.25, viewBox, 1, ui.NewRectFromLTWH(100, 200, 100, 100)},
		{"the default side is left", "", 0.5, viewBox, 1, ui.NewRectFromLTWH(100, 200, 200, 100)},
		{"right, a quarter", timeline.RevealRight, 0.25, viewBox, 1, ui.NewRectFromLTWH(400, 200, 100, 100)},
		{"top, a quarter", timeline.RevealTop, 0.25, viewBox, 1, ui.NewRectFromLTWH(100, 200, 400, 25)},
		{"bottom, a quarter", timeline.RevealBottom, 0.25, viewBox, 1, ui.NewRectFromLTWH(100, 275, 400, 25)},
		{"nothing revealed", timeline.RevealLeft, 0, viewBox, 1, ui.NewRectFromLTWH(100, 200, 0, 100)},
		{"all revealed", timeline.RevealBottom, 1, viewBox, 1, ui.NewRectFromLTWH(100, 200, 400, 100)},
		{"more than all is all", timeline.RevealLeft, 7, viewBox, 1, ui.NewRectFromLTWH(100, 200, 400, 100)},
		{"in pixels", timeline.RevealLeft, 0.5, viewBox, 0.5, ui.NewRectFromLTWH(50, 100, 100, 50)},
		{"view box offset", timeline.RevealLeft, 0.5, scene.ViewBox{X: 50, Y: 100, Width: 1600, Height: 900}, 1, ui.NewRectFromLTWH(50, 100, 200, 100)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := revealRect(tt.vb, box, tt.from, tt.reveal, tt.k); got != tt.want {
				t.Errorf("revealRect = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRevealClipper(t *testing.T) {
	size := ui.Size{Width: 800, Height: 450}
	box := []float64{100, 200, 400, 100}
	half := newRevealClipper(viewBox, box, timeline.RevealLeft, 0.5, 0.5)
	if got, want := half.GetClip(size), ui.NewRectFromLTWH(50, 100, 100, 50); got != want {
		t.Errorf("half clip = %v, want %v", got, want)
	}
	full := newRevealClipper(viewBox, box, timeline.RevealLeft, 1, 0.5)
	if got := full.GetClip(size); got != ui.NewRectFromLTWH(0, 0, 800, 450) {
		t.Errorf("a fully revealed actor is clipped to %v, want the whole box", got)
	}
	tests := []struct {
		name string
		a, b *revealClipper
		want bool
	}{
		{"same", half, newRevealClipper(viewBox, box, timeline.RevealLeft, 0.5, 0.5), false},
		{"more revealed", half, newRevealClipper(viewBox, box, timeline.RevealLeft, 0.6, 0.5), true},
		{"fully revealed", half, full, true},
		{"fully revealed twice", full, newRevealClipper(viewBox, box, timeline.RevealTop, 1, 0.5), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.ShouldReclip(tt.b); got != tt.want {
				t.Errorf("ShouldReclip = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStageFit(t *testing.T) {
	tests := []struct {
		name          string
		view          scene.ViewBox
		width, height float64
		want          ui.Size
	}{
		{"wider space: letterbox at the sides", viewBox, 1000, 450, ui.Size{Width: 800, Height: 450}},
		{"taller space: letterbox above and below", viewBox, 800, 800, ui.Size{Width: 800, Height: 450}},
		{"exact", viewBox, 1600, 900, ui.Size{Width: 1600, Height: 900}},
		{"4:3", scene.ViewBox{Width: 400, Height: 300}, 800, 300, ui.Size{Width: 400, Height: 300}},
		{"no space", viewBox, 0, 0, ui.Size{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &stage{view: tt.view}
			if got := s.fit(tt.width, tt.height); got != tt.want {
				t.Errorf("fit = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatTime(t *testing.T) {
	tests := []struct {
		seconds float64
		want    string
	}{
		{0, "0:00"}, {-3, "0:00"}, {0.99, "0:00"}, {5, "0:05"}, {59.9, "0:59"}, {60, "1:00"}, {125, "2:05"}, {600, "10:00"},
	}
	for _, tt := range tests {
		if got := formatTime(tt.seconds); got != tt.want {
			t.Errorf("formatTime(%v) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}

// testLessonFS is a cassette folder with the test lesson and a narration.
func testLessonFS(t *testing.T) fstest.MapFS {
	t.Helper()
	read := func(path string) []byte {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	return fstest.MapFS{
		"lessons/l1/scene.svg":     {Data: read("testdata/lesson/scene.svg")},
		"lessons/l1/narration.ogg": {Data: read(validCassette + "/lessons/what-is-a-fraction/narration.ogg")},
	}
}

func fakeAudio() (app.SinkFactory, audio.Clock) {
	clock := audio.NewFakeClock(time.Unix(0, 0))
	return func(context.Context) (audio.Sink, error) { return audio.NewFakeSink(clock), nil }, clock
}

func TestLoadLesson(t *testing.T) {
	c := &cassette.Cassette{Lessons: map[string]*cassette.Lesson{"l1": {ID: "l1"}}}
	tests := []struct {
		name   string
		change func(fs fstest.MapFS)
		lesson string
		route  string // empty: loads
	}{
		{"loads", func(fstest.MapFS) {}, "l1", ""},
		{"unknown lesson", func(fstest.MapFS) {}, "nope", "loadLesson.PlayerLessonUnknown"},
		{"no scene", func(fs fstest.MapFS) { delete(fs, "lessons/l1/scene.svg") }, "l1", "loadLesson.PlayerSceneUnreadable"},
		{"scene with a script", func(fs fstest.MapFS) {
			fs["lessons/l1/scene.svg"] = &fstest.MapFile{Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1600 900"><script>1</script></svg>`)}
		}, "l1", "loadLesson.PlayerSceneInvalid"},
		{"scene that is not SVG", func(fs fstest.MapFS) {
			fs["lessons/l1/scene.svg"] = &fstest.MapFile{Data: []byte("nope")}
		}, "l1", "loadLesson.PlayerSceneInvalid"},
		{"no narration", func(fs fstest.MapFS) { delete(fs, "lessons/l1/narration.ogg") }, "l1", "loadLesson.PlayerNarrationUnreadable"},
		{"narration that is not Ogg", func(fs fstest.MapFS) {
			fs["lessons/l1/narration.ogg"] = &fstest.MapFile{Data: []byte("not ogg at all")}
		}, "l1", "loadLesson->DecodeLimit->stream.AudioHeaderInvalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := testLessonFS(t)
			tt.change(fsys)
			open, clock := fakeAudio()
			ld, err := loadLesson(context.Background(), student.LessonRequest{Cassette: c, FS: fsys, LessonID: tt.lesson}, open, clock)
			if tt.route == "" {
				if err != nil {
					t.Fatalf("loadLesson: %v", err)
				}
				defer ld.close()
				if ld.player.Duration() != 3 || len(ld.scene.Layers) == 0 {
					t.Errorf("duration %v, %d layers", ld.player.Duration(), len(ld.scene.Layers))
				}
				return
			}
			appErr, ok := errors.FromError(err)
			if !ok {
				t.Fatalf("expected *errors.Error, got %T: %v", err, err)
			}
			if ld != nil {
				t.Error("a loaded lesson came with an error")
			}
			if !appErr.HasRoute(tt.route) {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
		})
	}
}

func TestLoadLessonCancelled(t *testing.T) {
	c := &cassette.Cassette{Lessons: map[string]*cassette.Lesson{"l1": {ID: "l1"}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	open, clock := fakeAudio()
	_, err := loadLesson(ctx, student.LessonRequest{Cassette: c, FS: testLessonFS(t), LessonID: "l1"}, open, clock)
	if err == nil {
		t.Fatal("a cancelled load succeeded")
	}
}

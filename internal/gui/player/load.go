package player

import (
	"context"
	"io/fs"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/audio"
	"github.com/rm4n0s/radio-tutor/internal/core/problem"
	"github.com/rm4n0s/radio-tutor/internal/core/scene"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/student"
)

// The files of a lesson inside the cassette (CASSETTE-FORMAT §5).
const (
	lessonsDir    = "lessons/"
	sceneFile     = "scene.svg"
	narrationFile = "narration.ogg"
)

// maxNarrationSeconds is the longest narration the player decodes: the 10
// minutes of CASSETTE-FORMAT §6 and a second of rounding.
const maxNarrationSeconds = 601

// loaded is a lesson ready to play: the scene, and the player with the
// narration in it. The sink and the player belong to whoever called
// loadLesson, who calls close.
type loaded struct {
	scene  *scene.Scene
	player *audio.Player
	sink   audio.Sink
	// noSound is true when the sink is the silent one: no sound device was
	// found.
	noSound bool
}

// close stops the playback and closes the player and its sink.
func (l *loaded) close() {
	l.player.Close()
	l.sink.Close()
}

// loadLesson reads the scene, decodes the narration into memory and opens the
// sound output (SPEC §8.1) with the sink factory; the player's end timer runs
// on clock. It runs on a goroutine. It fails with the tags
// PlayerLessonUnknown, PlayerSceneUnreadable, PlayerSceneInvalid and
// PlayerNarrationUnreadable, with the tags of audio.DecodeLimit when the
// narration cannot be decoded, and with the tags of the player's Load.
func loadLesson(ctx context.Context, req student.LessonRequest, open app.SinkFactory, clock audio.Clock) (*loaded, error) {
	if req.Cassette == nil || req.Cassette.Lessons[req.LessonID] == nil {
		return nil, errors.New("PlayerLessonUnknown", "the cassette has no such lesson", "lesson_id", req.LessonID)
	}
	dir := lessonsDir + req.LessonID + "/"
	data, err := fs.ReadFile(req.FS, dir+sceneFile)
	if err != nil {
		return nil, errors.NewErr("PlayerSceneUnreadable", err).SetMetadata("lesson_id", req.LessonID)
	}
	sc, problems, err := scene.Split(data)
	if err != nil {
		return nil, err
	}
	if sc == nil || problem.HasErrors(problems) {
		return nil, errors.New("PlayerSceneInvalid", "the scene of the lesson has errors", "lesson_id", req.LessonID, "problems", problems)
	}
	f, err := req.FS.Open(dir + narrationFile)
	if err != nil {
		return nil, errors.NewErr("PlayerNarrationUnreadable", err).SetMetadata("lesson_id", req.LessonID)
	}
	clip, err := audio.DecodeLimit(ctx, f, maxNarrationSeconds)
	_ = f.Close() // a read-only file: nothing is lost if closing fails
	if err != nil {
		return nil, err
	}
	sink, err := open(ctx)
	if err != nil {
		return nil, err
	}
	pl := audio.NewPlayer(sink, clock)
	if err := pl.Load(clip); err != nil {
		sink.Close()
		return nil, err
	}
	return &loaded{scene: sc, player: pl, sink: sink, noSound: sink.Name() == app.SilentSinkName}, nil
}

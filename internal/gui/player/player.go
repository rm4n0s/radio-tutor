// Package player is the lesson player (SPEC §8): the screen that plays the
// narration of a lesson, animates its scene by the timeline and shows the
// captions, with Play, Pause, Stop, a progress bar and the time, and that
// saves the lesson as completed when the narration plays to its end.
//
// # Threads
//
// Widgets are touched on the UI thread only. The audio is not: decoding the
// narration, opening the sound device, starting and stopping the stream and
// saving the progress all do I/O, so one goroutine per screen (State.Go) owns
// the audio player. The UI thread sends it commands (play, pause, stop) on a
// channel and it answers with SetState; it also listens to the player's Ended
// channel. The frame ticker, which only runs while the lesson plays, reads the
// player's position once per frame on the UI thread (a cheap, non-blocking
// read) and evaluates the timeline at that time.
//
// Leaving the screen cancels the goroutine's context, which makes it stop and
// close the player and the sink.
package player

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/glutter/scheduler"
	"github.com/rm4n0s/glutter/services"
	"github.com/rm4n0s/glutter/widgets"
	"github.com/rm4n0s/radio-tutor/internal/core/audio"
	"github.com/rm4n0s/radio-tutor/internal/core/timeline"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/student"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
)

// Keys of the widgets that tests look for.
const (
	// KeyPlay is the key of the Play/Pause toggle.
	KeyPlay = "player-play"
	// KeyStop is the key of the Stop button.
	KeyStop = "player-stop"
	// KeyBack is the key of the back button of the app bar.
	KeyBack = "player-back"
	// KeyNext is the key of the Next (or Done) button that appears when the
	// lesson has played to its end.
	KeyNext = "player-next"
	// KeySkip is the key of the Skip button (shown only with skip lessons on).
	KeySkip = "player-skip"
	// KeyProgress is the key of the progress bar.
	KeyProgress = "player-progress"
	// KeyTime is the key of the "m:ss / m:ss" text.
	KeyTime = "player-time"
	// KeyCaption is the key of the caption text.
	KeyCaption = "player-caption"
	// KeyLoading is the key of the loading indicator.
	KeyLoading = "player-loading"
	// KeyNotice is the key of the notice (no sound device, play failed).
	KeyNotice = "player-notice"
	// KeyNoticeOK is the key of the notice's OK button.
	KeyNoticeOK = "player-notice-ok"
	// KeyStage is the key of the stage (the scene's box).
	KeyStage = "player-stage"
	// KeyCaptionBar is the key of the caption bar, which has a fixed height.
	KeyCaptionBar = "player-caption-bar"
)

// Sizes of the player, in logical pixels.
const (
	captionFontSize = 24.0
	captionLineGap  = 1.3
	captionLines    = 2
	captionPadding  = 8.0
	stagePadding    = 8.0
	// completeTimeout bounds the saving of the completion, which is not
	// cancelled when the child leaves the screen at the very end.
	completeTimeout = 10 * time.Second
)

// Builder returns the function that goes into student.Screens.Lesson.
func Builder() func(student.LessonRequest) m.Widget {
	return func(req student.LessonRequest) m.Widget { return NewLesson(req) }
}

// Lesson is the lesson player screen.
type Lesson struct {
	m.StatefulWidget
	req student.LessonRequest
}

// NewLesson returns the player for the lesson of the request.
func NewLesson(req student.LessonRequest, opts ...m.WidgetOpts) *Lesson {
	return &Lesson{StatefulWidget: m.NewStatefulWidgetBase(opts...), req: req}
}

// CreateState creates the state.
func (w *Lesson) CreateState() m.State { return &lessonState{cmds: make(chan command, 8)} }

// command is what the UI asks of the goroutine that owns the audio player.
type command int

const (
	cmdPlay command = iota
	cmdPause
	cmdStop
)

type lessonState struct {
	m.StateOf[*Lesson]
	widgets.SingleTickerProvider

	started bool
	cmds    chan command
	ticker  *scheduler.Ticker

	// The fields below are touched on the UI thread only.
	loaded    bool
	failed    bool
	stage     *stage
	eval      *timeline.Evaluator
	states    []timeline.ActorState
	captions  []timeline.Caption
	player    *audio.Player // only Position and State are read from the UI thread
	duration  float64
	pos       float64
	playing   bool
	completed bool // the lesson is saved as completed: Next is shown
	skip      bool // the student's skip_lessons setting
	notice    string
	leaving   bool
}

// InitState creates the frame ticker.
func (s *lessonState) InitState() {
	s.ticker = s.CreateTicker(s.onTick)
}

// Dispose stops the ticker; the goroutine closes the player and the sink when
// its context (the state's lifetime) is cancelled.
func (s *lessonState) Dispose() {
	s.ticker.Dispose()
}

// DidChangeDependencies starts the loading (the Scope cannot be read in
// InitState).
func (s *lessonState) DidChangeDependencies() {
	if s.started {
		return
	}
	s.started = true
	env, req := app.ScopeOf(s.Context()).Env(), s.Widget().req
	s.Go(func(ctx context.Context) { s.run(ctx, env, req) })
}

// run is the goroutine of the screen: it loads the lesson, then serves the
// commands and the end of the narration until the screen is left.
func (s *lessonState) run(ctx context.Context, env *app.Env, req student.LessonRequest) {
	ld, err := loadLesson(ctx, req, env.SinkFactory, env.AudioClock)
	if err != nil {
		if ctx.Err() == nil {
			env.Log.Error("opening a lesson failed", "cassette", req.Cassette.Manifest.ID, "lesson", req.LessonID, "err", err)
			s.SetState(func() { s.failed = true })
		}
		return
	}
	defer ld.close()
	if ctx.Err() != nil {
		return
	}
	skip := s.readSkip(ctx, env, req)
	tracks, captions := req.Cassette.Lessons[req.LessonID].Tracks, req.Cassette.Lessons[req.LessonID].Captions
	showNotice := ld.noSound && env.ClaimNoSoundNotice()
	s.SetState(func() {
		s.stage = newStage(ld.scene, tracks)
		s.eval = timeline.NewEvaluator(tracks)
		s.captions = captions
		s.player = ld.player
		s.duration = ld.player.Duration()
		s.skip = skip
		if showNotice {
			s.notice = "player.no_sound"
		}
		s.loaded = true
	})
	ended := ld.player.Ended()
	for {
		select {
		case c := <-s.cmds:
			s.exec(ctx, env, req, ld.player, c)
		case <-ended:
			s.finish(ctx, env, req, ld.player)
		case <-ctx.Done():
			return
		}
	}
}

// readSkip reads the student's skip_lessons setting.
func (s *lessonState) readSkip(ctx context.Context, env *app.Env, req student.LessonRequest) bool {
	p, err := env.Progress.Load(ctx, req.Username, req.Cassette.Manifest.ID)
	if err != nil && !isBrokenFile(err) {
		env.Log.Error("reading the progress failed", "username", req.Username, "err", err)
	}
	return p != nil && p.Settings.SkipLessons
}

// exec runs one command on the audio player and tells the UI what came of it.
func (s *lessonState) exec(ctx context.Context, env *app.Env, req student.LessonRequest, pl *audio.Player, c command) {
	failed := false
	switch c {
	case cmdPlay:
		fresh := pl.State() != audio.StatePaused
		pl.SetVolume(env.Settings.Get().Volume) // the parents' setting as it is now; a paused clip keeps its volume
		if err := pl.Play(ctx); err != nil {
			if ctx.Err() == nil {
				env.Log.Error("playing a lesson failed", "lesson", req.LessonID, "err", err)
			}
			failed = true
		} else if fresh {
			s.update(ctx, env, req, "counting a play", func(p *progress.Progress) error {
				return progress.PlayLesson(req.Cassette, p, req.LessonID)
			})
		}
	case cmdPause:
		pl.Pause()
	case cmdStop:
		pl.Stop()
	}
	state, pos := pl.State(), pl.Position()
	s.SetState(func() {
		s.playing, s.pos = state == audio.StatePlaying, pos
		if failed {
			s.notice = "player.play_failed"
		}
		s.syncTicker()
	})
}

// finish handles the end of the narration: the stage holds the last frame, and
// the lesson is saved as completed, which makes Next appear.
func (s *lessonState) finish(ctx context.Context, env *app.Env, req student.LessonRequest, pl *audio.Player) {
	end := pl.Duration()
	s.SetState(func() {
		s.playing, s.pos = false, end
		s.syncTicker()
	})
	// The save is not cancelled when the child leaves at once: the lesson was
	// played to its end.
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), completeTimeout)
	defer cancel()
	s.update(saveCtx, env, req, "completing", func(p *progress.Progress) error {
		return progress.CompleteLesson(req.Cassette, p, req.LessonID, time.Now().UTC())
	})
	s.SetState(func() { s.completed = true })
}

// update changes the student's save file; a failure is logged, not shown (the
// child can go on).
func (s *lessonState) update(ctx context.Context, env *app.Env, req student.LessonRequest, what string, fn func(p *progress.Progress) error) {
	err := env.Progress.Update(ctx, req.Username, req.Cassette.Manifest.ID, fn)
	if err != nil && ctx.Err() == nil {
		level := slog.LevelError
		if isBrokenFile(err) {
			level = slog.LevelWarn
		}
		env.Log.Log(ctx, level, "saving the lesson progress failed", "what", what, "username", req.Username, "lesson", req.LessonID, "err", err)
	}
}

// isBrokenFile reports whether err is the progress store's note that a save
// file was set aside; the store has saved a fresh one with the change then.
func isBrokenFile(err error) bool {
	appErr, ok := errors.FromError(err)
	return ok && appErr.Tag == "ProgressFileBroken"
}

// syncTicker runs the frame ticker while the lesson plays and stops it
// otherwise, so that a paused or finished lesson costs no CPU. UI thread.
func (s *lessonState) syncTicker() {
	switch {
	case s.playing && !s.ticker.IsActive():
		s.ticker.Start()
	case !s.playing && s.ticker.IsActive():
		s.ticker.Stop()
	}
}

// onTick is the frame callback: it reads the player's position once.
func (s *lessonState) onTick(time.Duration) {
	pl := s.player
	if pl == nil {
		return
	}
	pos, st := pl.Position(), pl.State()
	s.SetState(func() { s.pos = pos })
	if st != audio.StatePlaying {
		// Paused, stopped or ended: the goroutine reports the change and
		// stops the ticker; do not tick until then.
		s.ticker.Stop()
	}
}

// send hands a command to the audio goroutine; a full queue drops it (the child
// is tapping faster than the audio answers).
func (s *lessonState) send(c command) {
	select {
	case s.cmds <- c:
	default:
	}
}

// toggle is Play when the lesson is not playing and Pause when it is.
func (s *lessonState) toggle() {
	if !s.loaded {
		return
	}
	if s.playing {
		s.send(cmdPause)
		return
	}
	s.send(cmdPlay)
}

// back leaves the player. Leaving stops the audio: the goroutine closes the
// player, which stops the stream, and the sink.
func (s *lessonState) back() {
	if s.leaving {
		return
	}
	s.leaving = true
	app.Pop(s.Context())
}

// forward opens the next item (Next, Skip) or, after the last lesson, goes
// back.
func (s *lessonState) forward() {
	if s.leaving {
		return
	}
	next := s.Widget().req.OpenNext
	if next == nil {
		s.back()
		return
	}
	s.leaving = true
	next(s.Context())
}

// formatTime is seconds as m:ss.
func formatTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	n := int(seconds)
	sec := strconv.Itoa(n % 60)
	if len(sec) < 2 {
		sec = "0" + sec
	}
	return strconv.Itoa(n/60) + ":" + sec
}

// shortcuts are the keys of the player: Space toggles, Escape goes back.
func (s *lessonState) shortcuts(child m.Widget) m.Widget {
	return widgets.NewCallbackShortcuts([]widgets.CallbackShortcutEntry{
		{Activator: widgets.NewSingleActivator(services.LogicalKeyboardKeySpace), Callback: s.toggle},
		{Activator: widgets.NewSingleActivator(services.LogicalKeyboardKeyEscape), Callback: s.back},
	}, widgets.NewFocus(child, widgets.FocusOpts{Autofocus: true}))
}

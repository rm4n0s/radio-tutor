package player

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/glutter/services"
	"github.com/rm4n0s/radio-tutor/internal/gui/student"
)

// playAt starts the lesson and moves the audio clock to t seconds.
func (h *harness) playAt(t time.Duration) {
	h.t.Helper()
	h.tap(KeyPlay)
	h.waitPlaying(true)
	h.advance(t)
}

func TestStageAtFixedTimes(t *testing.T) {
	tests := []struct {
		name string
		// at is the time of the lesson; before is true for the picture before
		// the lesson is played.
		at    time.Duration
		time  string
		quote string
	}{
		{"before_play", 0, "0:00 / 0:03", "Hello! This is a test lesson."},
		// The square is at x=400 (its box 500..700), the bar has turned 45
		// degrees, the dot is scaled 1.75 and the green circle is half opaque;
		// the label is not revealed yet (it starts at 1 s).
		{"mid_move", time.Second, "0:01 / 0:03", "A square moves right.\nA bar turns around its center."},
		// Half of the label is revealed (HELLO is cut at x=800), the square is at
		// 600 (700..900), the bar at 67.5 degrees.
		{"half_reveal", 1500 * time.Millisecond, "0:01 / 0:03", "A square moves right.\nA bar turns around its center."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := pump(t, config{})
			h.loaded()
			if tt.at > 0 {
				h.playAt(tt.at)
			}
			if got := h.textOf(KeyTime); got != tt.time {
				t.Errorf("time = %q, want %q", got, tt.time)
			}
			if got := h.textOf(KeyCaption); got != tt.quote {
				t.Errorf("caption = %q, want %q", got, tt.quote)
			}
			h.MatchesGolden("stage_" + tt.name)
		})
	}
}

func TestStageAfterTheEnd(t *testing.T) {
	h := pump(t, config{})
	h.loaded()
	h.playAt(3 * time.Second)
	h.until("Next", func() bool { return h.HasKey(KeyNext) })
	if got := h.textOf(KeyTime); got != "0:03 / 0:03" {
		t.Errorf("time = %q", got)
	}
	// The last caption ended at 2.9 s: the bar stays and is empty.
	if got := h.textOf(KeyCaption); got != "" {
		t.Errorf("caption at the end = %q, want none", got)
	}
	h.waitPlaying(false)
	h.MatchesGolden("stage_after_end")
}

func TestGreekControls(t *testing.T) {
	h := pump(t, config{locale: "el_GR.UTF-8"})
	h.loaded()
	if !h.hasText("Αναπαραγωγή") || !h.hasText("Διακοπή") {
		t.Error("the controls are not in Greek")
	}
	h.MatchesGolden("controls_el")
}

func TestLoadingIndicatorWhileDecoding(t *testing.T) {
	gate := make(chan struct{})
	h := pump(t, config{gate: gate, stayOnCassette: true})
	h.tap(student.KeyItemPrefix + lesson1)
	// The narration is decoded on a goroutine that has not handed over yet: the
	// screen shows the loading indicator and its message, and no controls.
	h.Pump(300 * time.Millisecond)
	if !h.HasKey(KeyLoading) || !h.hasText("Getting the lesson ready...") {
		t.Fatal("no loading indicator while loading")
	}
	if h.HasKey(KeyPlay) {
		t.Error("the controls are shown while loading")
	}
	close(gate)
	h.loaded()
	if h.HasKey(KeyLoading) {
		t.Error("the loading indicator stays after loading")
	}
}

func TestPlayPauseStop(t *testing.T) {
	h := pump(t, config{})
	h.loaded()
	if h.HasKey(KeyNext) || h.HasKey(KeySkip) {
		t.Fatal("Next or Skip shown before the end")
	}
	h.tap(KeyPlay)
	h.waitPlaying(true)
	h.advance(1200 * time.Millisecond)
	if got := h.textOf(KeyTime); got != "0:01 / 0:03" {
		t.Errorf("playing: time = %q", got)
	}

	// Pause holds the position.
	h.tap(KeyPlay)
	h.waitPlaying(false)
	h.advance(time.Second)
	if got := h.textOf(KeyTime); got != "0:01 / 0:03" {
		t.Errorf("paused: time = %q, want it held", got)
	}
	if n := h.Binding.Scheduler().TransientCallbackCount(); n != 0 {
		t.Errorf("%d frame callbacks while paused; the ticker must stop", n)
	}

	// Play resumes where it was.
	h.tap(KeyPlay)
	h.waitPlaying(true)
	h.advance(time.Second)
	if got := h.textOf(KeyTime); got != "0:02 / 0:03" {
		t.Errorf("resumed: time = %q", got)
	}

	// Stop goes back to 0 and the Play button comes back.
	h.tap(KeyStop)
	h.waitPlaying(false)
	h.until("the position 0", func() bool { return h.textOf(KeyTime) == "0:00 / 0:03" })
	if got := h.textOf(KeyCaption); got != "Hello! This is a test lesson." {
		t.Errorf("after Stop the caption is %q", got)
	}
	want := []string{"start", "pause", "resume", "stop"}
	if got := h.sink(0).Calls(); len(got) != len(want) {
		t.Errorf("sink calls = %v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("sink calls = %v, want %v", got, want)
				break
			}
		}
	}
	// One play from the start; resuming did not count.
	waitPlays(t, h, 1)
}

// waitPlays waits until the save file counts the plays of lesson 1.
func waitPlays(t *testing.T, h *harness, plays int) {
	t.Helper()
	h.until("the plays to be saved", func() bool { return h.savedProgress().Lessons[lesson1].Plays >= plays })
	if got := h.savedProgress().Lessons[lesson1].Plays; got != plays {
		t.Errorf("plays = %d, want %d", got, plays)
	}
}

func TestSpaceTogglesAndEscapeGoesBack(t *testing.T) {
	h := pump(t, config{})
	h.loaded()
	h.Pump()

	h.SendKeyEvent(services.LogicalKeyboardKeySpace)
	h.waitPlaying(true)
	h.advance(time.Second)
	h.SendKeyEvent(services.LogicalKeyboardKeySpace)
	h.waitPlaying(false)

	// A button that has the focus must not make Space act twice.
	h.tap(KeyStop)
	h.SendKeyEvent(services.LogicalKeyboardKeySpace)
	h.waitPlaying(true)
	h.SendKeyEvent(services.LogicalKeyboardKeySpace)
	h.waitPlaying(false)

	h.SendKeyEvent(services.LogicalKeyboardKeyEscape)
	h.until("the cassette screen", func() bool { return h.HasKey(student.KeyContinue) })
	h.waitClosed(0)
}

// waitClosed waits until the n-th sink was closed.
func (h *harness) waitClosed(n int) {
	h.t.Helper()
	sink := h.sink(n)
	h.until("the sink to be closed", func() bool {
		for _, c := range sink.Calls() {
			if c == "close" {
				return true
			}
		}
		return false
	})
}

func TestBackButtonStopsAndClosesThePlayer(t *testing.T) {
	h := pump(t, config{})
	h.loaded()
	h.playAt(time.Second)
	h.tap(KeyBack)
	h.until("the cassette screen", func() bool { return h.HasKey(student.KeyContinue) })
	h.waitClosed(0)
	calls := h.sink(0).Calls()
	if calls[0] != "start" || calls[len(calls)-1] != "close" {
		t.Errorf("sink calls = %v", calls)
	}
	sawStop := false
	for _, c := range calls {
		sawStop = sawStop || c == "stop"
	}
	if !sawStop {
		t.Errorf("the stream was not stopped: %v", calls)
	}
	// Not completed: only a play was counted.
	if h.savedProgress().Lessons[lesson1].Completed() {
		t.Error("the lesson was completed by leaving it")
	}
}

func TestLeavingWhileLoadingClosesTheSink(t *testing.T) {
	h := pump(t, config{})
	h.tap(KeyBack)
	h.until("the cassette screen", func() bool { return h.HasKey(student.KeyContinue) })
	h.until("the loading goroutine to end", func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		// The sink is opened only when the narration was decoded; if it was, it
		// must be closed.
		if len(h.sinks) == 0 {
			return true
		}
		for _, c := range h.sinks[0].Calls() {
			if c == "close" {
				return true
			}
		}
		return false
	})
}

func TestCompletionIsSavedAtTheEnd(t *testing.T) {
	h := pump(t, config{})
	h.loaded()
	h.playAt(2 * time.Second)
	if h.savedProgress().Lessons[lesson1].Completed() {
		t.Fatal("completed before the end")
	}
	h.advance(time.Second)
	h.until("Next", func() bool { return h.HasKey(KeyNext) })
	lp := h.savedProgress().Lessons[lesson1]
	if !lp.Completed() || lp.Plays != 1 {
		t.Errorf("lesson progress = %+v, want completed after 1 play", lp)
	}
	if !h.hasText("Next") {
		t.Error("the Next button has no label")
	}
	if h.Binding.Scheduler().TransientCallbackCount() != 0 {
		t.Error("the ticker still runs after the end")
	}

	// Playing again starts from 0 and counts another play.
	h.tap(KeyPlay)
	h.waitPlaying(true)
	h.advance(500 * time.Millisecond)
	if got := h.textOf(KeyTime); got != "0:00 / 0:03" {
		t.Errorf("replay: time = %q", got)
	}
	waitPlays(t, h, 2)
	if !h.savedProgress().Lessons[lesson1].Completed() {
		t.Error("playing again lost the completion")
	}
}

func TestSkipOnlyWithSkipLessons(t *testing.T) {
	tests := []struct {
		name string
		skip bool
	}{
		{"skip lessons off", false},
		{"skip lessons on", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := pump(t, config{skip: tt.skip})
			h.loaded()
			if got := h.HasKey(KeySkip); got != tt.skip {
				t.Fatalf("Skip shown = %v, want %v", got, tt.skip)
			}
			if !tt.skip {
				return
			}
			h.tap(KeySkip)
			h.until("the worksheet", func() bool { return h.hasText("worksheet opened") })
			if h.savedProgress().Lessons[lesson1].Completed() {
				t.Error("skipping completed the lesson")
			}
			h.waitClosed(0)
		})
	}
}

func TestNextOpensTheNextItem(t *testing.T) {
	tests := []struct {
		name         string
		lessonsFirst bool
		want         string
	}{
		{"a worksheet", false, "worksheet opened"},
		{"a lesson", true, "Adding fractions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := pump(t, config{lessonsFirst: tt.lessonsFirst})
			h.loaded()
			h.playAt(3 * time.Second)
			h.until("Next", func() bool { return h.HasKey(KeyNext) })
			h.tap(KeyNext)
			h.until(tt.want, func() bool { return h.hasText(tt.want) })
			if tt.lessonsFirst {
				// The second lesson is a player of its own, on its own sink.
				h.until("the second sink", func() bool {
					h.mu.Lock()
					defer h.mu.Unlock()
					return len(h.sinks) == 2
				})
				h.loaded()
			}
			h.waitClosed(0)
		})
	}
}

func TestDoneAfterTheLastItemGoesBack(t *testing.T) {
	// Lesson 1 is the last item of the cassette: there is no next item, so the
	// button says Done and goes back.
	h := pump(t, config{lastItem: true})
	h.loaded()
	h.playAt(3 * time.Second)
	h.until("Done", func() bool { return h.HasKey(KeyNext) && h.hasText("Done") })
	h.tap(KeyNext)
	h.until("the cassette screen", func() bool { return h.HasKey(student.KeyContinue) })
}

func TestNoSoundNoticeIsShownOncePerRun(t *testing.T) {
	const notice = "No sound device found. The lesson plays without sound."
	h := pump(t, config{noSound: true})
	h.loaded()
	if !h.hasText(notice) {
		t.Fatal("no notice about the missing sound device")
	}
	h.MatchesGolden("notice_no_sound")
	// The lesson can still be completed.
	h.playAt(3 * time.Second)
	h.until("Next", func() bool { return h.HasKey(KeyNext) })
	if !h.savedProgress().Lessons[lesson1].Completed() {
		t.Error("the lesson was not completed without sound")
	}
	h.tap(KeyBack)
	h.until("the cassette screen", func() bool { return h.HasKey(student.KeyContinue) })
	h.PumpAndSettle()

	// The second time in the same run: no notice.
	h.tap(student.KeyItemPrefix + lesson1)
	h.loaded()
	if h.hasText(notice) {
		t.Error("the notice was shown a second time")
	}
}

func TestNoticeCanBeDismissed(t *testing.T) {
	h := pump(t, config{noSound: true})
	h.loaded()
	h.tap(KeyNoticeOK)
	if h.HasKey(KeyNotice) {
		t.Error("the notice stays after OK")
	}
}

func TestNoNoticeWithASoundDevice(t *testing.T) {
	h := pump(t, config{})
	h.loaded()
	if h.HasKey(KeyNotice) {
		t.Error("a notice without a reason")
	}
}

func TestCaptionBarKeepsItsHeight(t *testing.T) {
	h := pump(t, config{})
	h.loaded()
	want := h.Size(h.finder(KeyCaptionBar))
	if want.Height != 24*1.3*2+16 {
		t.Errorf("the caption bar is %v high", want.Height)
	}
	h.tap(KeyPlay)
	h.waitPlaying(true)
	for _, at := range []time.Duration{500 * time.Millisecond, time.Second, 500 * time.Millisecond, 1500 * time.Millisecond} {
		h.advance(at)
		if got := h.Size(h.finder(KeyCaptionBar)); got != want {
			t.Errorf("caption bar %v at %v, want %v (one line, two lines and none)", got, h.textOf(KeyTime), want)
		}
	}
}

func TestProgressBarFollowsThePosition(t *testing.T) {
	h := pump(t, config{})
	h.loaded()
	h.playAt(1500 * time.Millisecond)
	if got := h.Size(h.finder(KeyProgress)); got.Width <= 0 {
		t.Fatalf("progress bar size %v", got)
	}
	bar, ok := h.Widget(h.finder(KeyProgress)).(*m.LinearProgressIndicator)
	if !ok {
		t.Fatal("the progress bar is not a LinearProgressIndicator")
	}
	if v := bar.Value().OrElse(-1); v < 0.49 || v > 0.51 {
		t.Errorf("progress = %v, want 0.5", v)
	}
}

func TestTheProgressBarCannotBeDragged(t *testing.T) {
	h := pump(t, config{})
	h.loaded()
	h.playAt(time.Second)
	before := h.textOf(KeyTime)
	h.Drag(h.finder(KeyProgress), offset(200, 0))
	h.Pump()
	if after := h.textOf(KeyTime); after != before {
		t.Errorf("dragging the progress bar moved the lesson: %s to %s", before, after)
	}
}

func TestLessonThatCannotBeOpened(t *testing.T) {
	tests := []struct {
		name   string
		break_ func(t *testing.T, dir string)
	}{
		{"narration is not Ogg", func(t *testing.T, dir string) {
			writeFile(t, filepath.Join(dir, "narration.ogg"), "not ogg")
		}},
		{"no narration", func(t *testing.T, dir string) {
			if err := os.Remove(filepath.Join(dir, "narration.ogg")); err != nil {
				t.Fatal(err)
			}
		}},
		{"scene has a script", func(t *testing.T, dir string) {
			writeFile(t, filepath.Join(dir, "scene.svg"), `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1600 900"><script>1</script></svg>`)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := pump(t, config{stayOnCassette: true})
			tt.break_(t, filepath.Join(h.Env.Paths.CassettesDir(), cassetteID, "lessons", lesson1))
			h.TapKey(student.KeyItemPrefix + lesson1)
			h.failedToLoad()
			if h.HasKey(KeyPlay) {
				t.Error("controls shown for a lesson that did not open")
			}
			h.tap(KeyBack)
			h.until("the cassette screen", func() bool { return h.HasKey(student.KeyContinue) })
			h.mu.Lock()
			defer h.mu.Unlock()
			for i, s := range h.sinks {
				closed := false
				for _, c := range s.Calls() {
					closed = closed || c == "close"
				}
				if !closed {
					t.Errorf("sink %d was left open", i)
				}
			}
		})
	}
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestPlayFailureIsShown(t *testing.T) {
	h := pump(t, config{})
	h.loaded()
	h.sink(0).FailStartWith(testFailure())
	h.tap(KeyPlay)
	h.until("the notice", func() bool { return h.hasText("The lesson could not start. Try again.") })
	if h.hasText("Pause") {
		t.Error("the button shows Pause after a failed start")
	}
}

func TestSavingFailureDoesNotBlockNext(t *testing.T) {
	h := pump(t, config{})
	h.loaded()
	// A file where the folder of the save files goes makes the save fail; the
	// failure is logged and the child goes on.
	dir := filepath.Join(h.Env.Paths.ProgressDir(), "mia")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "x")
	h.playAt(3 * time.Second)
	h.until("Next", func() bool { return h.HasKey(KeyNext) })
}

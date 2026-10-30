package player

import (
	"os"
	"testing"
	"time"

	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/glutter/ui"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
	"github.com/rm4n0s/radio-tutor/internal/gui/student"
)

// playedAfter plays the lesson from the start for d and returns what the fake
// device took, then stops the lesson.
func (h *harness) playedAfter(d time.Duration) []float32 {
	h.t.Helper()
	h.playAt(d)
	got := h.playedNow()
	h.tap(KeyStop)
	h.waitPlaying(false)
	return got
}

// playedNow is what the newest fake device took so far.
func (h *harness) playedNow() []float32 {
	h.t.Helper()
	h.mu.Lock()
	n := len(h.sinks)
	h.mu.Unlock()
	return h.sink(n - 1).Played()
}

// reopen leaves the lesson and opens it again.
func (h *harness) reopen() {
	h.t.Helper()
	h.tap(KeyBack)
	h.until("the lesson to be left", func() bool { return !h.HasKey(KeyPlay) && h.HasKey(student.KeyItemPrefix+lesson1) })
	h.tap(student.KeyItemPrefix + lesson1)
	h.loaded()
}

// sliderAt is the point of the volume slider that stands for the percent: the
// track is inset by the 24 pixels of the thumb's overlay.
func (h *harness) sliderAt(percent float64) ui.Offset {
	h.t.Helper()
	f := h.finder(KeyVolume)
	tl, br := h.TopLeft(f), h.BottomRight(f)
	return ui.Offset{Dx: tl.Dx + 24 + percent/100*(br.Dx-tl.Dx-48), Dy: h.Center(f).Dy}
}

// sliderValue is the value the volume slider shows.
func (h *harness) sliderValue() float64 {
	h.t.Helper()
	return gluttertest.WidgetAs[*m.Slider](h.Tester, h.finder(KeyVolume)).Value()
}

// showsMuted reports whether the mute button shows the crossed speaker.
func (h *harness) showsMuted() bool {
	return h.Exists(gluttertest.Descendant(h.finder(KeyMute), gluttertest.ByIcon(m.Icons.VolumeOff())))
}

// saved waits until the settings satisfy cond (the player saves on its own
// goroutine).
func (h *harness) saved(what string, cond func(settings.Settings) bool) {
	h.t.Helper()
	h.until(what, func() bool { return cond(h.Env.Settings.Get()) })
}

// scaled checks that the samples from..to of got are those of full times factor.
func scaled(t *testing.T, what string, got, full []float32, from, to int, factor float32) {
	t.Helper()
	if len(got) < to || len(full) < to {
		t.Fatalf("%s: %d samples played, %d in the reference, want %d", what, len(got), len(full), to)
	}
	for i := from; i < to; i++ {
		want := full[i] * factor
		if got[i] < want-1e-6 || got[i] > want+1e-6 {
			t.Fatalf("%s: sample %d = %v, want %v", what, i, got[i], want)
		}
	}
}

// loud reports whether any sample is not silence.
func loud(samples []float32) bool {
	for _, v := range samples {
		if v != 0 {
			return true
		}
	}
	return false
}

func TestVolumeControlsFollowTheStopButton(t *testing.T) {
	h := pump(t, config{})
	h.loaded()
	for _, k := range []string{KeyMute, KeyVolume} {
		if !h.HasKey(k) {
			t.Fatalf("%s is not shown", k)
		}
	}
	stop, mute, slider := h.BottomRight(h.finder(KeyStop)), h.TopLeft(h.finder(KeyMute)), h.TopLeft(h.finder(KeyVolume))
	play, spacerEnd := h.TopLeft(h.finder(KeyPlay)), h.BottomRight(h.finder(KeyVolume))
	if !(play.Dx < stop.Dx && stop.Dx <= mute.Dx && mute.Dx < slider.Dx) {
		t.Errorf("the order is wrong: play %v, stop ends %v, mute %v, slider %v", play, stop, mute, slider)
	}
	if mute.Dy < h.TopLeft(h.finder(KeyStop)).Dy-1 || spacerEnd.Dx > h.BottomRight(h.finder(KeyProgress)).Dx {
		t.Errorf("the controls are not on the row of the buttons")
	}
	if got := h.sliderValue(); got != 100 {
		t.Errorf("the slider starts at %v, want 100", got)
	}
	if h.showsMuted() {
		t.Error("the sound starts muted")
	}
	if !h.Exists(gluttertest.ByTooltip("Mute")) {
		t.Error("the mute button has no tooltip")
	}
}

func TestVolumeControlsInGreek(t *testing.T) {
	h := pump(t, config{locale: "el_GR.UTF-8"})
	h.loaded()
	if !h.Exists(gluttertest.ByTooltip("Σίγαση")) {
		t.Error("the mute button has no Greek tooltip")
	}
	h.tap(KeyMute)
	if !h.Exists(gluttertest.ByTooltip("Άρση σίγασης")) {
		t.Error("the unmute button has no Greek tooltip")
	}
}

func TestDraggingTheSliderChangesTheSoundAtOnceAndSavesOnRelease(t *testing.T) {
	h := pump(t, config{})
	h.loaded()
	full := h.playedAfter(time.Second)
	if !loud(full) {
		t.Fatal("the narration at full volume is silent")
	}

	h.tap(KeyPlay)
	h.waitPlaying(true)
	h.advance(400 * time.Millisecond)
	g := h.StartGesture(h.sliderAt(100))
	g.MoveTo(h.sliderAt(50))
	h.Pump()
	if got := h.sliderValue(); got != 50 {
		t.Fatalf("the slider shows %v, want 50", got)
	}
	h.advance(500 * time.Millisecond) // the thumb is still down
	if got := h.Env.Settings.Get().Volume; got != 100 {
		t.Errorf("the volume was saved while the thumb was still down: %d", got)
	}
	got := h.playedNow()
	n := len(full)
	// The first 40% of the second is before the drag; the last 40% is after it.
	scaled(t, "before the drag", got, full, 0, n*35/100, 1)
	scaled(t, "after the drag", got, full, n*45/100, n*90/100, 0.125)

	g.Up()
	h.saved("the volume to be saved", func(s settings.Settings) bool { return s.Volume == 50 })
	if s := h.Env.Settings.Get(); s.Muted {
		t.Errorf("settings after the drag = %+v", s)
	}
}

func TestVolumeIsRememberedByTheNextLesson(t *testing.T) {
	h := pump(t, config{})
	h.loaded()
	full := h.playedAfter(time.Second)
	g := h.StartGesture(h.sliderAt(100))
	g.MoveTo(h.sliderAt(50))
	g.Up()
	h.Pump()
	h.saved("the volume to be saved", func(s settings.Settings) bool { return s.Volume == 50 })

	// Leave and open the lesson again: it starts at the saved volume.
	h.reopen()
	if got := h.sliderValue(); got != 50 {
		t.Errorf("the slider of the next lesson shows %v, want 50", got)
	}
	got := h.playedAfter(time.Second)
	scaled(t, "the next lesson", got, full, 0, len(full), 0.125)
}

func TestMuteSilencesTheSoundThatPlaysAndIsSaved(t *testing.T) {
	h := pump(t, config{sound: func(s *settings.Settings) { s.Volume = 80 }})
	h.loaded()
	ref := h.playedAfter(time.Second) // the narration at volume 80
	if !loud(ref) {
		t.Fatal("the narration at volume 80 is silent")
	}

	h.tap(KeyPlay)
	h.waitPlaying(true)
	h.advance(300 * time.Millisecond)
	h.tap(KeyMute)
	if !h.showsMuted() || !h.Exists(gluttertest.ByTooltip("Unmute")) {
		t.Fatal("the button does not show the muted state")
	}
	h.advance(400 * time.Millisecond)
	h.tap(KeyMute)
	if h.showsMuted() {
		t.Fatal("the button still shows the muted state")
	}
	h.advance(300 * time.Millisecond)
	got := h.playedNow()
	n := len(ref)
	scaled(t, "before mute", got, ref, 0, n*25/100, 1)
	for i := n * 35 / 100; i < n*65/100; i++ {
		if got[i] != 0 {
			t.Fatalf("sample %d = %v while muted", i, got[i])
		}
	}
	scaled(t, "after unmute", got, ref, n*75/100, n*95/100, 1)

	// A press saves the mute and keeps the volume.
	h.tap(KeyMute)
	h.saved("the mute to be saved", func(s settings.Settings) bool { return s.Muted })
	if s := h.Env.Settings.Get(); s.Volume != 80 {
		t.Errorf("muting changed the volume to %d", s.Volume)
	}
	h.tap(KeyMute)
	h.saved("the unmute to be saved", func(s settings.Settings) bool { return !s.Muted })
	if s := h.Env.Settings.Get(); s.Volume != 80 {
		t.Errorf("unmuting changed the volume to %d", s.Volume)
	}
}

func audioGain(volume int) float32 {
	f := float32(volume) / 100
	return f * f * f
}

func TestAnotherLessonStartsMutedAtTheSavedVolume(t *testing.T) {
	h := pump(t, config{})
	h.loaded()
	h.tap(KeyMute)
	h.saved("the mute to be saved", func(s settings.Settings) bool { return s.Muted })

	h.reopen()
	if !h.showsMuted() {
		t.Fatal("the next lesson does not start muted")
	}
	if got := h.sliderValue(); got != 100 {
		t.Errorf("the slider shows %v, want the volume that was kept, 100", got)
	}
	if loud(h.playedAfter(time.Second)) {
		t.Error("the next lesson made sound although the sound is muted")
	}
	// Unmuting brings the sound back at the kept volume.
	h.tap(KeyMute)
	if !loud(h.playedAfter(time.Second)) {
		t.Error("unmuting did not bring the sound back")
	}
}

func TestLessonStartsAtTheSettingsOfTheParents(t *testing.T) {
	tests := []struct {
		name   string
		sound  func(*settings.Settings)
		muted  bool
		slider float64
		factor float32 // of the narration at full volume
	}{
		{"volume 35", func(s *settings.Settings) { s.Volume = 35 }, false, 35, audioGain(35)},
		{"muted keeps the volume", func(s *settings.Settings) { s.Volume, s.Muted = 35, true }, true, 35, 0},
		{"volume 0", func(s *settings.Settings) { s.Volume = 0 }, false, 0, 0},
	}
	ref := func() []float32 {
		h := pump(t, config{})
		h.loaded()
		return h.playedAfter(time.Second)
	}()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := pump(t, config{sound: tt.sound})
			h.loaded()
			if h.showsMuted() != tt.muted || h.sliderValue() != tt.slider {
				t.Errorf("muted = %v, slider = %v, want %v and %v", h.showsMuted(), h.sliderValue(), tt.muted, tt.slider)
			}
			got := h.playedAfter(time.Second)
			scaled(t, tt.name, got, ref, 0, len(ref), tt.factor)
		})
	}
}

func TestMovingTheSliderWhileMutedUnmutes(t *testing.T) {
	h := pump(t, config{sound: func(s *settings.Settings) { s.Volume, s.Muted = 100, true }})
	h.loaded()
	if !h.showsMuted() {
		t.Fatal("the lesson does not start muted")
	}
	g := h.StartGesture(h.sliderAt(100))
	g.MoveTo(h.sliderAt(60))
	h.Pump()
	if h.showsMuted() {
		t.Error("the button still shows the muted state while the thumb moves")
	}
	g.Up()
	h.saved("the volume to be saved", func(s settings.Settings) bool { return s.Volume == 60 && !s.Muted })
	if !loud(h.playedAfter(time.Second)) {
		t.Error("the lesson is silent after the slider was moved")
	}
}

func TestVolumeSaveFailureIsShownAndTheSoundStaysAsSet(t *testing.T) {
	h := pump(t, config{})
	h.loaded()
	// A settings file that cannot be written: a folder where the file goes.
	if err := os.MkdirAll(h.Env.Paths.SettingsFile(), 0o700); err != nil {
		t.Fatal(err)
	}
	h.tap(KeyMute)
	h.until("the error", func() bool { return h.hasText("Something went wrong. Ask a parent.") })
	if !h.showsMuted() {
		t.Error("the button went back to the sound on")
	}
	if loud(h.playedAfter(time.Second)) {
		t.Error("the sound came back after the save failed")
	}
	if h.Env.Settings.Get().Muted {
		t.Error("the setting changed although it could not be saved")
	}
}

func TestLessonAtVolumeZeroCanStillBeCompleted(t *testing.T) {
	tests := []struct {
		name  string
		sound func(*settings.Settings)
	}{
		{"volume 0", func(s *settings.Settings) { s.Volume = 0 }},
		{"muted", func(s *settings.Settings) { s.Muted = true }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := pump(t, config{sound: tt.sound})
			h.loaded()
			h.playAt(3 * time.Second)
			h.until("Next", func() bool { return h.HasKey(KeyNext) })
			if lp := h.savedProgress().Lessons[lesson1]; !lp.Completed() {
				t.Errorf("lesson progress = %+v, want completed", lp)
			}
		})
	}
}

func TestVolumeGoldens(t *testing.T) {
	t.Run("muted", func(t *testing.T) {
		h := pump(t, config{sound: func(s *settings.Settings) { s.Volume, s.Muted = 35, true }})
		h.loaded()
		h.MatchesGolden("controls_muted")
	})
	t.Run("dragging", func(t *testing.T) {
		h := pump(t, config{})
		h.loaded()
		g := h.StartGesture(h.sliderAt(100))
		g.MoveTo(h.sliderAt(60))
		for range 20 {
			h.Pump(16 * time.Millisecond)
		}
		h.MatchesGolden("controls_dragging")
		g.Up()
		h.Pump(300 * time.Millisecond)
	})
	t.Run("greek muted", func(t *testing.T) {
		h := pump(t, config{locale: "el_GR.UTF-8", sound: func(s *settings.Settings) { s.Muted = true }})
		h.loaded()
		h.MatchesGolden("controls_muted_el")
	})
	// The widest row: Greek labels with Skip and Next at the minimum window width.
	t.Run("greek skip and next", func(t *testing.T) {
		h := pump(t, config{locale: "el_GR.UTF-8", skip: true, lessonsFirst: true})
		h.loaded()
		h.tap(KeyPlay)
		h.until("Pause", func() bool {
			return h.Exists(gluttertest.Descendant(h.finder(KeyPlay), gluttertest.ByText("Παύση")))
		})
		h.advance(3 * time.Second)
		h.until("Next", func() bool { return h.HasKey(KeyNext) && h.HasKey(KeySkip) })
		h.until("Play", func() bool {
			return h.Exists(gluttertest.Descendant(h.finder(KeyPlay), gluttertest.ByText("Αναπαραγωγή")))
		})
		h.MatchesGolden("controls_el_skip_next")
		// The sound controls wrapped under Play: they do not run into Skip.
		sliderEnd, skipStart := h.BottomRight(h.finder(KeyVolume)), h.TopLeft(h.finder(KeySkip))
		if sliderEnd.Dx > skipStart.Dx && sliderEnd.Dy > skipStart.Dy && h.TopLeft(h.finder(KeyVolume)).Dy < h.BottomRight(h.finder(KeySkip)).Dy {
			t.Errorf("the slider (ends at %v) overlaps Skip (starts at %v)", sliderEnd, skipStart)
		}
	})
	t.Run("save failed", func(t *testing.T) {
		h := pump(t, config{})
		h.loaded()
		if err := os.MkdirAll(h.Env.Paths.SettingsFile(), 0o700); err != nil {
			t.Fatal(err)
		}
		h.tap(KeyMute)
		h.until("the error", func() bool { return h.hasText("Something went wrong. Ask a parent.") })
		h.Pump(500 * time.Millisecond)
		h.MatchesGolden("controls_save_failed")
	})
}

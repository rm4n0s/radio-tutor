package player

import (
	"context"
	"testing"
	"time"

	"github.com/rm4n0s/radio-tutor/internal/core/settings"
)

// setVolume changes the volume setting of the parents, as the settings screen does.
func (h *harness) setVolume(v int) {
	h.t.Helper()
	if err := h.Env.Settings.Update(context.Background(), func(s *settings.Settings) { s.Volume = v }); err != nil {
		h.t.Fatal(err)
	}
}

// playedAfter plays the lesson from the start for d and returns what the fake
// device took, then stops the lesson.
func (h *harness) playedAfter(d time.Duration) []float32 {
	h.t.Helper()
	h.playAt(d)
	got := h.sink(0).Played()
	h.tap(KeyStop)
	h.waitPlaying(false)
	return got
}

func TestLessonSoundFollowsTheVolumeSetting(t *testing.T) {
	h := pump(t, config{})
	h.loaded()
	full := h.playedAfter(time.Second)
	loud := false
	for _, v := range full {
		loud = loud || v != 0
	}
	if len(full) == 0 || !loud {
		t.Fatalf("the narration at full volume has %d samples and is silent: %v", len(full), loud)
	}

	tests := []struct {
		name   string
		volume int
		factor float32
	}{
		{"half the slider is an eighth of the gain", 50, 0.125},
		{"zero is silence", 0, 0},
		{"full is unchanged", 100, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h.setVolume(tt.volume)
			got := h.playedAfter(time.Second)
			if len(got) != len(full) {
				t.Fatalf("the device took %d samples, want %d", len(got), len(full))
			}
			for i, v := range got {
				want := full[i] * tt.factor
				if v < want-1e-6 || v > want+1e-6 {
					t.Fatalf("sample %d = %v, want %v", i, v, want)
				}
			}
		})
	}
}

func TestLessonAtVolumeZeroCanStillBeCompleted(t *testing.T) {
	h := pump(t, config{})
	h.loaded()
	h.setVolume(0)
	h.playAt(3 * time.Second)
	h.until("Next", func() bool { return h.HasKey(KeyNext) })
	if lp := h.savedProgress().Lessons[lesson1]; !lp.Completed() {
		t.Errorf("lesson progress = %+v, want completed at volume 0", lp)
	}
}

package worksheet

import (
	"context"
	"testing"
	"time"

	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/audio"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
)

const bannerText = "Solve the exercises on paper. Write the number of each exercise next to its solution."

func TestWorksheetShowsTheExercises(t *testing.T) {
	h := pump(t, config{})
	for _, text := range []string{
		bannerText, "Worksheet 1", "Shade the half", "Name the fraction", "B2",
		"Draw a circle, cut it into 2 equal parts and shade 1 part.",
		"2 selected", "Check my answers",
	} {
		if !h.has(text) {
			t.Errorf("the worksheet does not show %q", text)
		}
	}
	for _, k := range []string{KeyBanner, KeyCardPrefix + exShade, KeyCardPrefix + exName, KeyBadgePrefix + exShade, KeyBadgePrefix + exName} {
		if !h.HasKey(k) {
			t.Errorf("no widget with the key %s", k)
		}
	}
	// Only the first exercise has a recording.
	if !h.HasKey(KeySpeakerPrefix + exShade) {
		t.Error("the exercise with prompt.ogg has no speaker button")
	}
	if h.HasKey(KeySpeakerPrefix + exName) {
		t.Error("an exercise without prompt.ogg has a speaker button")
	}
	if h.HasKey(KeyCardPrefix + exSum) {
		t.Error("the second worksheet's exercise is on the first worksheet")
	}
}

func TestWorksheetShowsStatesAttemptsAndHints(t *testing.T) {
	h := pump(t, config{change: func(p *progress.Progress) {
		lessonDone(p)
		p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StatePassed, Attempts: 1, PassedBy: progress.PassedByAI}
		p.Exercises[exName] = progress.ExerciseProgress{Attempts: 1, HintsShown: 1}
	}})
	if !h.has("Passed") {
		t.Error("the passed exercise is not marked")
	}
	if !h.has("Hint 1: How many slices are there in all?") || !h.HasKey(KeyHintPrefix+exName+"-1") {
		t.Error("the unlocked hint is not shown")
	}
	// A passed exercise cannot be ticked and is not selected; only the other one is.
	if !h.has("1 selected") {
		t.Error("only the available exercise should be selected")
	}
	if h.Widget(key(KeySelectPrefix + exShade)).(*m.Checkbox).Value().OrElse(true) {
		t.Error("the passed exercise is ticked")
	}
}

func TestWorksheetNeedsReviewAndAttemptsLeft(t *testing.T) {
	h := pump(t, config{change: func(p *progress.Progress) {
		lessonDone(p)
		p.Exercises[exShade] = progress.ExerciseProgress{State: progress.StateNeedsReview, Attempts: 3}
	}})
	if !h.has("Ask a parent to look at this exercise.") {
		t.Error("the exercise that needs a review has no message")
	}
	// The exercise with a limit shows its tries; the one without does not.
	h2 := pump(t, config{change: func(p *progress.Progress) {
		lessonDone(p)
		p.Exercises[exShade] = progress.ExerciseProgress{Attempts: 1}
	}})
	if h2.has("Tries left: 2") {
		// exShade has the default of 3 tries and 1 is used.
		return
	}
	t.Error("the tries left are not shown")
}

func TestSelectionAndCheckButton(t *testing.T) {
	h := pump(t, config{})
	h.TapKey(KeySelectPrefix + exShade)
	if !h.has("1 selected") {
		t.Fatal("unticking one exercise should leave one selected")
	}
	h.TapKey(KeySelectPrefix + exName)
	if !h.has("Choose at least one exercise.") {
		t.Fatal("nothing selected should say so")
	}
	// Tapping the disabled button does nothing.
	h.Tap(key(KeyCheck))
	h.PumpAndSettle()
	if h.HasKey(KeyQR) || h.HasKey(KeyGrading) {
		t.Fatal("Check my answers opened a submission with no exercise selected")
	}
	h.TapKey(KeySelectPrefix + exName)
	if !h.has("1 selected") {
		t.Error("ticking an exercise again should select it")
	}
}

func TestWorksheetGreek(t *testing.T) {
	h := pump(t, config{locale: "el_GR.UTF-8"})
	for _, text := range []string{
		"Λύσε τις ασκήσεις στο χαρτί. Γράψε τον αριθμό κάθε άσκησης δίπλα στη λύση της.",
		"Φύλλο ασκήσεων 1", "Έλεγχος των απαντήσεων", "Επιλέχθηκαν 2",
	} {
		if !h.has(text) {
			t.Errorf("the Greek worksheet does not show %q", text)
		}
	}
}

func TestPromptAudioPlaysAndStops(t *testing.T) {
	sink := audio.NewFakeSink(audio.NewFakeClock(time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)))
	h := pump(t, config{sink: func(context.Context) (audio.Sink, error) { return sink, nil }})
	h.tap(KeySpeakerPrefix + exShade)
	h.whileBusy("the sink to start", func() bool { return len(sink.Calls()) > 0 })
	h.whileBusy("the Stop button", func() bool { return h.has("Stop") })
	h.tap(KeySpeakerPrefix + exShade)
	h.whileBusy("the Listen button", func() bool { return h.has("Listen") })
	calls := sink.Calls()
	if len(calls) < 2 {
		t.Fatalf("sink calls %v, want a start and a stop", calls)
	}
	if calls[0] != "start" || calls[len(calls)-1] != "stop" {
		t.Errorf("sink calls %v, want start first and stop last", calls)
	}
}

func TestPromptAudioFailureIsShown(t *testing.T) {
	h := pump(t, config{sink: func(context.Context) (audio.Sink, error) {
		s := audio.NewFakeSink(audio.NewFakeClock(time.Now()))
		s.FailStartWith(context.DeadlineExceeded)
		return s, nil
	}})
	h.tap(KeySpeakerPrefix + exShade)
	h.whileBusy("the message", func() bool { return h.has("The sound could not be played.") })
}

func TestTooManyExercisesIsRefused(t *testing.T) {
	// The limit of one submission is 10 and a worksheet may have more exercises;
	// the fixture has two, so the state is given ten more.
	h := pump(t, config{})
	st := h.stateOf()
	for i := range 10 {
		id := "extra-" + string(rune('a'+i))
		st.sheet.Exercises = append(st.sheet.Exercises, exerciseView{ID: id, State: progress.StateAvailable})
		st.selected[id] = true
	}
	st.toggle("extra-a", true)
	h.PumpAndSettle()
	if !h.has("You can check at most 10 exercises at once.") {
		t.Error("no message about the limit")
	}
}

// stateOf returns the state of the worksheet screen.
func (h *harness) stateOf() *screenState {
	h.t.Helper()
	el := h.Element(gluttertest.ByType[*Screen]())
	st, ok := el.(interface{ State() m.State }).State().(*screenState)
	if !ok {
		h.t.Fatal("no worksheet state")
	}
	return st
}

func TestPromptAudioFollowsTheVolumeSetting(t *testing.T) {
	clock := audio.NewFakeClock(time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC))
	sink := audio.NewFakeSink(clock)
	h := pump(t, config{sink: func(context.Context) (audio.Sink, error) { return sink, nil }})
	// listen plays the prompt from the start for a while, stops it and returns
	// what the fake device took.
	listen := func() []float32 {
		h.tap(KeySpeakerPrefix + exShade)
		h.whileBusy("the Stop button", func() bool { return h.has("Stop") })
		clock.Advance(100 * time.Millisecond)
		got := sink.Played()
		h.tap(KeySpeakerPrefix + exShade)
		h.whileBusy("the Listen button", func() bool { return h.has("Listen") })
		return got
	}
	full := listen()
	loud := false
	for _, v := range full {
		loud = loud || v != 0
	}
	if len(full) == 0 || !loud {
		t.Fatalf("the prompt at full volume has %d samples and is silent: %v", len(full), loud)
	}

	tests := []struct {
		name   string
		volume int
		factor float32
	}{
		{"half the slider is an eighth of the gain", 50, 0.125},
		{"zero is silence", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.Env.Settings.Update(context.Background(), func(s *settings.Settings) { s.Volume = tt.volume })
			if err != nil {
				t.Fatal(err)
			}
			got := listen()
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

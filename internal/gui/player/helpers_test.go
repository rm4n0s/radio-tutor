package player

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/audio"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/student"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

const (
	cassetteID    = "org.example.archive-1"
	lesson1       = "what-is-a-fraction"
	lesson2       = "adding-fractions"
	exShade       = "shade-the-half"
	password      = "abcd"
	validCassette = "../../core/archive/testdata/valid"
	waitLimit     = 5 * time.Second
)

// writeLesson installs the test lesson over lesson1 of a copy of the archive
// package's valid cassette and, with lessonsFirst, puts lesson2 right after it
// so that Next opens a lesson. It returns the folder to import.
func writeLesson(t testing.TB, lessonsFirst, lastItem bool) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "cassette")
	if err := os.CopyFS(dst, os.DirFS(validCassette)); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"scene.svg", "lesson.json"} {
		data, err := os.ReadFile(filepath.Join("testdata", "lesson", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, "lessons", lesson1, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if lastItem {
		file := filepath.Join(dst, "cassette.json")
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		start, end := strings.Index(text, `"items"`), strings.LastIndex(text, "]")
		items := `"items": [
    {"type": "lesson", "id": "adding-fractions"},
    {"type": "exercise", "id": "shade-the-half"},
    {"type": "exercise", "id": "name-the-fraction"},
    {"type": "exercise", "id": "write-the-sum"},
    {"type": "lesson", "id": "what-is-a-fraction"}
  `
		if err := os.WriteFile(file, []byte(text[:start]+items+text[end:]), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if lessonsFirst {
		file := filepath.Join(dst, "cassette.json")
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		from := `{"type": "lesson", "id": "adding-fractions"},`
		text := strings.Replace(string(data), from, "", 1)
		text = strings.Replace(text, `{"type": "exercise", "id": "shade-the-half"},`, from+"\n    "+`{"type": "exercise", "id": "shade-the-half"},`, 1)
		if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

// harness is the app with the player, a fake sink on a fake clock and the
// student "mia" on the lesson.
type harness struct {
	*testutil.App
	t     *testing.T
	clock *audio.FakeClock

	mu    sync.Mutex
	sinks []*audio.FakeSink
	opens int
}

// config is what a test sets up.
type config struct {
	// lastItem makes lesson1 the last item of the cassette (with free
	// navigation, so that it is open).
	lastItem bool
	// lessonsFirst puts lesson2 right after lesson1.
	lessonsFirst bool
	// skip turns skip_lessons on for mia.
	skip bool
	// noSound makes the audio opener report that there is no sound device.
	noSound bool
	// gate, when set, holds the audio opener (which runs after the narration is
	// decoded) until it is closed, so the lesson stays in the loading state.
	gate chan struct{}
	// locale is the LANG of the app.
	locale string
	// openLesson opens lesson1 right after the login (the default); false stops
	// at the cassette screen.
	stayOnCassette bool
}

// pump opens the app, logs mia in and opens lesson1 of the test cassette. The
// lesson is loading, or loaded, when it returns.
func pump(t *testing.T, c config) *harness {
	t.Helper()
	h := &harness{t: t, clock: audio.NewFakeClock(time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC))}
	open := func(context.Context) (audio.Sink, error) {
		if c.gate != nil {
			<-c.gate
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		h.opens++
		if c.noSound {
			// What audio.OpenDefaultSink returns when no server answers.
			return audio.NewSilentSink(h.clock), nil
		}
		s := audio.NewFakeSink(h.clock)
		h.sinks = append(h.sinks, s)
		return s, nil
	}
	worksheet := func(req student.WorksheetRequest) m.Widget {
		return m.NewScaffold(m.ScaffoldOpts{Body: m.NewCenter(m.NewText("worksheet opened"))})
	}
	screens := student.Screens{Lesson: Builder(), Worksheet: worksheet}
	a := testutil.Pump(t, testutil.Options{
		Locale:      c.locale,
		SinkFactory: open,
		AudioClock:  h.clock,
		Screens:     app.Screens{Student: student.HomeBuilder(screens)},
		Before: func(env *app.Env) {
			testutil.AddProfile(t, env, "mum", "Mum", profile.RoleAdmin, "parent-secret")
			testutil.AddProfile(t, env, "mia", "Mia", profile.RoleStudent, password)
			res, err := env.Archive.Import(context.Background(), writeLesson(t, c.lessonsFirst, c.lastItem), nil)
			if err != nil {
				t.Fatalf("installing the test cassette: %v", err)
			}
			if len(res.Problems) > 0 {
				t.Fatalf("installing the test cassette: %v", res.Problems)
			}
			if c.skip || c.lastItem {
				err := env.Progress.Update(context.Background(), "mia", cassetteID, func(p *progress.Progress) error {
					p.Settings.SkipLessons = c.skip
					p.Settings.FreeNavigation = c.lastItem
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
		},
	})
	h.App = a
	a.TapKey(app.KeyProfilePrefix + "mia")
	a.EnterKey(app.KeyPassword, password)
	a.TapKey(app.KeySubmit)
	a.WaitFor(t, "the shelf", func() bool { return a.HasKey(app.KeyLogout) })
	a.WaitFor(t, "the cards", func() bool { return a.HasKey(student.KeyCardPrefix + cassetteID) })
	a.TapKey(student.KeyCardPrefix + cassetteID)
	a.WaitFor(t, "the cassette screen", func() bool { return a.HasKey(student.KeyContinue) })
	if !c.stayOnCassette {
		a.TapKey(student.KeyItemPrefix + lesson1)
	}
	return h
}

// until pumps single frames (a playing lesson never settles) until cond holds.
func (h *harness) until(what string, cond func() bool) {
	h.t.Helper()
	deadline := time.Now().Add(waitLimit)
	for {
		h.FlushPosted()
		h.Pump(16 * time.Millisecond)
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// loaded waits until the lesson is ready to play.
func (h *harness) loaded() {
	h.t.Helper()
	h.until("the lesson to load", func() bool { return h.HasKey(KeyPlay) })
}

// failedToLoad waits for the failure message.
func (h *harness) failedToLoad() {
	h.t.Helper()
	h.until("the failure message", func() bool {
		return h.Exists(gluttertest.ByText("This lesson could not be opened. Ask a parent for help."))
	})
}

// tap taps the widget with the key and pumps one frame.
func (h *harness) tap(k string) {
	h.t.Helper()
	h.Tap(h.finder(k))
	h.Pump()
}

func (h *harness) finder(k string) gluttertest.Finder { return gluttertest.ByKey(m.NewValueKey(k)) }

// hasText reports whether the text is on screen.
func (h *harness) hasText(text string) bool { return h.Exists(gluttertest.ByText(text)) }

// isPlaying waits until the Play button has turned into Pause (or back).
func (h *harness) waitPlaying(playing bool) {
	h.t.Helper()
	label := "Play"
	if playing {
		label = "Pause"
	}
	h.until("the "+label+" button", func() bool {
		return h.Exists(gluttertest.Descendant(h.finder(KeyPlay), gluttertest.ByText(label)))
	})
}

// advance moves the audio clock and draws a frame.
func (h *harness) advance(d time.Duration) {
	h.t.Helper()
	h.clock.Advance(d)
	h.Pump(d)
}

// sink returns the n-th sink the player opened.
func (h *harness) sink(n int) *audio.FakeSink {
	h.mu.Lock()
	defer h.mu.Unlock()
	if n >= len(h.sinks) {
		h.t.Fatalf("the player opened %d sinks, want at least %d", len(h.sinks), n+1)
	}
	return h.sinks[n]
}

// textOf returns the text of the widget with the key.
func (h *harness) textOf(k string) string {
	h.t.Helper()
	w, ok := h.Widget(h.finder(k)).(*m.Text)
	if !ok {
		h.t.Fatalf("%s is not a Text", k)
	}
	return w.Data().OrElse("")
}

// savedProgress loads mia's save file.
func (h *harness) savedProgress() *progress.Progress {
	h.t.Helper()
	p, err := h.Env.Progress.Load(context.Background(), "mia", cassetteID)
	if err != nil {
		h.t.Fatal(err)
	}
	return p
}

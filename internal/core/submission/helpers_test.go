package submission

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/audio"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/grading/fake"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/phone"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/providers"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
)

const eventWait = 5 * time.Second

// testClock adapts the audio package's fake clock to the Clock interface.
type testClock struct{ *audio.FakeClock }

func (c testClock) AfterFunc(d time.Duration, f func()) Timer { return c.FakeClock.AfterFunc(d, f) }

// stubProviders is a ProviderSource the tests switch by hand.
type stubProviders struct {
	mu     sync.Mutex
	active bool
}

func (p *stubProviders) Active() (providers.Config, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return providers.Config{Name: "Test provider"}, p.active
}

func (p *stubProviders) set(active bool) {
	p.mu.Lock()
	p.active = active
	p.mu.Unlock()
}

// stubSettings is a SettingsSource the tests change by hand.
type stubSettings struct {
	mu sync.Mutex
	s  settings.Settings
}

func (s *stubSettings) Get() settings.Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.s
}

func (s *stubSettings) update(f func(*settings.Settings)) {
	s.mu.Lock()
	f(&s.s)
	s.mu.Unlock()
}

// env is a service over isolated storage, the fake provider, a fake clock and a
// real phone server on 127.0.0.1.
type env struct {
	t         *testing.T
	paths     storage.Paths
	progress  *progress.Store
	phone     *phone.Server
	svc       *Service
	clock     testClock
	providers *stubProviders
	settings  *stubSettings
	cass      *cassette.Cassette

	mu       sync.Mutex
	provider grading.Provider
	built    []providers.Config
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{
		t:         t,
		paths:     storagetest.Isolate(t),
		clock:     testClock{audio.NewFakeClock(time.Date(2026, 10, 30, 17, 19, 55, 0, time.UTC))},
		providers: &stubProviders{active: true},
		settings:  &stubSettings{s: settings.Settings{Language: i18n.EN}},
		cass:      testCassette(),
	}
	e.progress = progress.NewStore(e.paths)
	e.phone = phone.New(phone.Options{ListenHost: "127.0.0.1", Clock: e.clock.Now})
	ctx, cancel := context.WithCancel(context.Background())
	e.svc = New(ctx, Options{
		Paths:     e.paths,
		Progress:  e.progress,
		Providers: e.providers,
		Settings:  e.settings,
		Phone:     e.phone,
		Clock:     e.clock,
		DetectLAN: func(context.Context, string) (phone.LANAddr, error) {
			return phone.LANAddr{IP: net.ParseIP("127.0.0.1"), Interface: "test0"}, nil
		},
		BuildProvider: func(cfg providers.Config, _ *slog.Logger) (grading.Provider, bool, error) {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.built = append(e.built, cfg)
			return e.provider, false, nil
		},
	})
	t.Cleanup(func() {
		if err := e.svc.Close(context.Background()); err != nil {
			t.Errorf("Close: %v", err)
		}
		cancel()
	})
	return e
}

// use sets the provider the service builds.
func (e *env) use(p grading.Provider) {
	e.mu.Lock()
	e.provider = p
	e.mu.Unlock()
}

// open opens a submission for the exercises and fails the test on an error.
func (e *env) open(user string, ids ...string) *Session {
	e.t.Helper()
	s, err := e.svc.Open(context.Background(), user, e.cass, ids)
	if err != nil {
		e.t.Fatalf("Open: %v", err)
	}
	return s
}

// load reads the save file of the student.
func (e *env) load(user string) *progress.Progress {
	e.t.Helper()
	p, err := e.progress.Load(context.Background(), user, e.cass.Manifest.ID)
	if err != nil {
		e.t.Fatalf("Load: %v", err)
	}
	return p
}

// submission returns the stored submission of the session.
func (e *env) submission(s *Session) progress.Submission {
	e.t.Helper()
	p := e.load(s.Username)
	sub := p.Submission(s.ID)
	if sub == nil {
		e.t.Fatalf("the save file has no submission %s", s.ID)
	}
	return *sub
}

// save changes the save file of the student.
func (e *env) save(user string, change func(p *progress.Progress)) {
	e.t.Helper()
	err := e.progress.Update(context.Background(), user, e.cass.Manifest.ID, func(p *progress.Progress) error {
		change(p)
		return nil
	})
	if err != nil {
		e.t.Fatalf("Update: %v", err)
	}
}

// testCassette has two worksheets, e1 e2 e3 and (after a lesson) e4.
func testCassette() *cassette.Cassette {
	ex := func(id, label string, attempts int, hints ...string) *cassette.Exercise {
		return &cassette.Exercise{
			ID: id, Format: 1, Title: "Title of " + id, Label: label, Prompt: "Prompt of " + id,
			ReferenceSolution: "answer " + id, Grading: "grade " + id, MaxAttempts: attempts, Hints: hints,
		}
	}
	item := func(typ cassette.ItemType, id string) cassette.Item { return cassette.Item{Type: typ, ID: id} }
	return &cassette.Cassette{
		Manifest: cassette.Manifest{
			Format: 1, ID: "org.example.sub-1", Version: cassette.Version{Major: 1}, Title: "Fractions for all",
			Language: "en", Ages: []int{7, 9},
			Items: []cassette.Item{
				item(cassette.ItemExercise, "e1"), item(cassette.ItemExercise, "e2"), item(cassette.ItemExercise, "e3"),
				item(cassette.ItemLesson, "l1"),
				item(cassette.ItemExercise, "e4"),
			},
		},
		Lessons: map[string]*cassette.Lesson{"l1": {ID: "l1"}},
		Exercises: map[string]*cassette.Exercise{
			"e1": ex("e1", "1", 3, "hint 1a", "hint 1b"),
			"e2": ex("e2", "2", 2, "hint 2a"),
			"e3": ex("e3", "3", 3),
			"e4": ex("e4", "4", 3),
		},
	}
}

// res is a grading result with the verdict.
func res(id string, v grading.Verdict) grading.ExerciseResult {
	return grading.ExerciseResult{
		ExerciseID: id, Verdict: v, Transcription: "read " + id, FeedbackForChild: "feedback " + id, NotesForParent: "notes " + id,
	}
}

// reply is the JSON text of an answer with the results.
func reply(results ...grading.ExerciseResult) string {
	return fake.JSONReply("the page notes", results...)
}

// photo returns a small JPEG photo.
func photo(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x * 255 / w), uint8(y * 255 / h), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// upload posts the photos for the session's offer the way the phone page does
// and returns the status and the body.
func upload(t *testing.T, sess *Session, photos ...[]byte) (int, string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for i, data := range photos {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="photos"; filename="p`+string(rune('1'+i))+`.jpg"`)
		h.Set("Content-Type", "image/jpeg")
		part, err := mw.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(sess.URL+"/photos?offer="+strconv.FormatUint(sess.offer.ID(), 10), mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	text, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(text)
}

// get fetches the URL and returns the status and the body.
func get(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	text, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(text)
}

// next returns the next event of the session.
func next(t *testing.T, s *Session) Event {
	t.Helper()
	select {
	case ev, ok := <-s.Events():
		if !ok {
			t.Fatal("the event channel is closed")
		}
		return ev
	case <-time.After(eventWait):
		t.Fatal("no event")
	}
	return nil
}

// expect returns the next event of the session, which must have the type T.
func expect[T Event](t *testing.T, s *Session) T {
	t.Helper()
	ev := next(t, s)
	got, ok := ev.(T)
	if !ok {
		var want T
		t.Fatalf("event %#v, want %T", ev, want)
	}
	return got
}

// expectClosed fails unless the event channel is closed with nothing left.
func expectClosed(t *testing.T, s *Session) {
	t.Helper()
	select {
	case ev, ok := <-s.Events():
		if ok {
			t.Fatalf("unexpected event %#v", ev)
		}
	case <-time.After(eventWait):
		t.Fatal("the event channel was not closed")
	}
}

// expectQuiet fails if an event is waiting.
func expectQuiet(t *testing.T, s *Session) {
	t.Helper()
	select {
	case ev, ok := <-s.Events():
		t.Fatalf("unexpected event %#v (open %v)", ev, ok)
	default:
	}
}

// wantRoute fails unless err is an *errors.Error whose route contains route.
func wantRoute(t *testing.T, err error, route string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error with route %q, got none", route)
	}
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T: %v", err, err)
	}
	if !appErr.HasRoute(route) {
		t.Fatalf("unexpected failure path: %s (want %s)", appErr.Route(), route)
	}
}

// photoPath returns the folder of the session's photos.
func (e *env) photoPath(s *Session) string {
	return filepath.Join(e.paths.PhotosDir(), s.Username, s.CassetteID, s.ID)
}

// wantMode fails unless the path has the permission bits.
func wantMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	if !storagetest.UnixModes {
		return // Windows has no permission bits
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s has mode %v, want %v", path, got, want)
	}
}

// skipIfRoot skips a test that needs file permissions to be enforced.
func skipIfRoot(t *testing.T) {
	t.Helper()
	storagetest.SkipIfModesDoNotBlock(t)
}

// contains reports whether text has all the parts.
func contains(text string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(text, p) {
			return false
		}
	}
	return true
}

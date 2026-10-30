package worksheet

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
	"sync"
	"testing"
	"time"

	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/archive"
	"github.com/rm4n0s/radio-tutor/internal/core/audio"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/phone"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/providers"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
	"github.com/rm4n0s/radio-tutor/internal/core/submission"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/student"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

const (
	// tallWindow is the height of the test window, so that the whole worksheet is
	// on screen and every button can be tapped.
	tallWindow = 1100.0

	studentPassword = "abcd"
	fractionsID     = "org.example.archive-1"
	fractionsTitle  = "Fractions, part 1"
	// The fixture cassette: a lesson, a worksheet of two exercises (the first has
	// a picture and a recording), a lesson, a worksheet of one exercise.
	lesson1  = "what-is-a-fraction"
	exShade  = "shade-the-half"    // label "1" (its position), prompt.svg and prompt.ogg
	exName   = "name-the-fraction" // label "B2", one hint, no limit of tries
	exSum    = "write-the-sum"     // label "3", in the second worksheet
	testdata = "../../core/archive/testdata/valid"

	// goldenPort is the port of the golden tests, so the address in the QR dialog
	// is the same on every run.
	goldenPort = 38737
)

var completedAt = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

// cycle is an endless reader of the same bytes; as the random source of the
// phone server it gives the first token a fixed value.
type cycle struct {
	pattern []byte
	pos     int
}

func (c *cycle) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = c.pattern[c.pos%len(c.pattern)]
		c.pos++
	}
	return len(p), nil
}

// testClock adapts the audio package's fake clock to the submission service.
type testClock struct{ *audio.FakeClock }

func (c testClock) AfterFunc(d time.Duration, f func()) submission.Timer {
	return c.FakeClock.AfterFunc(d, f)
}

// scripted is the provider the service builds: tests replace it.
type scripted struct {
	mu    sync.Mutex
	p     grading.Provider
	built int
}

func (s *scripted) use(p grading.Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.p = p
}

func (s *scripted) build(providers.Config, *slog.Logger) (grading.Provider, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.built++
	return s.p, false, nil
}

// config is what a test sets up.
type config struct {
	locale string
	size   m.Size
	// change edits the save file of mia before the window opens; the default
	// completes the first lesson so that the first worksheet is available.
	change func(p *progress.Progress)
	// noProvider leaves the active provider unset.
	noProvider bool
	// port is the phone port; 0 picks a free one.
	port int
	// sink is the audio sink factory; the default fails the test.
	sink app.SinkFactory
}

// harness is the app pumped to the worksheet.
type harness struct {
	*testutil.App
	t        *testing.T
	provider *scripted
	phone    *phone.Server
	clock    testClock

	mu     sync.Mutex
	lanErr error
}

// failNextOpen makes the next LAN detection, which the next Open makes, fail
// with err.
func (h *harness) failNextOpen(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lanErr = err
}

// takeLANErr returns the error for the LAN detection and forgets it.
func (h *harness) takeLANErr() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	err := h.lanErr
	h.lanErr = nil
	return err
}

// lessonDone completes the first lesson, which makes the first worksheet available.
func lessonDone(p *progress.Progress) {
	p.Lessons[lesson1] = progress.LessonProgress{CompletedAt: completedAt, Plays: 1}
}

// freePort returns a TCP port that nobody listens on.
func freePort(t testing.TB) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ln.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

// pump opens the app with an admin ("mum") and a student ("mia"), the fixture
// cassette, a fake provider and a phone server on 127.0.0.1, logs mia in and
// opens the first worksheet.
func pump(t *testing.T, c config) *harness {
	t.Helper()
	if c.size == (m.Size{}) {
		c.size = m.Size{Width: app.MinWindowWidth, Height: tallWindow}
	}
	if c.change == nil {
		c.change = lessonDone
	}
	if c.port == 0 {
		c.port = freePort(t)
	}
	if c.sink == nil {
		c.sink = func(context.Context) (audio.Sink, error) {
			t.Error("the worksheet opened an audio sink and the test gave no factory")
			return audio.NewFakeSink(audio.NewFakeClock(time.Now())), nil
		}
	}
	h := &harness{t: t, provider: &scripted{}, clock: testClock{audio.NewFakeClock(time.Date(2026, 10, 30, 17, 19, 55, 0, time.UTC))}}
	h.phone = phone.New(phone.Options{ListenHost: "127.0.0.1", Random: &cycle{pattern: []byte("RadioTutor-token")}})
	h.App = testutil.Pump(t, testutil.Options{
		Locale:      c.locale,
		Size:        c.size,
		Screens:     app.Screens{Student: student.HomeBuilder(student.Screens{Worksheet: Builder()})},
		SinkFactory: c.sink,
		Submission: app.SubmissionOptions{
			Phone:         h.phone,
			Clock:         h.clock,
			BuildProvider: h.provider.build,
			DetectLAN: func(context.Context, string) (phone.LANAddr, error) {
				if err := h.takeLANErr(); err != nil {
					return phone.LANAddr{}, err
				}
				return phone.LANAddr{IP: net.ParseIP("127.0.0.1"), Interface: "test0"}, nil
			},
		},
		Before: func(env *app.Env) {
			testutil.AddProfile(t, env, "mum", "Mum", profile.RoleAdmin, "parent-secret")
			testutil.AddProfile(t, env, "mia", "Mia", profile.RoleStudent, studentPassword)
			install(t, env, fixture(t))
			setProgress(t, env, c.change)
			err := env.Settings.Update(context.Background(), func(s *settings.Settings) { s.Phone.Port = c.port })
			if err != nil {
				t.Fatal(err)
			}
			if !c.noProvider {
				cfg := providers.Config{
					Name: "Test checker", Protocol: "openai", BaseURL: "http://127.0.0.1:1/v1", Model: "fake",
					Kind: providers.KindLocal, StructuredOutput: grading.StructuredJSONSchema,
				}
				if err := env.Providers.Add(context.Background(), cfg); err != nil {
					t.Fatal(err)
				}
				if err := env.Providers.SetActive(context.Background(), cfg.Name); err != nil {
					t.Fatal(err)
				}
			}
		},
	})
	h.TapKey(app.KeyProfilePrefix + "mia")
	h.EnterKey(app.KeyPassword, studentPassword)
	h.TapKey(app.KeySubmit)
	h.WaitFor(t, "the shelf", func() bool { return h.HasKey(app.KeyLogout) })
	h.TapKey(student.KeyCardPrefix + fractionsID)
	h.WaitFor(t, "the cassette screen", func() bool { return h.HasKey(student.KeyContinue) })
	h.TapKey(student.WorksheetKey(0))
	h.WaitFor(t, "the worksheet", func() bool { return h.HasKey(KeyCheck) })
	return h
}

// fixture copies the archive package's valid cassette to a temporary folder.
func fixture(t testing.TB) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "cassette")
	if err := os.CopyFS(dst, os.DirFS(testdata)); err != nil {
		t.Fatalf("copying the fixture: %v", err)
	}
	return dst
}

// install imports the cassette folder into the environment's store.
func install(t testing.TB, env *app.Env, path string) {
	t.Helper()
	res, err := env.Archive.Import(context.Background(), path, nil)
	if err != nil {
		t.Fatalf("installing %s: %v", path, err)
	}
	if res.Status != archive.ImportInstalled {
		t.Fatalf("installing %s: status %v, problems %v", path, res.Status, res.Problems)
	}
}

// setProgress changes the save file of mia.
func setProgress(t testing.TB, env *app.Env, change func(p *progress.Progress)) {
	t.Helper()
	err := env.Progress.Update(context.Background(), "mia", fractionsID, func(p *progress.Progress) error {
		change(p)
		return nil
	})
	if err != nil {
		t.Fatalf("setting the progress: %v", err)
	}
}

// load reads the save file of mia.
func (h *harness) load() *progress.Progress {
	h.t.Helper()
	p, err := h.Env.Progress.Load(context.Background(), "mia", fractionsID)
	if err != nil {
		h.t.Fatalf("loading the progress: %v", err)
	}
	return p
}

// key finds the widget with the key.
func key(k string) gluttertest.Finder { return gluttertest.ByKey(m.NewValueKey(k)) }

// has reports whether the text is on screen.
func (h *harness) has(text string) bool { return h.Exists(gluttertest.ByText(text)) }

// textOf returns the text of the Text widget with the key.
func (h *harness) textOf(k string) string {
	h.t.Helper()
	txt, ok := h.Widget(key(k)).(*m.Text)
	if !ok {
		h.t.Fatalf("%s is not a Text", k)
	}
	return txt.PlainText()
}

// waitLimit is a generous wait for the goroutines of the flow.
const waitLimit = 5 * time.Second

// whileBusy waits until cond is true, pumping single frames: a progress
// indicator animates forever, so the screen never settles.
func (h *harness) whileBusy(what string, cond func() bool) {
	h.t.Helper()
	deadline := time.Now().Add(waitLimit)
	for {
		h.FlushPosted()
		h.Pump(20 * time.Millisecond)
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// waitKey waits until the widget with the key is on screen.
func (h *harness) waitKey(k string) {
	h.t.Helper()
	h.whileBusy("the widget "+k, func() bool { return h.HasKey(k) })
}

// tap taps the widget with the key and pumps one frame.
func (h *harness) tap(k string) {
	h.t.Helper()
	h.Tap(key(k))
	h.Pump()
}

// check taps Check my answers and waits for the QR code.
func (h *harness) check() {
	h.t.Helper()
	h.tap(KeyCheck)
	h.waitKey(KeyQR)
}

// url is the address the QR dialog shows.
func (h *harness) url() string { return h.textOf(KeyURL) }

// photo returns a small JPEG photo.
func photo(t testing.TB) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for y := range 48 {
		for x := range 64 {
			img.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 5), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// upload posts one photo to the URL the way the phone page does and returns the
// status.
func upload(t testing.TB, url string) int {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", `form-data; name="photos"; filename="p1.jpg"`)
	hdr.Set("Content-Type", "image/jpeg")
	part, err := mw.CreatePart(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(photo(t)); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(url+"/photos", mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

// get fetches the address and returns the status.
func get(t testing.TB, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

// result is a grading result.
func result(id string, v grading.Verdict, feedback string) grading.ExerciseResult {
	return grading.ExerciseResult{
		ExerciseID: id, Verdict: v, Transcription: "read " + id, FeedbackForChild: feedback, NotesForParent: "notes " + id,
	}
}

// waitSubmission waits until the save file has a submission with the status.
func (h *harness) waitSubmission(status progress.SubmissionStatus) progress.Submission {
	h.t.Helper()
	deadline := time.Now().Add(waitLimit)
	for {
		p := h.load()
		for _, s := range p.Submissions {
			if s.Status == status {
				return s
			}
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("no submission became %s: %+v", status, p.Submissions)
		}
		h.FlushPosted()
		time.Sleep(5 * time.Millisecond)
	}
}

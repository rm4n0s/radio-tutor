package desktop_test

import (
	"bytes"
	"context"
	"encoding/json"
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

	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/glutter/ui"
	"github.com/rm4n0s/radio-tutor/internal/core/audio"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/grading/fake"
	"github.com/rm4n0s/radio-tutor/internal/core/phone"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/providers"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
	"github.com/rm4n0s/radio-tutor/internal/core/submission"
	"github.com/rm4n0s/radio-tutor/internal/gui/admin"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/desktop"
	"github.com/rm4n0s/radio-tutor/internal/gui/player"
	"github.com/rm4n0s/radio-tutor/internal/gui/student"
	"github.com/rm4n0s/radio-tutor/internal/gui/svgcheck"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
	"github.com/rm4n0s/radio-tutor/internal/gui/worksheet"
)

const (
	sampleCassette = "../../../testdata/cassettes/sample.cassette"
	sampleID       = "dev.radiotutor.sample.fractions"
	sampleTitle    = "Halves and quarters"
	lesson1        = "what-is-a-half"
	lesson2        = "quarters"
	exShade        = "shade-the-half"
	exWhich        = "which-is-a-half"

	parentPassword  = "parent-secret"
	studentPassword = "abcd"

	// tallWindow holds the longest screen of the walkthrough in one view.
	tallWindow = 1400.0
	// progressWindow holds the whole progress page of the cassette.
	progressWindow = 3200.0
	waitLimit      = 10 * time.Second
)

// submissionClock adapts the audio package's fake clock to the submission
// service; it is not the clock of the player, so playing a lesson does not run
// the timers of a submission.
type submissionClock struct{ *audio.FakeClock }

func (c submissionClock) AfterFunc(d time.Duration, f func()) submission.Timer {
	return c.FakeClock.AfterFunc(d, f)
}

// walk is the application pumped on the real screens of package desktop.
type walk struct {
	*testutil.App
	t         *testing.T
	audio     *audio.FakeClock
	provider  *fake.Provider
	mu        sync.Mutex
	sinks     []*audio.FakeSink
	choosings int
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

// pumpWalk starts the real application (desktop.Screens) on isolated storage
// with the test doubles of the specification: the file chooser answers with the
// sample cassette, the sound goes to a fake sink on a fake clock, and the
// phone server listens on 127.0.0.1 and the fake provider grades.
func pumpWalk(t *testing.T) *walk {
	t.Helper()
	sample, err := filepath.Abs(sampleCassette)
	if err != nil {
		t.Fatal(err)
	}
	w := &walk{t: t, audio: audio.NewFakeClock(time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC))}
	w.provider = fake.New("fake", fake.Reply(fake.JSONReply("A tidy page.",
		grading.ExerciseResult{ExerciseID: exShade, Verdict: grading.VerdictCorrect, Transcription: "a circle in two halves, one coloured", FeedbackForChild: "Two equal halves, well done!", NotesForParent: "Good."},
		grading.ExerciseResult{ExerciseID: exWhich, Verdict: grading.VerdictCorrect, Transcription: "A, 1/2", FeedbackForChild: "Yes, picture A!", NotesForParent: "Good."},
	)))
	port := freePort(t)
	w.App = testutil.Pump(t, testutil.Options{
		Size:    m.Size{Width: app.MinWindowWidth, Height: tallWindow},
		Screens: desktop.Screens(),
		Render:  svgcheck.Checker{},
		ChooseFile: func(context.Context, m.BuildContext, app.FileQuestion) (string, bool, error) {
			w.mu.Lock()
			w.choosings++
			w.mu.Unlock()
			return sample, true, nil
		},
		SinkFactory: func(context.Context) (audio.Sink, error) {
			w.mu.Lock()
			defer w.mu.Unlock()
			s := audio.NewFakeSink(w.audio)
			w.sinks = append(w.sinks, s)
			return s, nil
		},
		AudioClock: w.audio,
		Submission: app.SubmissionOptions{
			Phone: phone.New(phone.Options{ListenHost: "127.0.0.1"}),
			Clock: submissionClock{audio.NewFakeClock(time.Date(2026, 10, 30, 9, 30, 0, 0, time.UTC))},
			BuildProvider: func(providers.Config, *slog.Logger) (grading.Provider, bool, error) {
				return w.provider, false, nil
			},
			DetectLAN: func(context.Context, string) (phone.LANAddr, error) {
				return phone.LANAddr{IP: net.ParseIP("127.0.0.1"), Interface: "test0"}, nil
			},
		},
		Before: func(env *app.Env) {
			// What a parent sets up in the settings screens, which are not part
			// of this walkthrough: a free phone port and an active provider.
			ctx := context.Background()
			if err := env.Settings.Update(ctx, func(s *settings.Settings) { s.Phone.Port = port }); err != nil {
				t.Fatal(err)
			}
			cfg := providers.Config{
				Name: "Test checker", Protocol: "openai", BaseURL: "http://127.0.0.1:1/v1", Model: "fake",
				Kind: providers.KindLocal, StructuredOutput: grading.StructuredJSONSchema,
			}
			if err := env.Providers.Add(ctx, cfg); err != nil {
				t.Fatal(err)
			}
			if err := env.Providers.SetActive(ctx, cfg.Name); err != nil {
				t.Fatal(err)
			}
		},
	})
	return w
}

// key finds the widget with the key.
func key(k string) gluttertest.Finder { return gluttertest.ByKey(m.NewValueKey(k)) }

// until pumps single frames (a playing lesson and a spinner never settle) until
// cond holds, giving the goroutines of the screens real time.
func (w *walk) until(what string, cond func() bool) {
	w.t.Helper()
	deadline := time.Now().Add(waitLimit)
	for {
		w.FlushPosted()
		w.Pump(16 * time.Millisecond)
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			w.t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// waitKey waits until the widget with the key is on screen.
func (w *walk) waitKey(k string) {
	w.t.Helper()
	w.until("the widget "+k, func() bool { return w.HasKey(k) })
}

// waitText waits until the text is on screen.
func (w *walk) waitText(text string) {
	w.t.Helper()
	w.until("the text "+text, func() bool { return w.Exists(gluttertest.ByText(text)) })
}

// tap taps the widget with the key and pumps a frame.
func (w *walk) tap(k string) {
	w.t.Helper()
	w.Tap(key(k))
	w.Pump()
}

// enter types text into the field with the key.
func (w *walk) enter(k, text string) {
	w.t.Helper()
	w.EnterText(key(k), text)
	w.Pump()
}

// textOf returns the text of the Text widget with the key.
func (w *walk) textOf(k string) string {
	w.t.Helper()
	txt, ok := w.Widget(key(k)).(*m.Text)
	if !ok {
		w.t.Fatalf("%s is not a Text", k)
	}
	return txt.PlainText()
}

// back taps the back button of the app bar and waits for the transition to end.
func (w *walk) back() {
	w.t.Helper()
	w.Tap(gluttertest.ByType[*m.BackButton]())
	w.PumpAndSettle()
}

// logIn logs the person in from the login screen.
func (w *walk) logIn(username, password, home string) {
	w.t.Helper()
	w.waitKey(app.KeyProfilePrefix + username)
	w.tap(app.KeyProfilePrefix + username)
	w.enter(app.KeyPassword, password)
	w.tap(app.KeySubmit)
	w.waitKey(home)
}

// logOut logs out and waits for the login screen.
func (w *walk) logOut() {
	w.t.Helper()
	w.tap(app.KeyLogout)
	w.waitKey(app.KeyProfilePrefix + "mia")
}

// saved reads the save file of mia.
func (w *walk) saved() *progress.Progress {
	w.t.Helper()
	p, err := w.Env.Progress.Load(context.Background(), "mia", sampleID)
	if err != nil {
		w.t.Fatalf("loading the save file: %v", err)
	}
	return p
}

// jpegPhoto returns a small JPEG.
func jpegPhoto(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 90, 120))
	for y := range 120 {
		for x := range 90 {
			img.Set(x, y, color.RGBA{uint8(x * 2), uint8(y * 2), 100, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// postPhoto sends the photo to the address the way the phone page does and
// returns the status.
func postPhoto(t *testing.T, url string, photo []byte) int {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", `form-data; name="photos"; filename="page.jpg"`)
	hdr.Set("Content-Type", "image/jpeg")
	part, err := mw.CreatePart(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(photo); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url+"/photos?offer="+waitingOffer(t, url), &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

// TestWalkthrough drives the whole application on its real screens: the first
// run, a parent adds a student and imports the sample cassette, the student
// plays lesson 1 to the end, solves the worksheet with a photo that arrives over
// HTTP, sees the results and finds lesson 2 unlocked, and the parent reviews the
// submission and its photo (SPEC §16).
func TestWalkthrough(t *testing.T) {
	if _, err := os.Stat(sampleCassette); err != nil {
		t.Fatalf("the sample cassette is missing: %v", err)
	}
	w := pumpWalk(t)
	// The log of the application says what went wrong when a step times out.
	defer func() {
		if t.Failed() {
			log, err := os.ReadFile(w.Paths.LogFile())
			t.Logf("application log (%v):\n%s", err, log)
		}
	}()

	// First run: create the parent.
	w.waitKey(app.KeySubmit)
	w.enter(app.KeyUsername, "mum")
	w.enter(app.KeyDisplayName, "Mum")
	w.enter(app.KeyPassword, parentPassword)
	w.enter(app.KeyPasswordAgain, parentPassword)
	w.tap(app.KeySubmit)
	w.waitKey(admin.KeyStudents)

	// The parent adds a student.
	w.tap(admin.KeyStudents)
	w.waitKey(admin.KeyAddStudent)
	w.tap(admin.KeyAddStudent)
	w.waitKey(admin.KeyDialogOK)
	w.enter(app.KeyUsername, "mia")
	w.enter(app.KeyDisplayName, "Mia")
	w.enter(app.KeyPassword, studentPassword)
	w.enter(app.KeyPasswordAgain, studentPassword)
	w.tap(admin.KeyDialogOK)
	w.waitText("Mia")
	w.until("the dialog to close", func() bool { return !w.HasKey(admin.KeyDialogOK) })
	if _, err := w.Env.Profiles.Login("mia", studentPassword); err != nil {
		t.Fatalf("the new student cannot log in: %v", err)
	}
	w.PumpAndSettle()
	w.back()
	w.waitKey(admin.KeyCassettes)

	// The parent imports the sample cassette.
	w.tap(admin.KeyCassettes)
	w.waitKey(student.KeyImport)
	w.tap(student.KeyImport)
	w.waitText(sampleTitle)
	w.waitText("The cassette \"" + sampleTitle + "\" was added.")
	w.mu.Lock()
	choosings := w.choosings
	w.mu.Unlock()
	if choosings != 1 {
		t.Errorf("the file chooser was asked %d times, want 1", choosings)
	}
	if _, _, err := w.Env.Archive.Get(context.Background(), sampleID); err != nil {
		t.Fatalf("the sample is not installed: %v", err)
	}
	w.back()
	w.waitKey(admin.KeyStudents)

	// The student logs in and opens the cassette.
	w.logOut()
	w.logIn("mia", studentPassword, app.KeyLogout)
	w.waitKey(student.KeyCardPrefix + sampleID)
	w.tap(student.KeyCardPrefix + sampleID)
	w.waitKey(student.KeyContinue)
	if !w.HasKey(student.KeyItemPrefix + lesson2 + student.KeyStateSuffix) {
		t.Error("lesson 2 is not locked at the start")
	}
	if w.HasKey(student.KeyItemPrefix + lesson1 + student.KeyStateSuffix) {
		t.Error("lesson 1 is not available at the start")
	}

	// Lesson 1: play to the end on the fake sink.
	w.tap(student.KeyItemPrefix + lesson1)
	w.waitKey(player.KeyPlay)
	// Mia mutes the sound; it is saved for every profile and every lesson.
	w.tap(player.KeyMute)
	w.until("the mute to be saved", func() bool { return w.Env.Settings.Get().Muted })
	w.tap(player.KeyPlay)
	w.until("the sink to start", func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()
		return len(w.sinks) == 1 && len(w.sinks[0].Calls()) > 0
	})
	if got := w.textOf(player.KeyTime); !strings.HasPrefix(got, "0:") {
		t.Errorf("the time at the start is %q", got)
	}
	for range 600 {
		if w.HasKey(player.KeyNext) {
			break
		}
		w.audio.Advance(time.Second)
		w.FlushPosted()
		w.Pump(time.Second)
		time.Sleep(time.Millisecond)
	}
	w.waitKey(player.KeyNext)
	if lp := w.saved().Lessons[lesson1]; !lp.Completed() || lp.Plays != 1 {
		t.Fatalf("lesson 1 in the save file: %+v", lp)
	}
	w.tap(player.KeyNext)

	// The worksheet: select both exercises, check, upload.
	w.waitKey(worksheet.KeyCheck)
	w.PumpAndSettle()
	// Both exercises are ticked when the worksheet opens; unticking one and
	// ticking it again leaves both selected.
	w.waitText("2 selected")
	w.tap(worksheet.KeySelectPrefix + exWhich)
	w.waitText("1 selected")
	w.tap(worksheet.KeySelectPrefix + exWhich)
	w.waitText("2 selected")
	w.tap(worksheet.KeyCheck)
	w.waitKey(worksheet.KeyQR)
	url := w.textOf(worksheet.KeyURL)
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Fatalf("the address on the screen is %q", url)
	}
	if status := postPhoto(t, url, jpegPhoto(t)); status != http.StatusOK {
		t.Fatalf("the upload answered %d", status)
	}

	// The results.
	for _, id := range []string{exShade, exWhich} {
		w.waitKey(worksheet.KeyResultPrefix + id)
		w.waitKey(worksheet.KeyMarkPrefix + id)
		w.waitKey(worksheet.KeyStarPrefix + id)
	}
	w.waitText("Two equal halves, well done!")
	w.waitText("Yes, picture A!")
	if w.HasKey(worksheet.KeyReviewPrefix+exShade) || w.HasKey(worksheet.KeyFailure) {
		t.Error("the results show a problem")
	}
	if calls := w.provider.Calls(); len(calls) != 1 || len(calls[0].Request.Photos) != 1 || len(calls[0].Request.Exercises) != 2 {
		t.Errorf("the provider was called %d times", len(calls))
	}

	// Back to the worksheet, which shows both exercises as passed, and back to
	// the cassette: lesson 2 is unlocked.
	w.tap(worksheet.KeyBack)
	w.waitKey(worksheet.KeyCheck)
	w.PumpAndSettle()
	w.back()
	w.waitKey(student.KeyContinue)
	w.until("lesson 2 to unlock", func() bool { return !w.HasKey(student.KeyItemPrefix + lesson2 + student.KeyStateSuffix) })
	if !w.HasKey(student.KeyItemPrefix + lesson1 + student.KeyStateSuffix) {
		t.Error("lesson 1 is not shown as completed")
	}
	p := w.saved()
	if len(p.Submissions) != 1 || p.Submissions[0].Status != progress.SubmissionGraded {
		t.Fatalf("submissions: %+v", p.Submissions)
	}
	submissionID := p.Submissions[0].ID
	photo := filepath.Join(w.Paths.PhotosDir(), "mia", sampleID, submissionID, "1.jpg")
	if info, err := os.Stat(photo); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the stored photo: %v, %v", info, err)
	}
	for _, id := range []string{exShade, exWhich} {
		if p.Exercises[id].State != progress.StatePassed {
			t.Errorf("exercise %s: %+v", id, p.Exercises[id])
		}
	}

	// The parent reviews the submission and its photo. The progress pages are
	// long, so the window gets taller to keep every row on screen.
	w.SetViewSize(ui.Size{Width: app.MinWindowWidth, Height: progressWindow})
	w.logOut()
	w.logIn("mum", parentPassword, admin.KeyStudents)
	// The parent's Settings screen shows the mute that Mia set in the lesson.
	w.tap(admin.KeySettings)
	w.waitKey(admin.KeyMuteSwitch)
	w.PumpAndSettle()
	if !gluttertest.WidgetAs[*m.Switch](w.Tester, gluttertest.Descendant(key(admin.KeyMuteSwitch), gluttertest.ByType[*m.Switch]())).Value() {
		t.Error("the Settings screen does not show the sound as muted")
	}
	w.back()
	w.waitKey(admin.KeyStudents)
	w.tap(admin.KeyProgress)
	w.waitKey(admin.KeyProgressStudentPrefix + "mia")
	w.tap(admin.KeyProgressStudentPrefix + "mia")
	w.waitKey(admin.KeyProgressCassettePrefix + sampleID)
	w.tap(admin.KeyProgressCassettePrefix + sampleID)
	w.waitKey(admin.KeySubmissionPrefix + submissionID)
	w.tap(admin.KeySubmissionPrefix + submissionID)
	w.waitKey(admin.KeyPhotoPrefix + "1")
	for _, id := range []string{exShade, exWhich} {
		w.waitKey(admin.KeyResultPrefix + id)
		w.waitKey(admin.KeyVerdictPrefix + id)
	}
	if got := w.textOf(admin.KeyPageNotes); !strings.Contains(got, "A tidy page.") {
		t.Errorf("the page notes show %q", got)
	}
	w.tap(admin.KeyPhotoPrefix + "1")
	w.waitKey(admin.KeyPhotoViewer)
	if !w.HasKey(admin.KeyPhotoZoom) {
		t.Error("the photo viewer has no zoom")
	}
}

// waitingOffer asks the phone link which submission is waiting for photos, as
// the phone page does, and returns its number for the upload address.
func waitingOffer(t *testing.T, url string) string {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url+"/state", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var st struct {
		Offer uint64 `json:"offer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	return strconv.FormatUint(st.Offer, 10)
}

package e2e_test

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
	"testing"
	"time"

	"github.com/rm4n0s/radio-tutor/internal/core/archive"
	"github.com/rm4n0s/radio-tutor/internal/core/audio"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/grading/fake"
	"github.com/rm4n0s/radio-tutor/internal/core/phone"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/providers"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
	"github.com/rm4n0s/radio-tutor/internal/core/submission"
)

const (
	sampleCassette = "../../../testdata/cassettes/sample.cassette"
	sampleID       = "dev.radiotutor.sample.fractions"
	lesson1        = "what-is-a-half"
	lesson2        = "quarters"
	exShade        = "shade-the-half"
	exWhich        = "which-is-a-half"
	student        = "mia"
	eventWait      = 10 * time.Second
)

// activeProvider is the ProviderSource of the test: one provider, active.
type activeProvider struct{}

func (activeProvider) Active() (providers.Config, bool) {
	return providers.Config{Name: "Fake checker"}, true
}

// phoneSettings is the SettingsSource of the test: the system picks the port.
type phoneSettings struct{}

func (phoneSettings) Get() settings.Settings {
	s := settings.Defaults("en_US.UTF-8")
	s.Phone.Port = 0
	return s
}

// fakeClock adapts the audio package's fake clock to the service's Clock; the
// test never advances it, so the 60 s and 30 min timers do not fire.
type fakeClock struct{ *audio.FakeClock }

func (c fakeClock) AfterFunc(d time.Duration, f func()) submission.Timer {
	return c.FakeClock.AfterFunc(d, f)
}

// jpegPhoto returns a small JPEG.
func jpegPhoto(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 80, 60))
	for y := range 60 {
		for x := range 80 {
			img.Set(x, y, color.RGBA{uint8(x * 3), uint8(y * 4), 90, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// post sends the photo to the session's address as the phone page does and
// returns the status and the body.
func post(t *testing.T, url string, photo []byte) (int, string) {
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
	text, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(text)
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

// TestSampleCassetteEndToEnd is the test of SPEC §16: import, profile, lesson
// 1 played to the end, a submission graded from an uploaded photo, lesson 2
// unlocked.
func TestSampleCassetteEndToEnd(t *testing.T) {
	ctx := context.Background()
	paths := storagetest.Isolate(t)

	// Import the sample cassette into the store.
	store := archive.NewStore(paths, nil)
	imp, err := store.Import(ctx, sampleCassette, nil)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if imp.Status != archive.ImportInstalled || imp.ID != sampleID {
		t.Fatalf("Import = %v %q, problems %v", imp.Status, imp.ID, imp.Problems)
	}
	fsys, _, err := store.Get(ctx, sampleID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	c, problems, err := cassette.Load(ctx, fsys)
	if err != nil || c == nil {
		t.Fatalf("Load: %v, %v", c, err)
	}
	for _, p := range problems {
		if !p.Warning {
			t.Fatalf("the installed cassette has a problem: %+v", p)
		}
	}

	// A profile for the student.
	profiles, err := profile.Open(ctx, paths.ProfilesFile(), profile.WithIterations(1))
	if err != nil {
		t.Fatalf("profile.Open: %v", err)
	}
	if _, err := profiles.Create(ctx, student, "Mia", profile.RoleStudent, "abcd"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := profiles.Login(student, "abcd"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	prog := progress.NewStore(paths)

	states := func() map[string]progress.ItemState {
		t.Helper()
		p, err := prog.Load(ctx, student, sampleID)
		if err != nil {
			t.Fatalf("Load progress: %v", err)
		}
		return progress.States(c, p)
	}
	if got := states(); got[lesson1] != progress.StateAvailable || got[lesson2] != progress.StateLocked {
		t.Fatalf("at the start: lesson 1 is %q and lesson 2 is %q", got[lesson1], got[lesson2])
	}

	// Lesson 1: the real narration is decoded and played to its end on the
	// fake sink; the end completes the lesson.
	f, err := fsys.Open("lessons/" + lesson1 + "/narration.ogg")
	if err != nil {
		t.Fatal(err)
	}
	clip, err := audio.DecodeLimit(ctx, f, 601)
	_ = f.Close()
	if err != nil {
		t.Fatalf("DecodeLimit: %v", err)
	}
	clock := audio.NewFakeClock(time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC))
	sink := audio.NewFakeSink(clock)
	pl := audio.NewPlayer(sink, clock)
	defer pl.Close()
	if err := pl.Load(clip); err != nil {
		t.Fatalf("Load clip: %v", err)
	}
	if err := pl.Play(ctx); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if err := prog.Update(ctx, student, sampleID, func(p *progress.Progress) error {
		return progress.PlayLesson(c, p, lesson1)
	}); err != nil {
		t.Fatalf("PlayLesson: %v", err)
	}
	clock.Advance(time.Duration(clip.Seconds()/2*float64(time.Second)) + time.Millisecond)
	if st := pl.State(); st != audio.StatePlaying {
		t.Fatalf("halfway the player is %v, want playing", st)
	}
	if pos := pl.Position(); pos <= 0 || pos >= clip.Seconds() {
		t.Errorf("halfway the position is %v of %v", pos, clip.Seconds())
	}
	clock.Advance(time.Duration(clip.Seconds() * float64(time.Second)))
	select {
	case <-pl.Ended():
	case <-time.After(eventWait):
		t.Fatal("the player never reported the end")
	}
	if st := pl.State(); st != audio.StateEnded {
		t.Fatalf("at the end the player is %v, want ended", st)
	}
	if got, want := len(sink.Played()), len(clip.Samples); got != want {
		t.Errorf("the sink played %d samples of %d", got, want)
	}
	if err := prog.Update(ctx, student, sampleID, func(p *progress.Progress) error {
		return progress.CompleteLesson(c, p, lesson1, clock.Now())
	}); err != nil {
		t.Fatalf("CompleteLesson: %v", err)
	}
	got := states()
	if got[lesson1] != progress.StateCompleted || got[exShade] != progress.StateAvailable || got[exWhich] != progress.StateAvailable {
		t.Fatalf("after lesson 1: %v", got)
	}
	if got[lesson2] != progress.StateLocked {
		t.Fatalf("lesson 2 is %q before the worksheet is passed, want locked", got[lesson2])
	}

	// The submission service with a real phone server on 127.0.0.1 and the
	// fake provider, which says both exercises are correct.
	grader := fake.New("fake", fake.Reply(fake.JSONReply("A tidy page.",
		grading.ExerciseResult{ExerciseID: exShade, Verdict: grading.VerdictCorrect, Transcription: "a circle cut in two, one half coloured", FeedbackForChild: "Well done!", NotesForParent: "ok"},
		grading.ExerciseResult{ExerciseID: exWhich, Verdict: grading.VerdictCorrect, Transcription: "A, 1/2", FeedbackForChild: "Yes, A.", NotesForParent: "ok"},
	)))
	ph := phone.New(phone.Options{ListenHost: "127.0.0.1"})
	svcCtx, stop := context.WithCancel(ctx)
	svc := submission.New(svcCtx, submission.Options{
		Paths:     paths,
		Progress:  prog,
		Providers: activeProvider{},
		Settings:  phoneSettings{},
		Phone:     ph,
		Clock:     fakeClock{audio.NewFakeClock(time.Date(2026, 10, 30, 9, 5, 0, 0, time.UTC))},
		Log:       slog.New(slog.DiscardHandler),
		DetectLAN: func(context.Context, string) (phone.LANAddr, error) {
			return phone.LANAddr{IP: net.ParseIP("127.0.0.1"), Interface: "test0"}, nil
		},
		BuildProvider: func(providers.Config, *slog.Logger) (grading.Provider, bool, error) { return grader, false, nil },
	})
	defer func() {
		if err := svc.Close(ctx); err != nil {
			t.Errorf("Close: %v", err)
		}
		stop()
	}()
	sess, err := svc.Open(ctx, student, c, []string{exShade, exWhich})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !strings.HasPrefix(sess.URL, "http://127.0.0.1:") {
		t.Fatalf("the session URL is %q", sess.URL)
	}

	// The phone page opens and a photo is uploaded.
	resp, err := http.Get(sess.URL)
	if err != nil {
		t.Fatal(err)
	}
	page, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil || resp.StatusCode != http.StatusOK || !bytes.Contains(page, []byte("Halves and quarters")) {
		t.Fatalf("phone page: status %d, err %v", resp.StatusCode, err)
	}
	if status, body := post(t, sess.URL, jpegPhoto(t)); status != http.StatusOK {
		t.Fatalf("upload: status %d, body %q", status, body)
	}

	var graded submission.Graded
	deadline := time.After(eventWait)
wait:
	for {
		select {
		case ev, ok := <-sess.Events():
			if !ok {
				t.Fatal("the events ended before Graded")
			}
			switch e := ev.(type) {
			case submission.Graded:
				graded = e
				break wait
			case submission.Failed:
				t.Fatalf("grading failed: %v", e.Err)
			}
		case <-deadline:
			t.Fatal("never graded")
		}
	}
	if graded.Provider != "fake" || len(graded.Results) != 2 {
		t.Errorf("graded = %+v", graded)
	}
	if calls := grader.Calls(); len(calls) != 1 || len(calls[0].Request.Photos) != 1 || len(calls[0].Request.Exercises) != 2 {
		t.Errorf("the provider got %d calls", len(calls))
	}

	// Lesson 2 is available now; the save file has everything.
	got = states()
	if got[exShade] != progress.StatePassed || got[exWhich] != progress.StatePassed {
		t.Errorf("exercises after grading: %v", got)
	}
	if got[lesson2] != progress.StateAvailable {
		t.Errorf("lesson 2 is %q, want available", got[lesson2])
	}
	p, err := prog.Load(ctx, student, sampleID)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Lessons[lesson1].Completed() || p.Lessons[lesson1].Plays != 1 {
		t.Errorf("lesson 1 in the save file: %+v", p.Lessons[lesson1])
	}
	if len(p.Submissions) != 1 {
		t.Fatalf("submissions: %+v", p.Submissions)
	}
	sub := p.Submissions[0]
	if sub.ID != sess.ID || sub.Status != progress.SubmissionGraded || sub.Provider != "fake" || sub.PageNotes != "A tidy page." {
		t.Errorf("submission = %+v", sub)
	}
	if len(sub.Photos) != 1 || sub.Photos[0] != "1.jpg" || len(sub.Results) != 2 {
		t.Errorf("submission photos/results = %v / %+v", sub.Photos, sub.Results)
	}
	for _, r := range sub.Results {
		if r.Verdict != progress.Verdict(grading.VerdictCorrect) || !r.CountedAttempt {
			t.Errorf("result %+v", r)
		}
		if p.Exercises[r.ExerciseID].State != progress.StatePassed || p.Exercises[r.ExerciseID].Attempts != 1 {
			t.Errorf("exercise %s: %+v", r.ExerciseID, p.Exercises[r.ExerciseID])
		}
	}

	// Files: photos 0600 in 0700 folders, the save file 0600.
	dir := filepath.Join(paths.PhotosDir(), student, sampleID, sess.ID)
	wantMode(t, dir, 0o700)
	wantMode(t, filepath.Join(dir, "1.jpg"), 0o600)
	stored, err := os.ReadFile(filepath.Join(dir, "1.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if img, err := jpeg.Decode(bytes.NewReader(stored)); err != nil || img.Bounds().Dx() == 0 {
		t.Errorf("the stored photo is not a JPEG: %v", err)
	}
	save := filepath.Join(paths.ProgressDir(), student, sampleID+".json")
	wantMode(t, save, 0o600)
	raw, err := os.ReadFile(save)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"cassette_id": "` + sampleID + `"`, `"status": "graded"`, `"verdict": "correct"`, `"1.jpg"`, `"completed_at"`} {
		if !bytes.Contains(raw, []byte(want)) {
			t.Errorf("the save file lacks %s:\n%s", want, raw)
		}
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

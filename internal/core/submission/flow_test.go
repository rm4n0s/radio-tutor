package submission

import (
	"context"
	"log/slog"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/grading/fake"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/providers"
)

func TestFullFlow(t *testing.T) {
	e := newEnv(t)
	prov := fake.New("Fake provider", fake.Step{
		Text: reply(res("e1", grading.VerdictCorrect), res("e2", grading.VerdictPartial)),
		Meta: grading.Meta{Model: "gpt-fake", FinishReason: "stop"},
	})
	e.use(prov)

	s := e.open("ann", "e1", "e2")
	if !contains(s.ID, "20261030T171955-") || len(s.ID) != len("20261030T171955-3f9a") {
		t.Errorf("submission id %q", s.ID)
	}
	if s.Username != "ann" || s.CassetteID != "org.example.sub-1" || !slices.Equal(s.Exercises, []string{"e1", "e2"}) {
		t.Errorf("session %+v", s)
	}
	if len(s.Token) != 26 || !strings.HasPrefix(s.URL, "http://127.0.0.1:") || !strings.HasSuffix(s.URL, "/p/"+s.Token) {
		t.Errorf("token %q, url %q", s.Token, s.URL)
	}
	if want := e.clock.Now().Add(30 * time.Minute); !s.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %v, want %v", s.ExpiresAt, want)
	}

	// The submission is stored as soon as it is open.
	sub := e.submission(s)
	if sub.Status != progress.SubmissionWaitingPhotos || !slices.Equal(sub.ExerciseIDs, []string{"e1", "e2"}) ||
		len(sub.Photos) != 0 || !sub.CreatedAt.Equal(time.Date(2026, 10, 30, 17, 19, 55, 0, time.UTC)) {
		t.Errorf("waiting record %+v", sub)
	}

	// The page shows the cassette and the chosen exercises only.
	status, page := get(t, s.URL)
	if status != 200 || !contains(page, "Fractions for all", "Title of e1", "Title of e2", "Prompt of e2") || contains(page, "Title of e3") {
		t.Errorf("page: status %d\n%s", status, page)
	}
	expect[PageOpened](t, s)
	get(t, s.URL) // a second opening is not announced again

	p1, p2 := photo(t, 80, 60), photo(t, 60, 80)
	if status, body := upload(t, s, p1, p2); status != 200 {
		t.Fatalf("upload: %d %s", status, body)
	}
	expect[UploadStarted](t, s)
	if got := expect[PhotosReceived](t, s); got.Count != 2 {
		t.Errorf("PhotosReceived = %+v", got)
	}
	expect[Grading](t, s)
	graded := expect[Graded](t, s)
	expectClosed(t, s)

	if graded.Provider != "Fake provider" || graded.Model != "gpt-fake" || graded.PageNotes != "the page notes" || graded.Downgraded != "" {
		t.Errorf("Graded = %+v", graded)
	}
	if len(graded.Results) != 2 || graded.Results[0].ExerciseID != "e1" || !graded.Results[0].CountedAttempt ||
		graded.Results[1].Verdict != progress.VerdictPartial || graded.Results[0].FeedbackForChild != "feedback e1" {
		t.Errorf("results %+v", graded.Results)
	}
	if !slices.Equal(graded.Changes.NewlyPassed, []string{"e1"}) || !slices.Equal(graded.Changes.HintsUnlocked, []string{"e2"}) ||
		len(graded.Changes.NeedsReview) != 0 {
		t.Errorf("changes %+v", graded.Changes)
	}

	// The provider got the request of the chosen exercises and the stored photos.
	calls := prov.Calls()
	if len(calls) != 1 {
		t.Fatalf("%d provider calls", len(calls))
	}
	req := calls[0].Request
	if !slices.Equal(req.ExerciseIDs(), []string{"e1", "e2"}) || req.Title != "Fractions for all" || len(req.Photos) != 2 {
		t.Errorf("request %+v", req)
	}
	if len(e.built) != 1 || e.built[0].Name != "Test provider" {
		t.Errorf("provider built from %+v", e.built)
	}

	// The save file.
	p := e.load("ann")
	got := p.Submission(s.ID)
	if got.Status != progress.SubmissionGraded || got.Provider != "Fake provider" || got.Model != "gpt-fake" ||
		got.PageNotes != "the page notes" || got.Error != "" || !slices.Equal(got.Photos, []string{"1.jpg", "2.jpg"}) ||
		len(got.Results) != 2 || !got.Results[0].CountedAttempt {
		t.Errorf("graded record %+v", got)
	}
	e1, e2, e3 := p.Exercises["e1"], p.Exercises["e2"], p.Exercises["e3"]
	if e1.State != progress.StatePassed || e1.Attempts != 1 || e1.PassedBy != progress.PassedByAI || e1.LastVerdict != progress.VerdictCorrect {
		t.Errorf("e1 = %+v", e1)
	}
	if e2.State != "" || e2.Attempts != 1 || e2.HintsShown != 1 || e2.LastVerdict != progress.VerdictPartial {
		t.Errorf("e2 = %+v", e2)
	}
	if e3 != (progress.ExerciseProgress{}) {
		t.Errorf("e3, not in the submission, changed: %+v", e3)
	}

	// The photos on disk: the files the provider saw, with the modes of SPEC §4.
	dir := e.photoPath(s)
	for i, name := range []string{"1.jpg", "2.jpg"} {
		wantMode(t, dir+"/"+name, 0o600)
		if data := mustRead(t, dir+"/"+name); string(data) != string(req.Photos[i]) {
			t.Errorf("%s differs from the photo the provider got", name)
		}
	}
	for _, d := range []string{dir, e.paths.PhotosDir() + "/ann/org.example.sub-1", e.paths.PhotosDir() + "/ann"} {
		wantMode(t, d, 0o700)
	}

	// The offer is used up, and the finished session has nothing to cancel or retry.
	if status, _ := upload(t, s, p1); status != 409 {
		t.Errorf("second upload: %d, want 409 (stale)", status)
	}
	if err := e.svc.Cancel(context.Background(), s.ID); err != nil {
		t.Errorf("Cancel of a graded session: %v", err)
	}
	wantRoute(t, e.svc.Retry(context.Background(), s.ID), "Service.Retry.SubmissionSessionUnknown")
}

func TestFailureThenRetry(t *testing.T) {
	e := newEnv(t)
	boom := errors.New("OpenAIAuth", "the provider refused the key")
	prov := fake.New("Fake provider",
		fake.Fail(boom),
		fake.Reply(reply(res("e1", grading.VerdictIncorrect), res("e2", grading.VerdictCorrect))),
	)
	e.use(prov)
	s := e.open("ann", "e1", "e2")

	if status, body := upload(t, s, photo(t, 50, 50)); status != 200 {
		t.Fatalf("upload: %d %s", status, body)
	}
	expect[UploadStarted](t, s)
	expect[PhotosReceived](t, s)
	expect[Grading](t, s)
	failed := expect[Failed](t, s)
	if !failed.CanRetry {
		t.Error("a failed grading cannot be retried")
	}
	if appErr, ok := errors.FromError(failed.Err); !ok || appErr.Tag != "OpenAIAuth" {
		t.Errorf("Failed.Err = %v", failed.Err)
	}

	// The failure is stored; no attempt was used.
	sub := e.submission(s)
	if sub.Status != progress.SubmissionFailed || sub.Error != "the provider refused the key" || sub.Provider != "Test provider" ||
		!slices.Equal(sub.Photos, []string{"1.jpg"}) {
		t.Errorf("failed record %+v", sub)
	}
	if p := e.load("ann"); len(p.Exercises) != 0 {
		t.Errorf("a failure changed the exercises: %+v", p.Exercises)
	}

	// A failed session does not expire.
	e.clock.Advance(2 * time.Hour)
	expectQuiet(t, s)

	if err := e.svc.Retry(context.Background(), s.ID); err != nil {
		t.Fatal(err)
	}
	expect[Grading](t, s)
	graded := expect[Graded](t, s)
	expectClosed(t, s)
	if !slices.Equal(graded.Changes.NewlyPassed, []string{"e2"}) {
		t.Errorf("changes %+v", graded.Changes)
	}

	// Both calls graded the same stored photo, the second one as a retry of the whole request.
	calls := prov.Calls()
	if len(calls) != 2 || string(calls[0].Request.Photos[0]) != string(calls[1].Request.Photos[0]) {
		t.Errorf("calls %d", len(calls))
	}
	p := e.load("ann")
	if p.Exercises["e1"].Attempts != 1 || p.Exercises["e1"].HintsShown != 1 || p.Exercises["e2"].State != progress.StatePassed {
		t.Errorf("exercises %+v", p.Exercises)
	}
	if sub := p.Submission(s.ID); sub.Status != progress.SubmissionGraded || sub.Error != "" || len(p.Submissions) != 1 {
		t.Errorf("record %+v", sub)
	}
}

func TestNoActiveProvider(t *testing.T) {
	e := newEnv(t)
	e.providers.set(false)
	e.use(fake.New("Fake provider", fake.Reply(reply(res("e1", grading.VerdictCorrect)))))
	s := e.open("ann", "e1")
	upload(t, s, photo(t, 40, 40))
	expect[UploadStarted](t, s)
	expect[PhotosReceived](t, s)
	expect[Grading](t, s)
	failed := expect[Failed](t, s)
	if !NoProvider(failed.Err) || !failed.CanRetry {
		t.Errorf("Failed = %+v", failed)
	}
	wantRoute(t, failed.Err, "Session.grade.SubmissionNoProvider")
	if NoProvider(errors.New("Other", "x")) || NoProvider(context.Canceled) {
		t.Error("NoProvider is true for another error")
	}
	if sub := e.submission(s); sub.Status != progress.SubmissionFailed || !strings.Contains(sub.Error, "no grading provider") {
		t.Errorf("record %+v", sub)
	}

	// A parent sets the checker up; the same photos are graded.
	e.providers.set(true)
	if err := e.svc.Retry(context.Background(), s.ID); err != nil {
		t.Fatal(err)
	}
	expect[Grading](t, s)
	expect[Graded](t, s)
	if p := e.load("ann"); p.Exercises["e1"].State != progress.StatePassed {
		t.Errorf("e1 = %+v", p.Exercises["e1"])
	}
}

func TestProviderBuildFailure(t *testing.T) {
	e := newEnv(t)
	s := e.open("ann", "e1")
	e.svc.opts.BuildProvider = func(providers.Config, *slog.Logger) (grading.Provider, bool, error) {
		return nil, false, errors.New("OpenAIConfigModel", "no model")
	}
	upload(t, s, photo(t, 40, 40))
	expect[UploadStarted](t, s)
	expect[PhotosReceived](t, s)
	expect[Grading](t, s)
	failed := expect[Failed](t, s)
	wantRoute(t, failed.Err, "OpenAIConfigModel")
	if sub := e.submission(s); sub.Provider != "Test provider" || sub.Status != progress.SubmissionFailed {
		t.Errorf("record %+v", sub)
	}
}

func TestStoredPhotoMissingFailsTheRetry(t *testing.T) {
	e := newEnv(t)
	e.use(fake.New("Fake provider", fake.Fail(errors.New("OpenAITimeout", "slow"))))
	s := e.open("ann", "e1")
	upload(t, s, photo(t, 40, 40))
	expect[UploadStarted](t, s)
	expect[PhotosReceived](t, s)
	expect[Grading](t, s)
	expect[Failed](t, s)
	if err := os.Remove(e.photoPath(s) + "/1.jpg"); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Retry(context.Background(), s.ID); err != nil {
		t.Fatal(err)
	}
	expect[Grading](t, s)
	failed := expect[Failed](t, s)
	wantRoute(t, failed.Err, "loadPhotos.SubmissionPhotoReadFailed")
}

func TestResultsOutsideTheSessionAreIgnored(t *testing.T) {
	e := newEnv(t)
	// The model also answers for e3, which was not asked, and for an id the
	// cassette does not have.
	e.use(fake.New("Fake provider", fake.Reply(reply(
		res("e1", grading.VerdictCorrect), res("e3", grading.VerdictCorrect), res("nope", grading.VerdictIncorrect),
	))))
	s := e.open("ann", "e1")
	upload(t, s, photo(t, 40, 40))
	graded := untilGraded(t, s)
	if len(graded.Results) != 1 || graded.Results[0].ExerciseID != "e1" {
		t.Errorf("results %+v", graded.Results)
	}
	p := e.load("ann")
	if _, ok := p.Exercises["e3"]; ok || len(p.Exercises) != 1 || len(p.Submission(s.ID).Results) != 1 {
		t.Errorf("exercises %+v, record %+v", p.Exercises, p.Submission(s.ID))
	}
}

func TestDowngradeIsReported(t *testing.T) {
	e := newEnv(t)
	e.use(fake.New("Fake provider", fake.Step{
		Text: reply(res("e1", grading.VerdictCorrect)),
		Meta: grading.Meta{Model: "m", FinishReason: "stop", Downgraded: grading.StructuredJSONObject},
	}))
	s := e.open("ann", "e1")
	upload(t, s, photo(t, 40, 40))
	if graded := untilGraded(t, s); graded.Downgraded != grading.StructuredJSONObject {
		t.Errorf("Downgraded = %q", graded.Downgraded)
	}
}

func TestAttemptsRunOutAndHintsUnlock(t *testing.T) {
	// e4 has one counted attempt left to give; incorrect results exhaust e2 (2 attempts).
	e := newEnv(t)
	e.use(fake.New("Fake provider",
		fake.Reply(reply(res("e2", grading.VerdictIncorrect))),
		fake.Reply(reply(res("e2", grading.VerdictUnreadable))),
		fake.Reply(reply(res("e2", grading.VerdictPartial))),
	))
	for i, want := range []struct {
		attempts, hints int
		state           progress.ItemState
		needsReview     bool
	}{{1, 1, "", false}, {1, 1, "", false}, {2, 1, progress.StateNeedsReview, true}} {
		s := e.open("ann", "e2")
		upload(t, s, photo(t, 40, 40))
		graded := untilGraded(t, s)
		ep := e.load("ann").Exercises["e2"]
		if ep.Attempts != want.attempts || ep.HintsShown != want.hints || ep.State != want.state ||
			(len(graded.Changes.NeedsReview) == 1) != want.needsReview {
			t.Errorf("submission %d: %+v, changes %+v", i+1, ep, graded.Changes)
		}
	}
	// An exercise waiting for a parent cannot be submitted any more.
	_, err := e.svc.Open(context.Background(), "ann", e.cass, []string{"e2"})
	wantRoute(t, err, "checkAvailable.SubmissionExerciseUnavailable")
}

// untilGraded reads events until Graded and returns it; the channel then closes.
func untilGraded(t *testing.T, s *Session) Graded {
	t.Helper()
	for {
		switch ev := next(t, s).(type) {
		case Graded:
			expectClosed(t, s)
			return ev
		case Failed:
			t.Fatalf("grading failed: %v", ev.Err)
		}
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

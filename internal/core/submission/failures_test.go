package submission

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/grading/fake"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
)

// lockDir makes the folder read-only until the returned function runs.
func lockDir(t *testing.T, dir string) (unlock func()) {
	t.Helper()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Error(err)
		}
	}
}

func TestPhotosThatCannotBeStoredFailWithoutRetry(t *testing.T) {
	skipIfRoot(t)
	e := newEnv(t)
	s := e.open("ann", "e1")
	if err := os.MkdirAll(e.paths.PhotosDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	unlock := lockDir(t, e.paths.PhotosDir())
	defer unlock()

	// The phone is told the link is gone: the photos were not kept.
	if status, _ := upload(t, s.URL, photo(t, 40, 40)); status != 410 {
		t.Errorf("upload status %d", status)
	}
	expect[UploadStarted](t, s)
	failed := expect[Failed](t, s)
	if failed.CanRetry {
		t.Error("a session without photos can be retried")
	}
	wantRoute(t, failed.Err, "EnsureDir")
	if sub := e.submission(s); sub.Status != progress.SubmissionFailed || len(sub.Photos) != 0 || sub.Error == "" {
		t.Errorf("record %+v", sub)
	}
	wantRoute(t, e.svc.Retry(context.Background(), s.ID), "Session.retry.SubmissionNoPhotos")
	if err := e.svc.Cancel(context.Background(), s.ID); err != nil {
		t.Fatal(err)
	}
	expect[Cancelled](t, s)
}

func TestWritingAPhotoFails(t *testing.T) {
	skipIfRoot(t)
	e := newEnv(t)
	s := e.open("ann", "e1")
	dir := e.photoPath(s)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	// A file already in the way: the photo is not written over it.
	if err := os.WriteFile(filepath.Join(dir, "1.jpg"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if status, _ := upload(t, s.URL, photo(t, 40, 40)); status != 410 {
		t.Errorf("upload status %d", status)
	}
	expect[UploadStarted](t, s)
	failed := expect[Failed](t, s)
	wantRoute(t, failed.Err, "writePhoto.SubmissionPhotoCreateFailed")
	// What was there is not removed by mistake? The folder is removed with the failed write.
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("the half-written folder is still there: %v", err)
	}
}

func TestWritePhotoFailsOnWrite(t *testing.T) {
	// /dev/full accepts the open and refuses every write.
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skip("no /dev/full")
	}
	f, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
	if err != nil {
		t.Skip(err)
	}
	wantRoute(t, writeAndClose(f, "/dev/full", []byte("data")), "writeAndClose.SubmissionPhotoWriteFailed")
}

func TestSaveFailureAtReceiveCanBeRetried(t *testing.T) {
	skipIfRoot(t)
	e := newEnv(t)
	e.use(fake.New("Fake provider", fake.Reply(reply(res("e1", grading.VerdictCorrect)))))
	s := e.open("ann", "e1")
	unlock := lockDir(t, filepath.Join(e.paths.ProgressDir(), "ann"))
	defer unlock()

	// The photos arrived, so the phone sees "Sent!"; the save file could not be written.
	if status, body := upload(t, s.URL, photo(t, 40, 40)); status != 200 {
		t.Fatalf("upload: %d %s", status, body)
	}
	expect[UploadStarted](t, s)
	expect[PhotosReceived](t, s)
	expect[Grading](t, s)
	failed := expect[Failed](t, s)
	if !failed.CanRetry {
		t.Error("the stored photos cannot be graded again")
	}
	if _, err := os.Stat(e.photoPath(s) + "/1.jpg"); err != nil {
		t.Error(err)
	}

	// Retry fails while the disk is unwritable, and the session stays failed.
	if err := e.svc.Retry(context.Background(), s.ID); err == nil {
		t.Fatal("Retry succeeded on a read-only folder")
	}
	unlock()
	if err := e.svc.Retry(context.Background(), s.ID); err != nil {
		t.Fatal(err)
	}
	expect[Grading](t, s)
	untilGraded(t, s)
	if sub := e.submission(s); sub.Status != progress.SubmissionGraded || !slices.Equal(sub.Photos, []string{"1.jpg"}) {
		t.Errorf("record %+v", sub)
	}
}

func TestApplyFailureKeepsTheAnswerForRetry(t *testing.T) {
	skipIfRoot(t)
	e := newEnv(t)
	dir := filepath.Join(e.paths.ProgressDir(), "ann")
	var unlock func()
	prov := fake.New("Fake provider", fake.Step{
		Text:   reply(res("e1", grading.VerdictCorrect)),
		Meta:   grading.Meta{Model: "m", FinishReason: "stop"},
		Before: func() { unlock = lockDir(t, dir) }, // the disk fails while the model thinks
	})
	e.use(prov)
	s := e.open("ann", "e1")
	upload(t, s.URL, photo(t, 40, 40))
	expect[UploadStarted](t, s)
	expect[PhotosReceived](t, s)
	expect[Grading](t, s)
	failed := expect[Failed](t, s)
	if !failed.CanRetry {
		t.Error("cannot retry")
	}
	if ep := e.loadAfter(unlock, "ann").Exercises["e1"]; ep.Attempts != 0 {
		t.Errorf("results were applied: %+v", ep)
	}

	// Retry applies the answer that was already paid for: the provider is asked once only.
	if err := e.svc.Retry(context.Background(), s.ID); err != nil {
		t.Fatal(err)
	}
	expect[Grading](t, s)
	graded := untilGraded(t, s)
	if !slices.Equal(graded.Changes.NewlyPassed, []string{"e1"}) {
		t.Errorf("changes %+v", graded.Changes)
	}
	if n := len(prov.Calls()); n != 1 {
		t.Errorf("%d provider calls", n)
	}
	if ep := e.load("ann").Exercises["e1"]; ep.Attempts != 1 || ep.State != progress.StatePassed {
		t.Errorf("e1 = %+v", ep)
	}
}

// loadAfter runs unlock and reads the save file.
func (e *env) loadAfter(unlock func(), user string) *progress.Progress {
	e.t.Helper()
	unlock()
	return e.load(user)
}

func TestSaveFileThatBrokeMeanwhileIsSetAside(t *testing.T) {
	e := newEnv(t)
	e.use(fake.New("Fake provider", fake.Reply(reply(res("e1", grading.VerdictCorrect)))))
	s := e.open("ann", "e1")
	file := filepath.Join(e.paths.ProgressDir(), "ann", "org.example.sub-1.json")
	if err := os.WriteFile(file, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	upload(t, s.URL, photo(t, 40, 40))
	untilGraded(t, s)
	matches, err := filepath.Glob(file + ".broken-*")
	if err != nil || len(matches) != 1 {
		t.Errorf("set-aside files %v, %v", matches, err)
	}
	// The graded submission is in the fresh file.
	p := e.load("ann")
	if sub := p.Submission(s.ID); sub == nil || sub.Status != progress.SubmissionGraded || p.Exercises["e1"].State != progress.StatePassed {
		t.Errorf("fresh file %+v", p)
	}
}

func TestReceivedWhenNotWaiting(t *testing.T) {
	e := newEnv(t)
	s := e.open("ann", "e1")
	if err := e.svc.Cancel(context.Background(), s.ID); err != nil {
		t.Fatal(err)
	}
	wantRoute(t, s.received(context.Background(), [][]byte{photo(t, 30, 30)}), "Session.received.SubmissionNotWaiting")
}

func TestCancelWhileThePhotosArrive(t *testing.T) {
	e := newEnv(t)
	s := e.open("ann", "e1")
	e.svc.onReceiving = func() { s.end(context.Background(), []state{stReceiving}, Cancelled{}, "") }
	err := s.received(context.Background(), [][]byte{photo(t, 30, 30)})
	wantRoute(t, err, "Session.received.SubmissionCancelled")
	expect[Cancelled](t, s)
	expectClosed(t, s)
	if _, err := os.Stat(e.photoPath(s)); !os.IsNotExist(err) {
		t.Errorf("the photos of a cancelled submission were kept: %v", err)
	}
	if sub := e.submission(s); sub.Status != progress.SubmissionCancelled || len(sub.Photos) != 0 {
		t.Errorf("record %+v", sub)
	}
}

func TestCancelWhileThePhotosFailToStore(t *testing.T) {
	skipIfRoot(t)
	e := newEnv(t)
	s := e.open("ann", "e1")
	if err := os.MkdirAll(e.paths.PhotosDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	unlock := lockDir(t, e.paths.PhotosDir())
	defer unlock()
	e.svc.onReceiving = func() { s.end(context.Background(), []state{stReceiving}, Cancelled{}, "") }
	err := s.received(context.Background(), [][]byte{photo(t, 30, 30)})
	wantRoute(t, err, "EnsureDir")
	expect[Cancelled](t, s) // the failure did not overwrite the cancellation
	expectClosed(t, s)
	if sub := e.submission(s); sub.Status != progress.SubmissionCancelled {
		t.Errorf("record %+v", sub)
	}
}

func TestEventsNeverBlockAndKeepTheLastOnes(t *testing.T) {
	e := newEnv(t)
	s := e.open("ann", "e1")
	s.mu.Lock()
	for range 3 * EventBuffer {
		s.emit(UploadStarted{})
	}
	s.mu.Unlock()
	if err := e.svc.Cancel(context.Background(), s.ID); err != nil {
		t.Fatal(err)
	}
	var got []Event
	for ev := range s.Events() {
		got = append(got, ev)
	}
	if len(got) != EventBuffer {
		t.Fatalf("%d events kept, want %d", len(got), EventBuffer)
	}
	if _, ok := got[len(got)-1].(Cancelled); !ok {
		t.Errorf("the last event is %#v, want Cancelled", got[len(got)-1])
	}
	// Events after the end are dropped quietly.
	s.mu.Lock()
	s.emit(UploadStarted{})
	s.mu.Unlock()
}

func TestRetryAfterANonFailureAndSaveErrorsAreTagged(t *testing.T) {
	// A grading error carried through Failed keeps its own tag and route.
	e := newEnv(t)
	boom := errors.New("OpenAIRateLimited", "slow down", "retry_after_seconds", 30)
	e.use(fake.New("Fake provider", fake.Fail(boom)))
	s := e.open("ann", "e1")
	upload(t, s.URL, photo(t, 40, 40))
	for range 3 {
		next(t, s)
	}
	failed := expect[Failed](t, s)
	appErr, ok := errors.FromError(failed.Err)
	if !ok || appErr.Tag != "OpenAIRateLimited" || appErr.Metadata["retry_after_seconds"] != 30 {
		t.Errorf("Failed.Err = %#v", failed.Err)
	}
}

package submission

import (
	"context"
	"net"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/grading/fake"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
)

type settingsT = settings.Settings

// freePort returns a port nobody listens on right now.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestCancelWhileWaiting(t *testing.T) {
	e := newEnv(t)
	s := e.open("ann", "e1", "e3")
	if err := e.svc.Cancel(context.Background(), s.ID); err != nil {
		t.Fatal(err)
	}
	expect[Cancelled](t, s)
	expectClosed(t, s)
	if sub := e.submission(s); sub.Status != progress.SubmissionCancelled || sub.Error != "" {
		t.Errorf("record %+v", sub)
	}
	// The phone cannot send any more: its page goes back to waiting, and the
	// link stays for the next submission.
	if status, body := get(t, s.URL+"/state"); status != 200 || !strings.Contains(body, `"state":"waiting"`) {
		t.Errorf("state after the cancel: %d %s", status, body)
	}
	if status, _ := upload(t, s, photo(t, 30, 30)); status != 409 {
		t.Errorf("upload to a cancelled submission: %d", status)
	}
	// Cancelling again, or an unknown id, does nothing.
	for _, id := range []string{s.ID, "nope"} {
		if err := e.svc.Cancel(context.Background(), id); err != nil {
			t.Errorf("Cancel(%s) = %v", id, err)
		}
	}
	wantRoute(t, e.svc.Retry(context.Background(), s.ID), "Service.Retry.SubmissionSessionUnknown")
}

func TestCancelWhileGrading(t *testing.T) {
	e := newEnv(t)
	inCall, release := make(chan struct{}), make(chan struct{})
	e.use(fake.New("Fake provider", fake.Step{
		Text:   reply(res("e1", grading.VerdictCorrect)),
		Meta:   grading.Meta{Model: "m", FinishReason: "stop"},
		Before: func() { close(inCall); <-release },
	}))
	s := e.open("ann", "e1")
	upload(t, s, photo(t, 40, 40))
	expect[UploadStarted](t, s)
	expect[PhotosReceived](t, s)
	expect[Grading](t, s)
	<-inCall

	if err := e.svc.Cancel(context.Background(), s.ID); err != nil {
		t.Fatal(err)
	}
	expect[Cancelled](t, s)
	expectClosed(t, s)
	close(release) // the answer arrives too late and is dropped

	// A new Open for the student finishes only after the old goroutine could have applied anything.
	s2 := e.open("ann", "e1")
	if err := e.svc.Cancel(context.Background(), s2.ID); err != nil {
		t.Fatal(err)
	}
	p := e.load("ann")
	if ep := p.Exercises["e1"]; ep != (progress.ExerciseProgress{}) {
		t.Errorf("a cancelled grading changed e1: %+v", ep)
	}
	sub := p.Submission(s.ID)
	if sub.Status != progress.SubmissionCancelled || !slices.Equal(sub.Photos, []string{"1.jpg"}) || len(sub.Results) != 0 {
		t.Errorf("record %+v", sub)
	}
	// The photos stay for the parent.
	if _, err := os.Stat(e.photoPath(s) + "/1.jpg"); err != nil {
		t.Error(err)
	}
}

func TestCancelAFailedSession(t *testing.T) {
	e := newEnv(t)
	e.use(fake.New("Fake provider", fake.Fail(errors.New("OpenAITimeout", "slow"))))
	s := e.open("ann", "e1")
	upload(t, s, photo(t, 40, 40))
	for range 3 {
		next(t, s)
	}
	expect[Failed](t, s)
	if err := e.svc.Cancel(context.Background(), s.ID); err != nil {
		t.Fatal(err)
	}
	expect[Cancelled](t, s)
	expectClosed(t, s)
	if sub := e.submission(s); sub.Status != progress.SubmissionCancelled {
		t.Errorf("record %+v", sub)
	}
}

func TestExpiry(t *testing.T) {
	e := newEnv(t)
	s := e.open("ann", "e1")
	e.clock.Advance(30*time.Minute - time.Second)
	expect[FirewallHint](t, s) // nobody opened the page
	expectQuiet(t, s)
	if status, _ := get(t, s.URL); status != 200 {
		t.Errorf("the page just before expiry: %d", status)
	}
	expect[PageOpened](t, s)
	e.clock.Advance(time.Second)
	expect[Expired](t, s)
	expectClosed(t, s)
	sub := e.submission(s)
	if sub.Status != progress.SubmissionCancelled || sub.Error == "" {
		t.Errorf("record %+v", sub)
	}
	if status, _ := upload(t, s, photo(t, 30, 30)); status != 409 {
		t.Errorf("upload after expiry: %d", status)
	}
	if err := e.svc.Cancel(context.Background(), s.ID); err != nil {
		t.Error(err)
	}
}

func TestExpiryStopsWhenPhotosArrive(t *testing.T) {
	e := newEnv(t)
	e.use(fake.New("Fake provider", fake.Reply(reply(res("e1", grading.VerdictCorrect)))))
	s := e.open("ann", "e1")
	upload(t, s, photo(t, 40, 40))
	untilGraded(t, s)
	if n := e.clock.Pending(); n != 0 {
		t.Errorf("%d timers still pending", n)
	}
}

func TestSecondOpenCancelsTheFirst(t *testing.T) {
	e := newEnv(t)
	first := e.open("ann", "e1")
	second := e.open("ann", "e2")

	expect[Cancelled](t, first)
	expectClosed(t, first)
	if sub := e.submission(first); sub.Status != progress.SubmissionCancelled {
		t.Errorf("first record %+v", sub)
	}
	// The same link now shows the second submission.
	if first.URL != second.URL || first.Token != second.Token {
		t.Errorf("the links differ: %q and %q", first.URL, second.URL)
	}
	if status, body := get(t, second.URL+"/state"); status != 200 || !strings.Contains(body, `"state":"ready"`) || !strings.Contains(body, "Title of e2") || strings.Contains(body, "Title of e1") {
		t.Errorf("the page: %d %s", status, body)
	}
	expect[PageOpened](t, second)
	expectQuiet(t, second)
	if sub := e.submission(second); sub.Status != progress.SubmissionWaitingPhotos {
		t.Errorf("second record %+v", sub)
	}
	if first.ID == second.ID {
		t.Error("two submissions share an id")
	}
	if status, _ := upload(t, first, photo(t, 30, 30)); status != 409 {
		t.Errorf("an upload for the cancelled first: %d, want 409", status)
	}
}

func TestAnotherStudentOpeningEndsTheLinkOfThePrevious(t *testing.T) {
	e := newEnv(t)
	ann := e.open("ann", "e1")
	if status, _ := get(t, ann.URL); status != 200 {
		t.Fatalf("ann's page: %d", status)
	}
	bob := e.open("bob", "e1")
	if bob.URL == ann.URL {
		t.Error("two students share a link")
	}
	if status, _ := get(t, ann.URL); status != 410 {
		t.Errorf("ann's page after bob opened: %d, want 410", status)
	}
	if status, _ := get(t, bob.URL); status != 200 {
		t.Errorf("bob's page: %d", status)
	}
	// Ann's next submission makes a new link.
	ann2 := e.open("ann", "e1")
	if ann2.URL == ann.URL {
		t.Error("an ended link was reused")
	}
	if status, _ := get(t, bob.URL); status != 410 {
		t.Errorf("bob's page after ann opened again: %d, want 410", status)
	}
}

func TestSecondOpenCancelsAGradingSession(t *testing.T) {
	e := newEnv(t)
	inCall, release := make(chan struct{}), make(chan struct{})
	e.use(fake.New("Fake provider", fake.Step{
		Text:   reply(res("e1", grading.VerdictCorrect)),
		Meta:   grading.Meta{Model: "m", FinishReason: "stop"},
		Before: func() { close(inCall); <-release },
	}))
	first := e.open("ann", "e1")
	upload(t, first, photo(t, 40, 40))
	<-inCall
	second := e.open("ann", "e1")
	close(release)
	for range 3 {
		next(t, first)
	}
	expect[Cancelled](t, first)
	if err := e.svc.Cancel(context.Background(), second.ID); err != nil {
		t.Fatal(err)
	}
	if ep := e.load("ann").Exercises["e1"]; ep.Attempts != 0 {
		t.Errorf("the abandoned grading counted: %+v", ep)
	}
}

func TestFirewallHint(t *testing.T) {
	e := newEnv(t)
	silent := e.open("ann", "e1")
	e.clock.Advance(59 * time.Second)
	expectQuiet(t, silent)
	e.clock.Advance(time.Second)
	expect[FirewallHint](t, silent)
	e.clock.Advance(10 * time.Minute)
	expectQuiet(t, silent) // once only

	// A phone that opened the page in time cancels the hint.
	opened := e.open("bob", "e1")
	e.clock.Advance(30 * time.Second)
	get(t, opened.URL)
	expect[PageOpened](t, opened)
	e.clock.Advance(time.Minute)
	expectQuiet(t, opened)
}

func TestNoHintAfterTheSessionLeftWaiting(t *testing.T) {
	e := newEnv(t)
	s := e.open("ann", "e1")
	if err := e.svc.Cancel(context.Background(), s.ID); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(time.Hour)
	expect[Cancelled](t, s)
	expectClosed(t, s)
}

func TestCloseCancelsEverythingAndStopsThePhone(t *testing.T) {
	e := newEnv(t)
	a, b := e.open("ann", "e1"), e.open("bob", "e1")
	if !e.phone.Running() {
		t.Fatal("the phone server is not running")
	}
	if err := e.svc.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, s := range []*Session{a, b} {
		expect[Cancelled](t, s)
		expectClosed(t, s)
	}
	if e.phone.Running() {
		t.Error("the phone server still runs")
	}
	// A later Open starts it again.
	s := e.open("ann", "e1")
	if !e.phone.Running() {
		t.Error("the phone server did not restart")
	}
	if status, _ := get(t, s.URL); status != 200 {
		t.Errorf("page: %d", status)
	}
}

func TestPhoneKeepsRunningAndFollowsThePortSetting(t *testing.T) {
	e := newEnv(t)
	first := e.open("ann", "e1")
	port := e.phone.Port()
	if status, _ := get(t, first.URL); status != 200 {
		t.Errorf("page: %d", status)
	}
	// Same settings: the server is kept, and a second student shares it.
	other := e.open("bob", "e1")
	if e.phone.Port() != port {
		t.Error("the port changed")
	}

	// A new port while a session waits does not break it.
	e.settings.update(func(s *settingsT) { s.Phone.Port = freePort(t) })
	e.open("carl", "e1")
	if e.phone.Port() != port {
		t.Error("the server moved while sessions were waiting")
	}
	if err := e.svc.Cancel(context.Background(), first.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Cancel(context.Background(), other.ID); err != nil {
		t.Fatal(err)
	}
}

func TestPhoneMovesToANewPortWhenNobodyWaits(t *testing.T) {
	e := newEnv(t)
	s := e.open("ann", "e1")
	old := e.phone.Port()
	if err := e.svc.Cancel(context.Background(), s.ID); err != nil {
		t.Fatal(err)
	}
	want := freePort(t)
	e.settings.update(func(s *settingsT) { s.Phone.Port = want })
	s2 := e.open("ann", "e1")
	if e.phone.Port() != want || want == old {
		t.Errorf("port %d, want %d (was %d)", e.phone.Port(), want, old)
	}
	if status, _ := get(t, s2.URL); status != 200 {
		t.Errorf("page: %d", status)
	}
}

func TestRetryErrors(t *testing.T) {
	e := newEnv(t)
	s := e.open("ann", "e1")
	wantRoute(t, e.svc.Retry(context.Background(), s.ID), "Session.retry.SubmissionNotRetryable")
	wantRoute(t, e.svc.Retry(context.Background(), "nope"), "Service.Retry.SubmissionSessionUnknown")
}

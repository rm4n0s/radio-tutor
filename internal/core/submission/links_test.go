package submission

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/grading/fake"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/phone"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
)

// gradeOnce sends photos for the session and waits for the end of grading.
func (e *env) gradeOnce(s *Session) {
	e.t.Helper()
	if status, body := upload(e.t, s, photo(e.t, 40, 40)); status != 200 {
		e.t.Fatalf("upload: %d %s", status, body)
	}
	expect[UploadStarted](e.t, s)
	expect[PhotosReceived](e.t, s)
	expect[Grading](e.t, s)
	expect[Graded](e.t, s)
	expectClosed(e.t, s)
}

func TestTheNextSubmissionReusesTheLink(t *testing.T) {
	e := newEnv(t)
	e.use(fake.New("Fake provider", fake.Step{
		Text: reply(res("e1", grading.VerdictCorrect)),
		Meta: grading.Meta{Model: "m", FinishReason: "stop"},
	}))
	first := e.open("ann", "e1")
	if first.PhoneReady {
		t.Error("PhoneReady with no phone")
	}
	// The phone scans the code once and opens the page.
	get(t, first.URL)
	expect[PageOpened](t, first)
	e.gradeOnce(first)

	// The page is waiting, and the next submission shows on it.
	if _, body := get(t, first.URL+"/state"); !strings.Contains(body, `"state":"waiting"`) {
		t.Errorf("state after the first submission: %s", body)
	}
	second := e.open("ann", "e2")
	if second.URL != first.URL || second.Token != first.Token {
		t.Errorf("the link changed: %q then %q", first.URL, second.URL)
	}
	if !second.PhoneReady {
		t.Error("PhoneReady is false although the phone had the page open")
	}
	if _, body := get(t, second.URL+"/state"); !strings.Contains(body, `"state":"ready"`) || !strings.Contains(body, "Title of e2") {
		t.Errorf("state of the second submission: %s", body)
	}
	// A phone that is there needs no PageOpened and no firewall hint.
	e.clock.Advance(2 * time.Minute)
	expectQuiet(t, second)
}

func TestPhoneReadyOnlyWhileThePhoneIsThere(t *testing.T) {
	e := newEnv(t)
	first := e.open("ann", "e1")
	get(t, first.URL)
	if err := e.svc.Cancel(context.Background(), first.ID); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(phone.SeenWithin + time.Second)
	second := e.open("ann", "e1")
	if second.PhoneReady {
		t.Error("PhoneReady although the phone was not seen for more than 30 seconds")
	}
	// It comes back: the page is announced, once.
	get(t, second.URL+"/state")
	expect[PageOpened](t, second)
}

func TestEndLinkEndsThePhoneLink(t *testing.T) {
	e := newEnv(t)
	first := e.open("ann", "e1")
	e.svc.EndLink("ann")
	if status, body := get(t, first.URL); status != 410 || !strings.Contains(body, "This link has expired") {
		t.Errorf("page after EndLink: %d %s", status, body)
	}
	if status, _ := get(t, first.URL+"/state"); status != 410 {
		t.Errorf("state after EndLink: %d", status)
	}
	if status, _ := upload(t, first, photo(t, 30, 30)); status != 410 {
		t.Errorf("upload after EndLink: %d", status)
	}
	// The session itself is not cancelled by it.
	expectQuiet(t, first)
	// The next Open makes a new link.
	second := e.open("ann", "e1")
	if second.URL == first.URL {
		t.Error("an ended link was reused")
	}
	if status, _ := get(t, second.URL); status != 200 {
		t.Errorf("new link: %d", status)
	}
	e.svc.EndLink("ann") // with nothing to end
	e.svc.EndLink("bob")
	if status, _ := get(t, second.URL); status != 410 {
		t.Errorf("second link after EndLink: %d", status)
	}
}

func TestLinkIdleHourAndSessionExpiry(t *testing.T) {
	e := newEnv(t)
	first := e.open("ann", "e1")
	get(t, first.URL)

	// The submission expires after 30 minutes: the page waits and the link stays.
	e.clock.Advance(ExpiresAfter)
	expect[PageOpened](t, first)
	expect[Expired](t, first)
	expectClosed(t, first)
	if status, body := get(t, first.URL+"/state"); status != 200 || !strings.Contains(body, `"state":"waiting"`) {
		t.Errorf("after the expiry: %d %s", status, body)
	}
	// 60 minutes after it ended, the link is idle and ends.
	e.clock.Advance(time.Hour - time.Second)
	if status, _ := get(t, first.URL); status != 200 {
		t.Errorf("one second before the hour: %d", status)
	}
	e.clock.Advance(time.Second)
	if status, _ := get(t, first.URL); status != 410 {
		t.Errorf("after the hour: %d, want 410", status)
	}
	second := e.open("ann", "e1")
	if second.URL == first.URL {
		t.Error("an idle link was reused")
	}
}

func TestANewLinkWhenTheLanguageOrTheAddressChanges(t *testing.T) {
	e := newEnv(t)
	first := e.open("ann", "e1")

	e.settings.update(func(s *settings.Settings) { s.Language = i18n.EL })
	second := e.open("ann", "e1")
	if second.URL == first.URL {
		t.Error("the link was reused after the language changed")
	}
	if status, _ := get(t, first.URL); status != 410 {
		t.Errorf("the old link: %d, want 410", status)
	}
	if _, body := get(t, second.URL); !strings.Contains(body, `lang="el"`) {
		t.Errorf("the new link is not in Greek: %.200s", body)
	}

	// The same language and address keep the link.
	third := e.open("ann", "e1")
	if third.URL != second.URL {
		t.Error("the link changed with nothing changed")
	}
}

func TestCloseEndsTheLinks(t *testing.T) {
	e := newEnv(t)
	s := e.open("ann", "e1")
	if err := e.svc.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	expect[Cancelled](t, s)
	expectClosed(t, s)
	if e.phone.Running() {
		t.Error("the phone server still runs")
	}
	e.svc.mu.Lock()
	n := len(e.svc.links)
	e.svc.mu.Unlock()
	if n != 0 {
		t.Errorf("%d links are kept after Close", n)
	}
}

func TestOpenMakesANewLinkWhenTheOldOneEnded(t *testing.T) {
	e := newEnv(t)
	first := e.open("ann", "e1")
	e.svc.mu.Lock()
	link := e.svc.links["ann"].link
	e.svc.mu.Unlock()
	link.End() // ended behind the service's back, e.g. by the idle rule
	second := e.open("ann", "e2")
	if second.URL == first.URL {
		t.Error("an ended link was reused")
	}
	if status, _ := get(t, second.URL); status != 200 {
		t.Errorf("the new link: %d", status)
	}
}

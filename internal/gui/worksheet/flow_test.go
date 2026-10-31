package worksheet

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/grading/fake"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/submission"
)

// goodReply grades the shade exercise correct and the name exercise partial.
func goodReply() fake.Step {
	return fake.Reply(fake.JSONReply("neat page",
		result(exShade, grading.VerdictCorrect, "Two equal halves, well done!"),
		result(exName, grading.VerdictPartial, "You are close, look at the slices again."),
	))
}

func TestFullFlowFromTheWorksheetToTheResults(t *testing.T) {
	h := pump(t, config{})
	prov := fake.New("fake", goodReply())
	h.provider.use(prov)

	h.check()
	url := h.url()
	if !strings.HasPrefix(url, "http://127.0.0.1:") || !strings.Contains(url, "/p/") {
		t.Fatalf("the address %q is not a phone address", url)
	}
	for _, text := range []string{"Scan this with your phone", "Exercises: 1, B2", "Photos received: 0"} {
		if !h.has(text) {
			t.Errorf("the QR dialog does not show %q", text)
		}
	}
	if sz := h.Size(key(KeyQR)); sz.Width < MinQRSize || sz.Height < MinQRSize {
		t.Errorf("the QR code is %vx%v, want at least %v", sz.Width, sz.Height, MinQRSize)
	}
	if h.HasKey(KeyFirewall) {
		t.Error("the firewall hint is shown before it is time")
	}
	if got := h.waitSubmission(progress.SubmissionWaitingPhotos); len(got.ExerciseIDs) != 2 {
		t.Errorf("the submission has %v, want two exercises", got.ExerciseIDs)
	}

	// The phone opens the page, then sends a photo.
	if status := get(t, url); status != 200 {
		t.Fatalf("GET status %d", status)
	}
	h.whileBusy("the page opened", func() bool { return h.has("Your phone opened the page.") })
	if status := upload(t, url); status != 200 {
		t.Fatalf("upload status %d", status)
	}
	h.waitKey(KeyResultPrefix + exShade)

	// The results.
	for _, text := range []string{"Correct!", "Almost there", "Two equal halves, well done!", "You are close, look at the slices again.",
		"New hint: How many slices are there in all?"} {
		if !h.has(text) {
			t.Errorf("the results do not show %q", text)
		}
	}
	if !h.HasKey(KeyStarPrefix + exShade) {
		t.Error("the passed exercise has no star")
	}
	if h.HasKey(KeyStarPrefix + exName) {
		t.Error("an exercise that was not passed has a star")
	}
	if !h.HasKey(KeyMarkPrefix+exShade) || !h.HasKey(KeyMarkPrefix+exName) || !h.HasKey(KeyNewHintPrefix+exName) {
		t.Error("a mark or the new hint is missing")
	}
	if len(prov.Calls()) != 1 {
		t.Errorf("the provider was called %d times, want 1", len(prov.Calls()))
	}
	p := h.load()
	if p.Exercises[exShade].State != progress.StatePassed || p.Exercises[exName].HintsShown != 1 {
		t.Errorf("the save file does not show the results: %+v", p.Exercises)
	}

	// The submission is no longer waiting, but the link stays open for the next.
	if status := get(t, url); status != 200 {
		t.Errorf("GET after grading status %d, want 200", status)
	}
	if offer := offerOf(t, url); offer != 0 {
		t.Errorf("the page still shows submission %d after grading", offer)
	}

	// Back to the worksheet, which shows the new state.
	h.TapKey(KeyBack)
	h.WaitFor(t, "the worksheet", func() bool { return h.HasKey(KeyCheck) && h.has("Hint 1: How many slices are there in all?") })
	if h.HasKey(KeyQR) || h.HasKey(KeyResultPrefix+exShade) {
		t.Error("the flow is still on screen")
	}
	if !h.has("Passed") || !h.has("1 selected") {
		t.Error("the worksheet did not reload the new state")
	}
}

func TestBackToTheWorksheetShowsNextWhenGradingFinishedIt(t *testing.T) {
	h := pump(t, config{change: func(p *progress.Progress) {
		lessonDone(p)
		p.Exercises[exName] = progress.ExerciseProgress{State: progress.StatePassed, Attempts: 1, PassedBy: progress.PassedByAI}
	}})
	if h.HasKey(KeyNext) || !h.has("1 selected") {
		t.Fatal("the worksheet is finished before the last exercise is graded")
	}
	h.provider.use(fake.New("fake", fake.Reply(fake.JSONReply("",
		result(exShade, grading.VerdictCorrect, "Two equal halves, well done!")))))
	h.check()
	if status := upload(t, h.url()); status != 200 {
		t.Fatalf("upload status %d", status)
	}
	h.waitKey(KeyResultPrefix + exShade)
	h.TapKey(KeyBack)
	h.WaitFor(t, "the finished worksheet", func() bool { return h.HasKey(KeyNext) })
	if h.HasKey(KeyCheck) || !h.has("You finished this worksheet!") {
		t.Error("the worksheet that grading finished still offers Check my answers")
	}
}

func TestRetryAfterAFailure(t *testing.T) {
	h := pump(t, config{})
	prov := fake.New("fake", fake.Fail(errors.New("OpenAINetwork", "the server cannot be reached")), goodReply())
	h.provider.use(prov)

	h.check()
	if status := upload(t, h.url()); status != 200 {
		t.Fatalf("upload status %d", status)
	}
	h.waitKey(KeyFailure)
	if !h.has("The checker could not answer. Try again.") {
		t.Error("no friendly failure text")
	}
	// A child never sees the technical details.
	if h.HasKey(KeyDetails) || h.has("OpenAINetwork") {
		t.Error("the failure shows technical details to a student")
	}
	sub := h.waitSubmission(progress.SubmissionFailed)
	if len(sub.Photos) != 1 {
		t.Errorf("the failed submission has photos %v, want the one that was sent", sub.Photos)
	}

	h.tap(KeyRetry)
	h.waitKey(KeyResultPrefix + exShade)
	calls := prov.Calls()
	if len(calls) != 2 {
		t.Fatalf("the provider was called %d times, want 2", len(calls))
	}
	if len(calls[0].Request.Photos) != len(calls[1].Request.Photos) {
		t.Error("the retry did not grade the same photos")
	}
	if h.load().Exercises[exShade].State != progress.StatePassed {
		t.Error("the retry did not pass the exercise")
	}
}

func TestNoProviderAsksAParent(t *testing.T) {
	h := pump(t, config{noProvider: true})
	h.check()
	if status := upload(t, h.url()); status != 200 {
		t.Fatalf("upload status %d", status)
	}
	h.waitKey(KeyFailure)
	if !h.has("Ask a parent to set up the checker.") {
		t.Error("no message about the checker")
	}
}

func TestCancelOnTheQRDialog(t *testing.T) {
	h := pump(t, config{})
	h.check()
	url := h.url()
	h.TapKey(KeyCancel)
	if h.HasKey(KeyQR) || !h.HasKey(KeyCheck) {
		t.Fatal("Cancel did not return to the worksheet")
	}
	h.waitSubmission(progress.SubmissionCancelled)
	if offer := offerOf(t, url); offer != 0 {
		t.Errorf("the page still shows submission %d after Cancel", offer)
	}
	if len(h.load().Submissions) != 1 {
		t.Errorf("the save file has %d submissions, want 1", len(h.load().Submissions))
	}
}

func TestCancelFromTheFailure(t *testing.T) {
	h := pump(t, config{})
	h.provider.use(fake.New("fake", fake.Fail(errors.New("OpenAIAuth", "refused"))))
	h.check()
	upload(t, h.url())
	h.waitKey(KeyFailure)
	h.TapKey(KeyCancel)
	if !h.HasKey(KeyCheck) {
		t.Fatal("Cancel did not return to the worksheet")
	}
	h.waitSubmission(progress.SubmissionCancelled)
}

func TestCancelWhileGrading(t *testing.T) {
	h := pump(t, config{})
	asked := make(chan struct{})
	release := make(chan struct{})
	step := goodReply()
	step.Before = func() {
		close(asked)
		<-release
	}
	prov := fake.New("fake", step)
	h.provider.use(prov)
	h.check()
	upload(t, h.url())
	h.waitKey(KeyGrading)
	<-asked
	if !h.has("Checking your answers...") || !h.has("Photos received: 1") {
		t.Error("no grading text or photo count")
	}
	h.tap(KeyCancel)
	// The screen leaves after its pop animation; its disposal cancels the session.
	h.whileBusy("the flow to close", func() bool { return !h.HasKey(KeyGrading) })
	close(release)
	if !h.HasKey(KeyCheck) {
		t.Fatal("Cancel did not return to the worksheet")
	}
	h.waitSubmission(progress.SubmissionCancelled)
	if p := h.load(); p.Exercises[exShade].State == progress.StatePassed {
		t.Error("an abandoned grading changed the progress")
	}
}

func TestFirewallHintAfterAMinute(t *testing.T) {
	h := pump(t, config{})
	h.check()
	h.clock.Advance(59 * time.Second)
	h.Pump()
	if h.HasKey(KeyFirewall) {
		t.Fatal("the firewall hint came before a minute")
	}
	h.clock.Advance(time.Second)
	h.whileBusy("the firewall hint", func() bool { return h.HasKey(KeyFirewall) })
	text := h.textOf(KeyFirewall)
	port := strconv.Itoa(h.Env.Settings.Get().Phone.Port)
	for _, part := range []string{"port " + port, "firewall-cmd --add-port=" + port + "/tcp --permanent", "firewall-cmd --reload", "README"} {
		if !strings.Contains(text, part) {
			t.Errorf("the firewall hint %q lacks %q", text, part)
		}
	}
}

func TestOpenFailures(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want string
	}{
		{"no network", "PhoneNoNetwork", "This computer is not connected to a network."},
		{"port in use", "PhonePortInUse", "The page for the phone could not start. Ask a parent."},
		{"anything else", "StorageBroken", "Something went wrong. Try again."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := pump(t, config{})
			h.failNextOpen(errors.New(tt.tag, "no way"))
			h.tap(KeyCheck)
			h.waitKey(KeyFailure)
			if got := h.textOf(KeyFailure); got != tt.want {
				t.Errorf("failure text %q, want %q", got, tt.want)
			}
			if h.HasKey(KeyQR) {
				t.Error("a QR code is shown although opening failed")
			}
			h.TapKey(KeyCancel)
			if !h.HasKey(KeyCheck) {
				t.Error("Cancel did not return to the worksheet")
			}
		})
	}
}

func TestTryAgainAfterAnOpenFailureOpensANewSubmission(t *testing.T) {
	h := pump(t, config{})
	h.failNextOpen(errors.New("PhoneNoNetwork", "no way"))
	h.tap(KeyCheck)
	h.waitKey(KeyFailure)
	h.tap(KeyRetry)
	h.waitKey(KeyQR)
}

func TestExpiredLinkOffersANewOne(t *testing.T) {
	h := pump(t, config{})
	h.check()
	h.clock.Advance(submission.ExpiresAfter)
	h.waitKey(KeyFailure)
	if !h.has("The link ran out of time. Try again.") {
		t.Error("no message about the expired link")
	}
	h.tap(KeyRetry)
	h.waitKey(KeyQR)
}

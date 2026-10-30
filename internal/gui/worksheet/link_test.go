package worksheet

import (
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/grading/fake"
	"github.com/rm4n0s/radio-tutor/internal/core/submission"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

func TestNextCheckWithAPhoneThatIsThereShowsYourPhoneIsReady(t *testing.T) {
	h := pump(t, config{})
	h.provider.use(fake.New("fake", goodReply()))

	// The first Check my answers has no phone yet: the code is large.
	h.check()
	url := h.url()
	if h.HasKey(KeyPhoneReady) {
		t.Fatal("the first dialog says the phone is ready")
	}
	if sz := h.Size(key(KeyQR)); sz.Width < MinQRSize {
		t.Fatalf("the first code is %v wide, want at least %v", sz.Width, MinQRSize)
	}
	if status := get(t, url); status != 200 {
		t.Fatalf("GET status %d", status)
	}
	h.whileBusy("the page opened", func() bool { return h.has("Your phone opened the page.") })
	if status := upload(t, url); status != 200 {
		t.Fatalf("upload status %d", status)
	}
	h.waitKey(KeyResultPrefix + exShade)
	h.TapKey(KeyBack)
	h.WaitFor(t, "the worksheet", func() bool { return h.HasKey(KeyCheck) })

	// The phone is still there (the page waits for news): the next dialog says
	// so, shows the same link, and keeps the code for another phone.
	h.check()
	if got := h.url(); got != url {
		t.Errorf("the second submission has the address %q, want the same %q", got, url)
	}
	if !h.HasKey(KeyPhoneReady) || !h.has("Your phone is ready: the page updated by itself") {
		t.Fatal("the dialog does not say that the phone is ready")
	}
	sz := h.Size(key(KeyQR))
	if sz.Width >= MinQRSize || sz.Width < SmallQRSize {
		t.Errorf("the code for another phone is %v wide, want between %v and %v", sz.Width, SmallQRSize, MinQRSize)
	}
	if !h.has("Another phone? Scan this.") {
		t.Error("the dialog does not invite another phone")
	}
	// No firewall hint: a phone is clearly connected.
	h.clock.Advance(submission.FirewallHintAfter)
	h.Pump()
	if h.HasKey(KeyFirewall) {
		t.Error("the firewall hint is shown although a phone has the link open")
	}
	// The page shows the new submission by itself, and its photos are graded
	// as usual.
	if offerOf(t, url) == 0 {
		t.Error("the phone's page does not show the second submission")
	}
}

func TestADifferentSubmissionNeedsNoNewScan(t *testing.T) {
	// The phone that scanned once sends the photos of the second submission to
	// the same address.
	h := pump(t, config{})
	h.provider.use(fake.New("fake", goodReply(), goodReply()))
	h.check()
	url := h.url()
	if status := upload(t, url); status != 200 {
		t.Fatalf("first upload %d", status)
	}
	h.waitKey(KeyResultPrefix + exShade)
	h.TapKey(KeyBack)
	h.WaitFor(t, "the worksheet", func() bool { return h.HasKey(KeyCheck) })
	h.check()
	if status := upload(t, h.url()); status != 200 {
		t.Fatalf("second upload %d", status)
	}
	h.waitKey(KeyBack)
}

func TestLogoutEndsThePhoneLink(t *testing.T) {
	h := pump(t, config{})
	h.check()
	url := h.url()
	if status := get(t, url); status != 200 {
		t.Fatalf("GET status %d", status)
	}
	h.TapKey(KeyCancel)
	h.WaitFor(t, "the worksheet", func() bool { return h.HasKey(KeyCheck) })
	h.TapKey(app.KeyLogout)
	h.whileBusy("the link to end", func() bool { return get(t, url) == 410 })
	if status := uploadOffer(t, url, 1); status != 410 {
		t.Errorf("upload on the ended link: %d, want 410", status)
	}
}

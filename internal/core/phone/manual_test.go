//go:build radiotutor_phone

package phone

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
)

// TestServeForRealPhone serves the page on the LAN for a person with a real
// phone (docs/MANUAL-CHECKS.md, step 4.1). It prints the URL, saves the
// received photos in a temporary folder and ends when they arrive or after
// five minutes. Run it with: go test -tags radiotutor_phone ./internal/core/phone -run TestServeForRealPhone -v
func TestServeForRealPhone(t *testing.T) {
	lang := i18n.EN
	if os.Getenv("RADIO_TUTOR_LANG") == "el" {
		lang = i18n.EL
	}
	lan, err := DetectLAN(context.Background(), os.Getenv("RADIO_TUTOR_INTERFACE"))
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{Lang: lang})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := s.Start(ctx, DefaultPort); err != nil {
		t.Fatal(err)
	}
	defer s.Stop(context.Background())

	dir := t.TempDir()
	k, err := s.OpenLink(lan.IP.String(), lang)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("open this on the phone, once: %s", k.URL)
	// Two submissions one after the other on the same page (SPEC §10.1): after
	// the first is sent the page waits, and the second appears by itself.
	pages := []PageData{
		{Title: "Manual check <&> cassette", Exercises: []PageExercise{
			{Label: "1", Title: "Add", Prompt: "What is 2 + 2?\nWrite the answer."},
			{Label: "2", Title: "Subtract", Prompt: "What is 5 - 3?"},
		}},
		{Title: "Manual check <&> cassette", Exercises: []PageExercise{
			{Label: "3", Title: "Multiply", Prompt: "What is 3 x 4?"},
		}},
	}
	for n, page := range pages {
		done := make(chan struct{})
		_, _, err := k.Offer(page, Handlers{
			Opened:  func() { t.Log("page opened") },
			Started: func() { t.Log("upload started") },
			Received: func(_ context.Context, photos [][]byte) error {
				for i, p := range photos {
					name := filepath.Join(dir, strconv.Itoa(n+1)+"-"+strconv.Itoa(i+1)+".jpg")
					if err := os.WriteFile(name, p, 0o600); err != nil {
						return err
					}
				}
				t.Logf("received %d photos in %s", len(photos), dir)
				close(done)
				return nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("no photos within five minutes")
		}
		time.Sleep(10 * time.Second) // the page shows "Sent!" and waits for the next
	}
}

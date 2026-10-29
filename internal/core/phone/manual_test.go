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
	done := make(chan struct{})
	page := PageData{Lang: lang, Title: "Manual check <&> cassette", Exercises: []PageExercise{
		{Label: "1", Title: "Add", Prompt: "What is 2 + 2?\nWrite the answer."},
		{Label: "2", Title: "Subtract", Prompt: "What is 5 - 3?"},
	}}
	_, url, err := s.Open(lan.IP.String(), page, Handlers{
		Opened:  func() { t.Log("page opened") },
		Started: func() { t.Log("upload started") },
		Received: func(_ context.Context, photos [][]byte) error {
			for i, p := range photos {
				name := filepath.Join(dir, strconv.Itoa(i+1)+".jpg")
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
	t.Logf("open this on the phone: %s", url)
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("no photos within five minutes")
	}
}

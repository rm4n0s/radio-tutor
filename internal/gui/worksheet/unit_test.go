package worksheet

import (
	"testing"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/qrcode"
)

func TestFailureTexts(t *testing.T) {
	tests := []struct {
		name    string
		f       failure
		admin   bool
		want    string
		details string
	}{
		{"no provider", failure{err: errors.New("SubmissionNoProvider", "none"), canRetry: true}, false,
			"Ask a parent to set up the checker.", ""},
		{"auth", failure{err: errors.New("OpenAIAuth", "refused"), canRetry: true}, false,
			"The checker could not answer. Try again.", ""},
		{"not found", failure{err: errors.New("OpenAINotFound", "no model"), canRetry: true}, false,
			"The checker could not answer. Try again.", ""},
		{"rate limited", failure{err: errors.New("OpenAIRateLimited", "slow down"), canRetry: true}, false,
			"The checker could not answer. Try again.", ""},
		{"network", failure{err: errors.New("OpenAINetwork", "down"), canRetry: true}, false,
			"The checker could not answer. Try again.", ""},
		{"bad reply", failure{err: errors.New("GradingReplyInvalid", "bad"), canRetry: true}, false,
			"The checker could not answer. Try again.", ""},
		{"saving failed but can retry", failure{err: errors.New("StorageBroken", "disk"), canRetry: true}, false,
			"The checker could not answer. Try again.", ""},
		{"no network", failure{err: errors.New("PhoneNoNetwork", "none")}, false,
			"This computer is not connected to a network.", ""},
		{"port in use", failure{err: errors.New("PhonePortInUse", "port 8737 is taken")}, false,
			"The page for the phone could not start. Ask a parent.", ""},
		{"cancelled elsewhere", failure{}, false, "Something went wrong. Try again.", ""},
		{"expired", failure{expired: true}, false, "The link ran out of time. Try again.", ""},
		{"nothing to retry", failure{err: errors.New("StorageBroken", "disk")}, false, "Something went wrong. Try again.", ""},
		{"admin sees the details", failure{err: errors.New("OpenAIAuth", "the server refused the API key"), canRetry: true}, true,
			"The checker could not answer. Try again.", "Details: the server refused the API key (OpenAIAuth)"},
		{"admin: expired has none", failure{expired: true}, true, "The link ran out of time. Try again.", ""},
	}
	cat := i18n.For(i18n.EN)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := failureText(cat, &tt.f); got != tt.want {
				t.Errorf("text %q, want %q", got, tt.want)
			}
			if got := failureDetails(cat, tt.admin, &tt.f); got != tt.details {
				t.Errorf("details %q, want %q", got, tt.details)
			}
		})
	}
}

func TestQRModuleSize(t *testing.T) {
	tests := []struct {
		name, url string
		min       float64
	}{
		{"a short address", "http://192.168.1.20:8737/p/abcdefghijklmnopqrstuvwxyz", MinQRSize},
		{"a long address", "http://192.168.100.200:8737/p/abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789", MinQRSize},
		{"a short address, small", "http://192.168.1.20:8737/p/abcdefghijklmnopqrstuvwxyz", SmallQRSize},
		{"a long address, small", "http://192.168.100.200:8737/p/abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789", SmallQRSize},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matrix, err := qrcode.Encode(tt.url, qrcode.DefaultLevel)
			if err != nil {
				t.Fatal(err)
			}
			module := qrModuleSize(matrix.Total(), tt.min)
			if module != float64(int(module)) || module < 1 {
				t.Errorf("module %v is not a whole number of pixels", module)
			}
			if side := module * float64(matrix.Total()); side < tt.min {
				t.Errorf("the code is %v pixels wide, want at least %v", side, tt.min)
			}
		})
	}
	if qrModuleSize(0, MinQRSize) != 1 {
		t.Error("an empty matrix has no module size")
	}
}

func TestBadgeWidth(t *testing.T) {
	tests := []struct {
		label string
		want  float64
	}{{"1", badgeSize}, {"B2", badgeSize}, {"A.1", 84}, {"ABCDEF", 144}}
	for _, tt := range tests {
		if got := badgeWidth(tt.label); got != tt.want {
			t.Errorf("badgeWidth(%q) = %v, want %v", tt.label, got, tt.want)
		}
	}
}

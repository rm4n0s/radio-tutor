package profile

import (
	"bytes"
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"strings"
	"testing"

	"github.com/rm4n0s/errors"
)

// routeOf returns err as an *errors.Error or fails the test.
func routeOf(t *testing.T, err error) *errors.Error {
	t.Helper()
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T: %v", err, err)
	}
	return appErr
}

func TestHashFormat(t *testing.T) {
	h := newHasher()
	h.iterations = 1000
	stored, err := h.hash("secret-pass")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" || parts[1] != "1000" {
		t.Fatalf("hash %q does not have the SPEC §3.1 shape", stored)
	}
	salt, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) != 16 {
		t.Errorf("salt: %d bytes, %v", len(salt), err)
	}
	key, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil || len(key) != 32 {
		t.Errorf("key: %d bytes, %v", len(key), err)
	}
	want, err := pbkdf2.Key(sha256.New, "secret-pass", salt, 1000, 32)
	if err != nil || !bytes.Equal(want, key) {
		t.Errorf("key is not PBKDF2-HMAC-SHA256 of the password: %v", err)
	}
	again, err := h.hash("secret-pass")
	if err != nil || again == stored {
		t.Errorf("two hashes of one password must differ (random salt): %v", err)
	}
	if got := newHasher().iterations; got != 600_000 {
		t.Errorf("default iterations = %d, want 600000", got)
	}
}

func TestVerify(t *testing.T) {
	low := newHasher()
	low.iterations = 3
	stored, err := low.hash("right-pass")
	if err != nil {
		t.Fatal(err)
	}
	// The iteration count comes from the stored hash, not from the hasher.
	other := newHasher()
	other.iterations = 99

	tests := []struct {
		name     string
		password string
		stored   string
		want     bool
	}{
		{"right password", "right-pass", stored, true},
		{"wrong password", "wrong-pass", stored, false},
		{"empty password", "", stored, false},
		{"case matters", "RIGHT-PASS", stored, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := other.verify(tt.password, tt.stored)
			if err != nil || got != tt.want {
				t.Errorf("verify = %v, %v; want %v", got, err, tt.want)
			}
		})
	}
}

func TestParseHashInvalid(t *testing.T) {
	b64 := func(n int) string { return base64.StdEncoding.EncodeToString(make([]byte, n)) }
	tests := []struct {
		name    string
		stored  string
		wantTag string
	}{
		{"empty", "", "PasswordHashMalformed"},
		{"too few parts", "pbkdf2-sha256$10$" + b64(16), "PasswordHashMalformed"},
		{"too many parts", "pbkdf2-sha256$10$" + b64(16) + "$" + b64(32) + "$x", "PasswordHashMalformed"},
		{"other scheme", "pbkdf2-sha1$10$" + b64(16) + "$" + b64(32), "PasswordHashMalformed"},
		{"iterations not a number", "pbkdf2-sha256$ten$" + b64(16) + "$" + b64(32), "PasswordHashIterationsInvalid"},
		{"iterations zero", "pbkdf2-sha256$0$" + b64(16) + "$" + b64(32), "PasswordHashIterationsInvalid"},
		{"iterations huge", "pbkdf2-sha256$999999999$" + b64(16) + "$" + b64(32), "PasswordHashIterationsInvalid"},
		{"salt not base64", "pbkdf2-sha256$10$!!!$" + b64(32), "PasswordHashSaltInvalid"},
		{"salt short", "pbkdf2-sha256$10$" + b64(4) + "$" + b64(32), "PasswordHashSaltInvalid"},
		{"key not base64", "pbkdf2-sha256$10$" + b64(16) + "$!!!", "PasswordHashKeyInvalid"},
		{"key empty", "pbkdf2-sha256$10$" + b64(16) + "$", "PasswordHashKeyInvalid"},
		{"key long", "pbkdf2-sha256$10$" + b64(16) + "$" + b64(65), "PasswordHashKeyInvalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newHasher().verify("x", tt.stored)
			if err == nil {
				t.Fatal("expected an error")
			}
			appErr := routeOf(t, err)
			if appErr.Tag != tt.wantTag || !appErr.HasRoute("hasher.verify->parseHash."+tt.wantTag) {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
			if tt.stored != "" && strings.Contains(appErr.Message, tt.stored) {
				t.Errorf("message leaks the stored hash: %q", appErr.Message)
			}
		})
	}
}

func TestHashFailures(t *testing.T) {
	failing := errors.New("Fake", "boom")
	tests := []struct {
		name   string
		mutate func(*hasher)
		route  string
	}{
		{"random source fails", func(h *hasher) { h.random = io.LimitReader(strings.NewReader(""), 0) }, "hasher.hash.SaltGenerationFailed"},
		{"derivation fails", func(h *hasher) {
			h.derive = func(string, []byte, int, int) ([]byte, error) { return nil, failing }
		}, "hasher.hash.KeyDerivationFailed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHasher()
			tt.mutate(&h)
			_, err := h.hash("password")
			if err == nil {
				t.Fatal("expected an error")
			}
			if appErr := routeOf(t, err); !appErr.HasRoute(tt.route) {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
		})
	}

	t.Run("verify derivation fails", func(t *testing.T) {
		h := newHasher()
		h.iterations = 1
		stored, err := h.hash("password")
		if err != nil {
			t.Fatal(err)
		}
		h.derive = func(string, []byte, int, int) ([]byte, error) { return nil, failing }
		_, err = h.verify("password", stored)
		if err == nil {
			t.Fatal("expected an error")
		}
		if appErr := routeOf(t, err); !appErr.HasRoute("hasher.verify.KeyDerivationFailed") {
			t.Errorf("unexpected failure path: %s", appErr.Route())
		}
	})
}

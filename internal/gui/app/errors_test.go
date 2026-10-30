package app

import (
	"testing"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
)

func TestErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		detailed bool
		want     i18n.Message
	}{
		{"username", errors.New("UsernameInvalid", "x"), true, i18n.M("err.username_invalid")},
		{"display name", errors.New("DisplayNameInvalid", "x"), true, i18n.M("err.display_name_invalid")},
		{"password short", errors.New("PasswordTooShort", "x", "min", 8), true, i18n.M("err.password_short", "min", "8")},
		{"taken", errors.New("UsernameTaken", "x"), false, i18n.M("err.username_taken")},
		{"wrong password", errors.New("LoginFailed", "x"), false, i18n.M("login.wrong")},
		{"throttled rounds up", errors.New("LoginThrottled", "x", "retry_after", "29.2s"), false, i18n.M("login.throttled", "seconds", "30")},
		{"throttled bad duration", errors.New("LoginThrottled", "x", "retry_after", "soon"), false, i18n.M("login.throttled", "seconds", "1")},
		{"throttled no duration", errors.New("LoginThrottled", "x"), false, i18n.M("login.throttled", "seconds", "1")},
		{"other, child", errors.New("RenameFailed", "disk full"), false, i18n.M("err.generic")},
		{"other, admin", errors.New("RenameFailed", "disk full"), true, i18n.M("err.generic_detail", "message", "disk full", "tag", "RenameFailed")},
		{"not an app error, admin", plainError("plain"), true, i18n.M("err.generic_detail", "message", "plain", "tag", "Unknown")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := errorMessage(tt.err, tt.detailed)
			if got.Key != tt.want.Key || len(got.Args) != len(tt.want.Args) {
				t.Fatalf("errorMessage = %+v, want %+v", got, tt.want)
			}
			for i := range got.Args {
				if got.Args[i] != tt.want.Args[i] {
					t.Errorf("args = %v, want %v", got.Args, tt.want.Args)
				}
			}
		})
	}
}

// plainError is an error that is not from the errors package.
type plainError string

func (e plainError) Error() string { return string(e) }

func TestErrorMessageIsExported(t *testing.T) {
	err := errors.New("UsernameTaken", "taken")
	if got := ErrorMessage(err, true); got.Key != "err.username_taken" {
		t.Errorf("ErrorMessage = %v", got)
	}
	if got := ErrorMessage(errors.New("Other", "x"), true); got.Key != "err.generic_detail" {
		t.Errorf("detailed message = %v", got)
	}
	if got := ErrorMessage(errors.New("Other", "x"), false); got.Key != "err.generic" {
		t.Errorf("plain message = %v", got)
	}
}

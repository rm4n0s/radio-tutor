package app

import (
	"math"
	"strconv"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
)

// ErrorMessage turns an error of the profile store (or any other error) into the
// message the user sees; see errorMessage. Admin screens call it with detailed
// true, so that the message and the tag are shown.
func ErrorMessage(err error, detailed bool) i18n.Message { return errorMessage(err, detailed) }

// errorMessage turns an error of the profile store into the message the user
// sees. Failures the user can fix (a wrong password, a short one) get their own
// message. Anything else is "Something went wrong. Ask a parent." for a child
// and the full message with the tag for an admin (detailed, SPEC §15).
func errorMessage(err error, detailed bool) i18n.Message {
	appErr, ok := errors.FromError(err)
	if !ok {
		return genericMessage(err.Error(), "Unknown", detailed)
	}
	switch appErr.Tag {
	case "UsernameInvalid":
		return i18n.M("err.username_invalid")
	case "DisplayNameInvalid":
		return i18n.M("err.display_name_invalid")
	case "PasswordTooShort":
		min, _ := appErr.Metadata["min"].(int)
		return i18n.M("err.password_short", "min", strconv.Itoa(min))
	case "UsernameTaken":
		return i18n.M("err.username_taken")
	case "LoginFailed":
		return i18n.M("login.wrong")
	case "LoginThrottled":
		return i18n.M("login.throttled", "seconds", strconv.Itoa(waitSeconds(appErr.Metadata["retry_after"])))
	}
	return genericMessage(appErr.Message, appErr.Tag, detailed)
}

func genericMessage(message, tag string, detailed bool) i18n.Message {
	if detailed {
		return i18n.M("err.generic_detail", "message", message, "tag", tag)
	}
	return i18n.M("err.generic")
}

// waitSeconds reads the "retry_after" metadata of LoginThrottled (a duration
// string) and rounds it up to whole seconds, at least 1.
func waitSeconds(v any) int {
	text, _ := v.(string)
	d, err := time.ParseDuration(text)
	if err != nil || d <= 0 {
		return 1
	}
	return max(1, int(math.Ceil(d.Seconds())))
}

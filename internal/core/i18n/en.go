package i18n

// messagesEN are the English messages (the source language). Keep the keys the
// same as in el.go.
var messagesEN = map[string]string{
	// The language switch.
	"lang.switch": "Language",

	// First run: create the parent account.
	"firstrun.title":          "Create the parent account",
	"firstrun.intro":          "Welcome! Start by creating the account of a parent. Parents add the children and the cassettes.",
	"firstrun.create":         "Create account",
	"field.username":          "Username",
	"field.username_hint":     "a-z, 0-9 and _ (2 to 32)",
	"field.display_name":      "Name shown",
	"field.password":          "Password",
	"field.password_again":    "Repeat the password",
	"field.password_min_hint": "At least {min} characters",

	// Login.
	"login.title":     "Who is learning today?",
	"login.password":  "Hello, {name}! Type your password.",
	"login.enter":     "Enter",
	"login.back":      "Back",
	"login.wrong":     "That is not the right password. Try again.",
	"login.throttled": "Too many wrong tries. Wait {seconds} seconds and try again.",

	// The homes (placeholders until the next steps).
	"home.admin.title":       "Parent area",
	"home.admin.hello":       "Hello, {name}",
	"home.admin.placeholder": "The parent screens are coming soon.",
	"home.student.title":     "My cassettes",
	"home.student.hello":     "Hello, {name}!",
	"home.student.shelf":     "Your cassettes will appear here soon.",
	"home.logout":            "Log out",

	// Errors.
	"err.username_invalid":     "The username needs 2 to 32 characters: letters a-z, digits and _.",
	"err.display_name_invalid": "The name shown needs 1 to 40 characters.",
	"err.password_short":       "The password needs at least {min} characters.",
	"err.passwords_differ":     "The two passwords are not the same.",
	"err.username_taken":       "This username is already used.",
	"err.generic":              "Something went wrong. Ask a parent.",
	"err.generic_detail":       "Something went wrong: {message} ({tag})",

	// Notices for the parent.
	"notice.profiles_broken": "The profiles file could not be read. It was kept as {path} and Radio Tutor started with no profiles.",
	"notice.settings_broken": "The settings file could not be read. It was kept as {path} and Radio Tutor started with the default settings.",
	"notice.dismiss":         "OK",

	// The window shown when a second Radio Tutor is started.
	"already_running.title": "Radio Tutor is already running",
	"already_running.body":  "Look for its window, or close it before starting Radio Tutor again.",
	"already_running.ok":    "OK",
}

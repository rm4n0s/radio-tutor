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

	// The student area (SPEC §13).
	"student.shelf.empty":          "There are no cassettes yet. Ask a parent to add one.",
	"student.shelf.load_failed":    "The cassettes could not be shown. Ask a parent.",
	"student.progress":             "{done} of {total} done",
	"student.import":               "Import a cassette",
	"student.import.title":         "Choose a cassette file",
	"student.import.accept":        "Import",
	"student.import.filter":        "Cassettes (*.cassette)",
	"student.import.all_files":     "All files",
	"student.import.working":       "Adding the cassette...",
	"student.import.cancel":        "Cancel",
	"student.import.added":         "The cassette \"{title}\" was added.",
	"student.import.updated":       "The cassette \"{title}\" was updated to version {version}.",
	"student.import.already":       "This cassette is already installed.",
	"student.import.newer":         "A newer version ({version}) is installed.",
	"student.import.declined":      "The cassette was not changed.",
	"student.import.invalid_title": "This file is not a good cassette",
	"student.import.invalid_intro": "Nothing was added. These are the problems:",
	"student.import.more":          "... and {n} more",
	"student.import.ok":            "OK",
	"student.import.upgrade_title": "Update the cassette?",
	"student.import.upgrade_body":  "Version {old} is installed. Replace it with version {new}? The progress is kept.",
	"student.import.upgrade_yes":   "Update",
	"student.import.upgrade_no":    "Keep the old one",
	"student.import.failed":        "The cassette could not be added. Ask a parent.",
	"student.import.failed_detail": "The cassette could not be added: {message} ({tag})",
	"student.import.no_dialog":     "This computer has no file dialog, so a file cannot be chosen.",
	"student.cassette.continue":    "Continue",
	"student.cassette.lesson":      "Lesson",
	"student.cassette.worksheet":   "Worksheet {n}",
	"student.cassette.locked":      "Finish the items before this one first.",
	"student.cassette.load_failed": "This cassette could not be opened. Ask a parent.",
	"student.state.locked":         "Locked",
	"student.state.completed":      "Done",
	"student.state.passed":         "Passed",
	"student.state.needs_review":   "A parent will look at this",
	"student.soon.lesson":          "The lesson \"{title}\" will open here soon.",
	"student.soon.worksheet":       "The exercises of worksheet {n} will open here soon.",

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

	// The phone page (SPEC §10.1, §10.2).
	"phone.heading":          "Exercises",
	"phone.write_labels":     "Solve the exercises on paper. Write the number of each exercise next to its solution, then take photos of the pages.",
	"phone.take":             "Take photos of the solution",
	"phone.take_more":        "Take another photo",
	"phone.remove":           "Remove this photo",
	"phone.count":            "{n} of {max} photos",
	"phone.send":             "Send",
	"phone.sending":          "Sending...",
	"phone.sent":             "Sent! Look at the computer.",
	"phone.expired.title":    "This link has expired",
	"phone.expired.body":     "Go back to the computer and scan the new code.",
	"phone.not_found.title":  "Page not found",
	"phone.not_found.body":   "Go back to the computer and scan its code.",
	"phone.error.none":       "Take at least one photo first.",
	"phone.error.too_many":   "You can send at most {max} photos.",
	"phone.error.too_big":    "This photo is too big.",
	"phone.error.total_big":  "The photos are too big all together. Send fewer photos.",
	"phone.error.not_photo":  "Only photos can be sent",
	"phone.error.unreadable": "This photo could not be read",
	"phone.error.rate":       "Too many tries. Wait a minute and try again.",
	"phone.error.busy":       "The photos are being sent already.",
	"phone.error.network":    "The photos could not be sent. Check the Wi-Fi and try again.",
	"phone.error.server":     "Something went wrong. Try again.",
}

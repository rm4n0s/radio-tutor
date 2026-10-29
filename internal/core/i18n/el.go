package i18n

// messagesEL are the Greek messages. Keep the keys the same as in en.go.
var messagesEL = map[string]string{
	// The language switch.
	"lang.switch": "Γλώσσα",

	// First run: create the parent account.
	"firstrun.title":          "Δημιουργία λογαριασμού γονέα",
	"firstrun.intro":          "Καλώς ήρθατε! Ξεκινήστε δημιουργώντας τον λογαριασμό ενός γονέα. Οι γονείς προσθέτουν τα παιδιά και τις κασέτες.",
	"firstrun.create":         "Δημιουργία λογαριασμού",
	"field.username":          "Όνομα χρήστη",
	"field.username_hint":     "a-z, 0-9 και _ (2 έως 32)",
	"field.display_name":      "Όνομα εμφάνισης",
	"field.password":          "Κωδικός",
	"field.password_again":    "Επανάληψη του κωδικού",
	"field.password_min_hint": "Τουλάχιστον {min} χαρακτήρες",

	// Login.
	"login.title":     "Ποιος μαθαίνει σήμερα;",
	"login.password":  "{name}, γράψε τον κωδικό σου.",
	"login.enter":     "Είσοδος",
	"login.back":      "Πίσω",
	"login.wrong":     "Αυτός δεν είναι ο σωστός κωδικός. Δοκίμασε ξανά.",
	"login.throttled": "Πάρα πολλές λάθος προσπάθειες. Περίμενε {seconds} δευτερόλεπτα και δοκίμασε ξανά.",

	// The homes (placeholders until the next steps).
	"home.admin.title":       "Περιοχή γονέα",
	"home.admin.hello":       "Γεια σας, {name}",
	"home.admin.placeholder": "Οι οθόνες του γονέα έρχονται σύντομα.",
	"home.student.title":     "Οι κασέτες μου",
	"home.student.hello":     "Γεια σου, {name}!",
	"home.student.shelf":     "Οι κασέτες σου θα εμφανιστούν εδώ σύντομα.",
	"home.logout":            "Αποσύνδεση",

	// Errors.
	"err.username_invalid":     "Το όνομα χρήστη θέλει 2 έως 32 χαρακτήρες: γράμματα a-z, ψηφία και _.",
	"err.display_name_invalid": "Το όνομα εμφάνισης θέλει 1 έως 40 χαρακτήρες.",
	"err.password_short":       "Ο κωδικός θέλει τουλάχιστον {min} χαρακτήρες.",
	"err.passwords_differ":     "Οι δύο κωδικοί δεν είναι ίδιοι.",
	"err.username_taken":       "Αυτό το όνομα χρήστη χρησιμοποιείται ήδη.",
	"err.generic":              "Κάτι πήγε στραβά. Ρώτα έναν γονέα.",
	"err.generic_detail":       "Κάτι πήγε στραβά: {message} ({tag})",

	// Notices for the parent.
	"notice.profiles_broken": "Το αρχείο των προφίλ δεν μπόρεσε να διαβαστεί. Φυλάχθηκε ως {path} και το Radio Tutor ξεκίνησε χωρίς προφίλ.",
	"notice.settings_broken": "Το αρχείο των ρυθμίσεων δεν μπόρεσε να διαβαστεί. Φυλάχθηκε ως {path} και το Radio Tutor ξεκίνησε με τις προεπιλεγμένες ρυθμίσεις.",
	"notice.dismiss":         "Εντάξει",

	// The window shown when a second Radio Tutor is started.
	"already_running.title": "Το Radio Tutor εκτελείται ήδη",
	"already_running.body":  "Ψάξτε το παράθυρό του ή κλείστε το πριν ξεκινήσετε ξανά το Radio Tutor.",
	"already_running.ok":    "Εντάξει",
}

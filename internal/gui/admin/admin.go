// Package admin is the admin area of Radio Tutor (SPEC §12): the home with its
// tiles, the students (add, rename, reset password, delete, the cassettes each
// student can see), the cassettes (import, remove), the settings and the AI
// providers (add from presets, edit, set active, Test). The progress and review
// screens (step 6.6) plug in through Screens.
//
// The screens are widgets on Glutter. They read and write through the stores of
// app.Env on goroutines (State.Go) and hand the results back with SetState.
package admin

import (
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/phone"
)

// Keys of the widgets that tests and the later steps look for.
const (
	// KeyStudents is the key of the Students tile of the home.
	KeyStudents = "admin-students"
	// KeyCassettes is the key of the Cassettes tile of the home.
	KeyCassettes = "admin-cassettes"
	// KeyProgress is the key of the Progress tile of the home.
	KeyProgress = "admin-progress"
	// KeySettings is the key of the Settings tile of the home.
	KeySettings = "admin-settings"
	// KeyProviders is the key of the AI providers tile of the home.
	KeyProviders = "admin-providers"
	// KeyPlayAsMe is the key of the Play as me tile of the home.
	KeyPlayAsMe = "admin-play-as-me"
	// KeyReviewCount is the key of the text that says how many exercises wait
	// for review.
	KeyReviewCount = "review-count"

	// KeyAddStudent is the key of the Add button of the students screen.
	KeyAddStudent = "add-student"
	// KeyStudentPrefix starts the key of a student's row; the username follows.
	KeyStudentPrefix = "student-"
	// KeyRenamePrefix starts the key of the Rename button of a row; the
	// username follows.
	KeyRenamePrefix = "rename-"
	// KeyResetPrefix starts the key of the Reset password button of a row; the
	// username follows.
	KeyResetPrefix = "reset-"
	// KeyDeletePrefix starts the key of the Delete button of a row; the username
	// follows.
	KeyDeletePrefix = "delete-"
	// KeyVisiblePrefix starts the key of the Cassettes button of a student's
	// row (the username follows) that opens the cassettes the student can see.
	KeyVisiblePrefix = "visible-"
	// KeyShowPrefix starts the key of the checkbox of a cassette in that dialog;
	// the cassette id follows.
	KeyShowPrefix = "show-"
	// KeyRoleStudent is the key of the Student choice of the Add dialog.
	KeyRoleStudent = "role-student"
	// KeyRoleAdmin is the key of the Parent choice of the Add dialog.
	KeyRoleAdmin = "role-admin"

	// KeyCassettePrefix starts the key of a cassette's row; the cassette id
	// follows.
	KeyCassettePrefix = "cassette-"
	// KeyRemovePrefix starts the key of the Remove button of a cassette's row;
	// the cassette id follows.
	KeyRemovePrefix = "remove-"
	// KeyDeleteData is the key of the checkbox "Delete the progress and photos of
	// every student for this cassette" of the Remove dialog.
	KeyDeleteData = "delete-data"

	// KeyImportSwitch is the key of the "Students can import" switch.
	KeyImportSwitch = "students-can-import"
	// KeyPort is the key of the phone port field.
	KeyPort = "phone-port"
	// KeyPortSave is the key of the button that saves the phone port.
	KeyPortSave = "phone-port-save"
	// KeyInterface is the key of the phone interface dropdown.
	KeyInterface = "phone-interface"
	// KeyInterfaceAuto is the key of the Automatic entry of that dropdown.
	KeyInterfaceAuto = "interface-auto"
	// KeyInterfacePrefix starts the key of an interface's entry of that
	// dropdown; the interface name follows.
	KeyInterfacePrefix = "interface-"

	// KeyProviderAdd is the key of the Add button of the providers screen.
	KeyProviderAdd = "provider-add"
	// KeyProviderPrefix starts the key of a provider's row; the name follows.
	KeyProviderPrefix = "provider-"
	// KeyProviderTestPrefix starts the key of the Test button of a row; the name
	// follows.
	KeyProviderTestPrefix = "provider-test-"
	// KeyProviderActivePrefix starts the key of the Set active button of a row;
	// the name follows.
	KeyProviderActivePrefix = "provider-active-"
	// KeyProviderEditPrefix starts the key of the Edit button of a row; the name
	// follows.
	KeyProviderEditPrefix = "provider-edit-"
	// KeyProviderDeletePrefix starts the key of the Delete button of a row; the
	// name follows.
	KeyProviderDeletePrefix = "provider-delete-"
	// KeyPresetPrefix starts the key of a preset of the Add screen; the preset id
	// follows.
	KeyPresetPrefix = "preset-"
	// KeyPresetCustom is the key of the "Other server" choice of the Add screen.
	KeyPresetCustom = "preset-custom"
	// KeyProviderName is the key of the name field of the provider form.
	KeyProviderName = "provider-name"
	// KeyProviderURL is the key of the base URL field of the provider form.
	KeyProviderURL = "provider-url"
	// KeyProviderModel is the key of the model field of the provider form.
	KeyProviderModel = "provider-model"
	// KeyProviderAPIKey is the key of the API key field of the provider form.
	KeyProviderAPIKey = "provider-api-key"
	// KeyProviderTimeout is the key of the timeout field of the provider form.
	KeyProviderTimeout = "provider-timeout"
	// KeyKindCloud is the key of the Cloud choice of the provider form.
	KeyKindCloud = "kind-cloud"
	// KeyKindLocal is the key of the Local choice of the provider form.
	KeyKindLocal = "kind-local"
	// KeyModePrefix starts the key of a structured output choice of the provider
	// form; the mode (json_schema, json_object, none) follows.
	KeyModePrefix = "mode-"
	// KeyTestResult is the key of the text that shows the result of a Test.
	KeyTestResult = "test-result"

	// KeyDialogOK is the key of the button that confirms a dialog (Add, Rename,
	// Reset, Delete, Remove, OK).
	KeyDialogOK = "dialog-ok"
	// KeyDialogCancel is the key of the button that cancels a dialog.
	KeyDialogCancel = "dialog-cancel"
)

// Screens are the screens the admin area opens. A nil function gives a
// placeholder that says the screen comes soon; step 6.6 provides the progress
// screen.
type Screens struct {
	// Progress builds the progress and review screen (SPEC §12.3).
	Progress func() m.Widget
	// Interfaces lists the network interfaces for the phone interface setting;
	// nil means phone.ListInterfaces (tests give fixed ones).
	Interfaces func() ([]phone.Interface, error)
}

// HomeBuilder returns the function the shell calls for the home of an admin
// (app.Screens.Admin).
func HomeBuilder(screens Screens) func() m.Widget {
	return func() m.Widget { return NewHome(screens) }
}

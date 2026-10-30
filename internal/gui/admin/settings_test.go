package admin

import (
	"context"
	"os"
	"testing"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/glutter/gluttertest"
	"github.com/rm4n0s/radio-tutor/internal/core/phone"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

func openSettings(t *testing.T, a *testutil.App) {
	t.Helper()
	a.TapKey(KeySettings)
	waitKey(t, a, KeyPort)
}

func TestSettingsInterfaceDropdown(t *testing.T) {
	a := pump(t, config{})
	openSettings(t, a)
	if !has(a, "Automatic") {
		t.Fatal("the dropdown does not start at Automatic")
	}
	a.TapKey(KeyInterface)
	a.PumpAndSettle()
	if a.Exists(gluttertest.ByTextContaining("127.0.0.1")) {
		t.Error("the loopback interface is offered")
	}
	a.TapKey(KeyInterfacePrefix + "wlan0")
	tillTrue(t, a, "wlan0 to be saved", func() bool { return a.Env.Settings.Get().Phone.Interface == "wlan0" })
	waitText(t, a, "Saved.")
	if !has(a, "wlan0 (192.168.1.20)") {
		t.Error("the dropdown does not show the chosen interface")
	}

	a.TapKey(KeyInterface)
	a.PumpAndSettle()
	a.TapKey(KeyInterfaceAuto)
	tillTrue(t, a, "automatic to be saved", func() bool { return a.Env.Settings.Get().Phone.Interface == "" })
}

func TestSettingsKeepsAnInterfaceThatIsGone(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		err := env.Settings.Update(context.Background(), func(s *settings.Settings) { s.Phone.Interface = "usb0" })
		if err != nil {
			t.Fatal(err)
		}
	}})
	openSettings(t, a)
	if !has(a, "usb0") {
		t.Error("the interface that was chosen is not shown")
	}
}

func TestSettingsWhenInterfacesCannotBeListed(t *testing.T) {
	a := pump(t, config{screens: Screens{Interfaces: func() ([]phone.Interface, error) {
		return nil, errors.New("TestInterfacesFailed", "no interfaces")
	}}})
	openSettings(t, a)
	if !has(a, "Automatic") {
		t.Error("Automatic is not offered")
	}
}

func TestSettingsStudentsCanImport(t *testing.T) {
	a := pump(t, config{})
	openSettings(t, a)
	if !a.Env.Settings.Get().StudentsCanImport {
		t.Fatal("students cannot import at first")
	}
	a.TapKey(KeyImportSwitch)
	tillTrue(t, a, "the switch to be saved", func() bool { return !a.Env.Settings.Get().StudentsCanImport })
	a.TapKey(KeyImportSwitch)
	tillTrue(t, a, "the switch to be saved again", func() bool { return a.Env.Settings.Get().StudentsCanImport })
}

func TestSettingsPort(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int // the saved port; 0 means an error is shown
	}{
		{"valid", "9000", 9000},
		{"with spaces", " 9001 ", 9001},
		{"lowest", "1024", 1024},
		{"highest", "65535", 65535},
		{"below 1024", "80", 0},
		{"too big", "65536", 0},
		{"not a number", "eight", 0},
		{"empty", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pump(t, config{})
			openSettings(t, a)
			a.EnterKey(KeyPort, tt.text)
			a.TapKey(KeyPortSave)
			if tt.want == 0 {
				waitText(t, a, "The port must be a number from 1024 to 65535.")
				if got := a.Env.Settings.Get().Phone.Port; got != settings.DefaultPhonePort {
					t.Errorf("port = %d after an invalid one", got)
				}
				return
			}
			tillTrue(t, a, "the port to be saved", func() bool { return a.Env.Settings.Get().Phone.Port == tt.want })
			waitText(t, a, "Saved.")
			if a.HasKey(app.KeyError) {
				t.Error("an error is shown after a good port")
			}
		})
	}
}

func TestSettingsPortSavesOnEnterAndClearsTheError(t *testing.T) {
	a := pump(t, config{})
	openSettings(t, a)
	a.EnterKey(KeyPort, "1")
	a.TapKey(KeyPortSave)
	waitKey(t, a, app.KeyError)
	a.EnterKey(KeyPort, "9100")
	a.TapKey(KeyPortSave)
	tillTrue(t, a, "the port", func() bool { return a.Env.Settings.Get().Phone.Port == 9100 })
	waitGone(t, a, app.KeyError)
}

func TestSettingsLanguage(t *testing.T) {
	a := pump(t, config{})
	openSettings(t, a)
	a.TapKey(app.KeyLanguagePrefix + "el")
	waitText(t, a, "Οι μαθητές μπορούν να εισάγουν κασέτες")
	tillTrue(t, a, "the language to be saved", func() bool { return a.Env.Settings.Get().Language == "el" })
	a.TapKey(app.KeyLanguagePrefix + "en")
	waitText(t, a, "Students can import cassettes")
}

func TestSettingsSaveFailureIsShown(t *testing.T) {
	a := pump(t, config{})
	openSettings(t, a)
	// A settings file that cannot be written: a folder where the file goes.
	path := a.Env.Paths.SettingsFile()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	a.TapKey(KeyImportSwitch)
	waitKey(t, a, app.KeyError)
	if !a.Env.Settings.Get().StudentsCanImport {
		t.Error("the setting changed although it could not be saved")
	}
}

package admin

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/glutter/gluttertest"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/glutter/ui"
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

// thumb is where the thumb of the volume slider is when the volume is 100: the
// right end of the track, which is inset by the 24 pixels of the thumb's overlay.
func thumb(a *testutil.App) ui.Offset {
	br := a.BottomRight(key(KeyVolume))
	return ui.Offset{Dx: br.Dx - 24, Dy: a.Center(key(KeyVolume)).Dy}
}

// volumeText is the text that shows the volume.
func volumeText(a *testutil.App) string {
	return gluttertest.WidgetAs[*m.Text](a.Tester, key(KeyVolumeValue)).Data().OrElse("")
}

func TestSettingsVolumeStartsAtTheSavedValue(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		if err := env.Settings.Update(context.Background(), func(s *settings.Settings) { s.Volume = 35 }); err != nil {
			t.Fatal(err)
		}
	}})
	openSettings(t, a)
	if got := volumeText(a); got != "35%" {
		t.Errorf("the volume reads %q, want 35%%", got)
	}
}

func TestSettingsVolumeSlider(t *testing.T) {
	tests := []struct {
		name string
		by   float64 // the drag, in logical pixels; the slider starts at 100
		want func(v int) bool
	}{
		{"all the way to the left is silence", -2000, func(v int) bool { return v == 0 }},
		{"part of the way is a step of 5", -150, func(v int) bool { return v > 0 && v < 100 && v%5 == 0 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pump(t, config{})
			openSettings(t, a)
			if got := a.Env.Settings.Get().Volume; got != 100 {
				t.Fatalf("the volume starts at %d, want 100", got)
			}
			if got := volumeText(a); got != "100%" {
				t.Fatalf("the volume reads %q at first", got)
			}
			a.DragFrom(thumb(a), ui.Offset{Dx: tt.by})
			a.PumpAndSettle()
			tillTrue(t, a, "the volume to be saved", func() bool { return tt.want(a.Env.Settings.Get().Volume) })
			saved := a.Env.Settings.Get().Volume
			waitText(t, a, "Saved.")
			if got, want := volumeText(a), itoa(saved)+"%"; got != want {
				t.Errorf("the volume reads %q, want %q", got, want)
			}
		})
	}
}

func TestSettingsVolumeIsSavedWhenTheDragEnds(t *testing.T) {
	a := pump(t, config{})
	openSettings(t, a)
	g := a.StartGesture(thumb(a))
	g.MoveBy(ui.Offset{Dx: -150})
	a.Pump(50 * time.Millisecond)
	if got := a.Env.Settings.Get().Volume; got != 100 {
		t.Errorf("the volume was saved while the thumb was still down: %d", got)
	}
	g.Up()
	a.PumpAndSettle()
	tillTrue(t, a, "the volume to be saved", func() bool { return a.Env.Settings.Get().Volume < 100 })
}

// muteSwitchOn reports whether the mute switch is on.
func muteSwitchOn(a *testutil.App) bool {
	return gluttertest.WidgetAs[*m.Switch](a.Tester, gluttertest.Descendant(key(KeyMuteSwitch), gluttertest.ByType[*m.Switch]())).Value()
}

func TestSettingsShowWhatTheLessonPlayerSaved(t *testing.T) {
	// The lesson player saves the volume and the mute in the same settings.
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		if err := env.Settings.Update(context.Background(), func(s *settings.Settings) { s.Volume, s.Muted = 35, true }); err != nil {
			t.Fatal(err)
		}
	}})
	openSettings(t, a)
	if got := volumeText(a); got != "35%" {
		t.Errorf("the volume reads %q, want 35%%", got)
	}
	if !muteSwitchOn(a) {
		t.Error("the mute switch is off although the sound is muted")
	}
}

func TestSettingsMuteSwitchKeepsTheVolume(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		if err := env.Settings.Update(context.Background(), func(s *settings.Settings) { s.Volume = 35 }); err != nil {
			t.Fatal(err)
		}
	}})
	openSettings(t, a)
	if muteSwitchOn(a) {
		t.Fatal("the mute switch starts on")
	}
	a.TapKey(KeyMuteSwitch)
	tillTrue(t, a, "the mute to be saved", func() bool { return a.Env.Settings.Get().Muted })
	if got := a.Env.Settings.Get().Volume; got != 35 {
		t.Errorf("muting changed the volume to %d", got)
	}
	waitText(t, a, "Saved.")
	if !muteSwitchOn(a) || volumeText(a) != "35%" {
		t.Errorf("the screen shows mute %v and volume %q", muteSwitchOn(a), volumeText(a))
	}
	a.TapKey(KeyMuteSwitch)
	tillTrue(t, a, "the unmute to be saved", func() bool { return !a.Env.Settings.Get().Muted })
	if got := a.Env.Settings.Get().Volume; got != 35 {
		t.Errorf("unmuting changed the volume to %d", got)
	}
}

func TestSettingsMovingTheSliderWhileMutedUnmutes(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		if err := env.Settings.Update(context.Background(), func(s *settings.Settings) { s.Muted = true }); err != nil {
			t.Fatal(err)
		}
	}})
	openSettings(t, a)
	a.DragFrom(thumb(a), ui.Offset{Dx: -150})
	a.PumpAndSettle()
	tillTrue(t, a, "the volume to be saved", func() bool { return a.Env.Settings.Get().Volume < 100 })
	if a.Env.Settings.Get().Muted {
		t.Error("the sound is still muted after the slider moved")
	}
	if muteSwitchOn(a) {
		t.Error("the mute switch is still on after the slider moved")
	}
}

package settings

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
)

func TestDefaults(t *testing.T) {
	tests := []struct {
		locale string
		want   i18n.Lang
	}{{"", i18n.EN}, {"en_US.UTF-8", i18n.EN}, {"el_GR.UTF-8", i18n.EL}, {"de_DE", i18n.EN}}
	for _, tt := range tests {
		s := Defaults(tt.locale)
		want := Settings{Format: 1, Language: tt.want, StudentsCanImport: true, Phone: Phone{Port: 8737}, HiddenCassettes: map[string][]string{}}
		if !reflect.DeepEqual(s, want) {
			t.Errorf("Defaults(%q) = %+v, want %+v", tt.locale, s, want)
		}
	}
}

func TestOpen(t *testing.T) {
	tests := []struct {
		name   string
		file   string // content, or "-" for no file
		locale string
		want   func(*Settings)
		route  string
	}{
		{"missing file gives defaults", "-", "el_GR", func(s *Settings) { s.Language = i18n.EL }, ""},
		{"empty object keeps defaults", `{"format":1}`, "", func(*Settings) {}, ""},
		{"fields override defaults", `{"format":1,"language":"el","students_can_import":false,"phone":{"port":9000,"interface":"wlan0"},"hidden_cassettes":{"mia":["math"]}}`, "",
			func(s *Settings) {
				s.Language, s.StudentsCanImport = i18n.EL, false
				s.Phone = Phone{Port: 9000, Interface: "wlan0"}
				s.HiddenCassettes = map[string][]string{"mia": {"math"}}
			}, ""},
		{"missing phone port keeps the default", `{"format":1,"phone":{"interface":"eth0"}}`, "", func(s *Settings) { s.Phone.Interface = "eth0" }, ""},
		{"format 2", `{"format":2}`, "", nil, "Open->validate.SettingsFormatUnsupported"},
		{"format zero", `{"format":0}`, "", nil, "Open->validate.SettingsFormatUnsupported"},
		{"language fr", `{"format":1,"language":"fr"}`, "", nil, "Open->validate.SettingsLanguageInvalid"},
		{"port 0", `{"format":1,"phone":{"port":0}}`, "", nil, "Open->validate.SettingsPortInvalid"},
		{"port too big", `{"format":1,"phone":{"port":70000}}`, "", nil, "Open->validate.SettingsPortInvalid"},
		{"not json", `{nope`, "", nil, "Open->ReadJSON.JSONInvalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if tt.file != "-" {
				if err := os.WriteFile(path, []byte(tt.file), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			st, err := Open(context.Background(), path, tt.locale)
			if tt.route != "" {
				if err == nil {
					t.Fatal("expected an error")
				}
				if appErr, _ := errors.FromError(err); appErr == nil || !appErr.HasRoute(tt.route) {
					t.Fatalf("unexpected failure path: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := Defaults(tt.locale)
			tt.want(&want)
			if got := st.Get(); !reflect.DeepEqual(got, want) {
				t.Errorf("settings = %+v, want %+v", got, want)
			}
		})
	}
}

func TestOpenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Open(ctx, filepath.Join(t.TempDir(), "settings.json"), "")
	if appErr, _ := errors.FromError(err); appErr == nil || !appErr.HasRoute("Open.SettingsOpenCancelled") {
		t.Fatalf("unexpected failure path: %v", err)
	}
}

func TestUpdatePersistsAndValidates(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sub", "settings.json")
	st, err := Open(ctx, path, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("Open must not write the file")
	}
	if err := st.SetLanguage(ctx, i18n.EL); err != nil {
		t.Fatal(err)
	}
	if err := st.Update(ctx, func(s *Settings) { s.HiddenCassettes["mia"] = []string{"a", "b"} }); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != storage.FileMode {
		t.Errorf("mode = %v", info.Mode().Perm())
	}
	again, err := Open(ctx, path, "")
	if err != nil {
		t.Fatal(err)
	}
	got := again.Get()
	if got.Language != i18n.EL || !reflect.DeepEqual(got.HiddenCassettes["mia"], []string{"a", "b"}) {
		t.Errorf("reopened = %+v", got)
	}

	tests := []struct {
		name   string
		change func(*Settings)
		route  string
	}{
		{"language", func(s *Settings) { s.Language = "fr" }, "Update->validate.SettingsLanguageInvalid"},
		{"port", func(s *Settings) { s.Phone.Port = 0 }, "Update->validate.SettingsPortInvalid"},
		{"format", func(s *Settings) { s.Format = 7 }, "Update->validate.SettingsFormatUnsupported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := st.Update(ctx, tt.change)
			if appErr, _ := errors.FromError(err); appErr == nil || !appErr.HasRoute(tt.route) {
				t.Fatalf("unexpected failure path: %v", err)
			}
			if st.Get().Language != i18n.EL {
				t.Error("a refused change reached the settings")
			}
		})
	}
	if err := st.SetLanguage(ctx, "xx"); err == nil {
		t.Error("SetLanguage accepted xx")
	}
}

func TestUpdateWriteFailureKeepsSettings(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	st, err := Open(ctx, filepath.Join(t.TempDir(), "settings.json"), "")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	err = st.SetLanguage(ctx, i18n.EL)
	if appErr, _ := errors.FromError(err); appErr == nil || !appErr.HasRoute("Update->WriteJSONAtomic->writeJSONAtomic.WriteCancelled") {
		t.Fatalf("unexpected failure path: %v", err)
	}
	if st.Get().Language != i18n.EN {
		t.Error("a failed write changed the settings")
	}
}

func TestGetReturnsACopy(t *testing.T) {
	st, err := Open(context.Background(), filepath.Join(t.TempDir(), "settings.json"), "")
	if err != nil {
		t.Fatal(err)
	}
	st.Get().HiddenCassettes["x"] = []string{"y"}
	if len(st.Get().HiddenCassettes) != 0 {
		t.Error("Get shares its map")
	}
}

package storage

import (
	"path/filepath"
	"testing"
)

func TestPathsFiles(t *testing.T) {
	p := Paths{Config: filepath.Join("c", "radio-tutor"), Data: filepath.Join("d", "radio-tutor"), State: filepath.Join("s", "radio-tutor")}
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"settings", p.SettingsFile(), filepath.Join("c", "radio-tutor", "settings.json")},
		{"providers", p.ProvidersFile(), filepath.Join("c", "radio-tutor", "providers.json")},
		{"profiles", p.ProfilesFile(), filepath.Join("d", "radio-tutor", "profiles.json")},
		{"cassettes", p.CassettesDir(), filepath.Join("d", "radio-tutor", "cassettes")},
		{"progress", p.ProgressDir(), filepath.Join("d", "radio-tutor", "progress")},
		{"photos", p.PhotosDir(), filepath.Join("d", "radio-tutor", "photos")},
		{"log", p.LogFile(), filepath.Join("s", "radio-tutor", "radio-tutor.log")},
		{"lock", p.LockFile(), filepath.Join("s", "radio-tutor", "lock")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

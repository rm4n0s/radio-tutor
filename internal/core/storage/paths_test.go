package storage

import (
	"path/filepath"
	"testing"

	"github.com/rm4n0s/errors"
)

func TestResolveFrom(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want Paths
	}{
		{
			name: "defaults from HOME",
			env:  map[string]string{"HOME": "/home/kid"},
			want: Paths{
				Config: "/home/kid/.config/radio-tutor",
				Data:   "/home/kid/.local/share/radio-tutor",
				State:  "/home/kid/.local/state/radio-tutor",
			},
		},
		{
			name: "all variables set",
			env: map[string]string{
				"XDG_CONFIG_HOME": "/c", "XDG_DATA_HOME": "/d", "XDG_STATE_HOME": "/s",
			},
			want: Paths{Config: "/c/radio-tutor", Data: "/d/radio-tutor", State: "/s/radio-tutor"},
		},
		{
			name: "variables win over HOME, mixed with defaults",
			env:  map[string]string{"HOME": "/home/kid", "XDG_DATA_HOME": "/d"},
			want: Paths{
				Config: "/home/kid/.config/radio-tutor",
				Data:   "/d/radio-tutor",
				State:  "/home/kid/.local/state/radio-tutor",
			},
		},
		{
			name: "empty and relative variables are ignored",
			env: map[string]string{
				"HOME": "/home/kid", "XDG_CONFIG_HOME": "", "XDG_DATA_HOME": "relative/dir", "XDG_STATE_HOME": "./x",
			},
			want: Paths{
				Config: "/home/kid/.config/radio-tutor",
				Data:   "/home/kid/.local/share/radio-tutor",
				State:  "/home/kid/.local/state/radio-tutor",
			},
		},
		{
			name: "trailing slash is cleaned",
			env:  map[string]string{"XDG_CONFIG_HOME": "/c/", "XDG_DATA_HOME": "/d/", "XDG_STATE_HOME": "/s/"},
			want: Paths{Config: "/c/radio-tutor", Data: "/d/radio-tutor", State: "/s/radio-tutor"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveFrom(func(k string) string { return tt.env[k] })
			if err != nil {
				t.Fatalf("ResolveFrom: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestResolveFromHomeUnknown(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"nothing set", map[string]string{}},
		{"relative HOME", map[string]string{"HOME": "home/kid"}},
		{"only some variables set", map[string]string{"XDG_CONFIG_HOME": "/c", "XDG_DATA_HOME": "/d"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolveFrom(func(k string) string { return tt.env[k] })
			appErr, ok := errors.FromError(err)
			if !ok {
				t.Fatalf("expected *errors.Error, got %T: %v", err, err)
			}
			if !appErr.HasRoute("ResolveFrom->ResolveFrom.func1.HomeUnknown") {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
		})
	}
}

func TestResolveReadsEnvironment(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/c")
	t.Setenv("XDG_DATA_HOME", "/d")
	t.Setenv("XDG_STATE_HOME", "/s")
	got, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := Paths{Config: "/c/radio-tutor", Data: "/d/radio-tutor", State: "/s/radio-tutor"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestPathsFiles(t *testing.T) {
	p := Paths{Config: "/c/radio-tutor", Data: "/d/radio-tutor", State: "/s/radio-tutor"}
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"settings", p.SettingsFile(), "/c/radio-tutor/settings.json"},
		{"providers", p.ProvidersFile(), "/c/radio-tutor/providers.json"},
		{"profiles", p.ProfilesFile(), "/d/radio-tutor/profiles.json"},
		{"cassettes", p.CassettesDir(), "/d/radio-tutor/cassettes"},
		{"progress", p.ProgressDir(), "/d/radio-tutor/progress"},
		{"photos", p.PhotosDir(), "/d/radio-tutor/photos"},
		{"log", p.LogFile(), "/s/radio-tutor/radio-tutor.log"},
		{"lock", p.LockFile(), filepath.Join("/s/radio-tutor", "lock")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

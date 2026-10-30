//go:build windows

package storage

import (
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
			name: "LOCALAPPDATA",
			env:  map[string]string{"LOCALAPPDATA": `C:\Users\kid\AppData\Local`},
			want: Paths{
				Config: `C:\Users\kid\AppData\Local\radio-tutor\config`,
				Data:   `C:\Users\kid\AppData\Local\radio-tutor\data`,
				State:  `C:\Users\kid\AppData\Local\radio-tutor\state`,
			},
		},
		{
			name: "LOCALAPPDATA wins over USERPROFILE, trailing separator cleaned",
			env:  map[string]string{"LOCALAPPDATA": `D:\L\`, "USERPROFILE": `C:\Users\kid`, "APPDATA": `C:\Users\kid\AppData\Roaming`},
			want: Paths{Config: `D:\L\radio-tutor\config`, Data: `D:\L\radio-tutor\data`, State: `D:\L\radio-tutor\state`},
		},
		{
			name: "USERPROFILE when LOCALAPPDATA is empty or relative",
			env:  map[string]string{"LOCALAPPDATA": `rel\dir`, "USERPROFILE": `C:\Users\kid`},
			want: Paths{
				Config: `C:\Users\kid\AppData\Local\radio-tutor\config`,
				Data:   `C:\Users\kid\AppData\Local\radio-tutor\data`,
				State:  `C:\Users\kid\AppData\Local\radio-tutor\state`,
			},
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

func TestResolveFromLocalAppDataUnknown(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"nothing set", map[string]string{}},
		{"relative variables", map[string]string{"LOCALAPPDATA": `a`, "USERPROFILE": `b`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolveFrom(func(k string) string { return tt.env[k] })
			appErr, ok := errors.FromError(err)
			if !ok {
				t.Fatalf("expected *errors.Error, got %T: %v", err, err)
			}
			if !appErr.HasRoute("ResolveFrom.LocalAppDataUnknown") {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
		})
	}
}

func TestResolveReadsEnvironment(t *testing.T) {
	t.Setenv("LOCALAPPDATA", `C:\L`)
	got, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := Paths{Config: `C:\L\radio-tutor\config`, Data: `C:\L\radio-tutor\data`, State: `C:\L\radio-tutor\state`}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

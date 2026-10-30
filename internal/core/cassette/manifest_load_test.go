package cassette

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/problem"
)

func TestValidIDs(t *testing.T) {
	tests := []struct {
		s          string
		id, cassID bool
	}{
		{"what-is-a-fraction", true, false},
		{"a", true, false},
		{"org.family.math-1", false, true},
		{"org", true, false},
		{"", false, false},
		{"../x", false, false},
		{"a/b", false, false},
		{"Upper", false, false},
		{"org.x_tmp-1", false, false},
		{"-a", false, false},
		{"con", false, false},
		{"nul", false, false},
		{"com1", false, false},
		{"lpt9", false, false},
		{"aux", false, false},
		{"con.family.math", false, false},
		{"aux.example.x", false, false},
		{"org.example.con", false, true},
		{"console", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			if got := ValidID(tt.s); got != tt.id {
				t.Errorf("ValidID(%q) = %v", tt.s, got)
			}
			if got := ValidCassetteID(tt.s); got != tt.cassID {
				t.Errorf("ValidCassetteID(%q) = %v", tt.s, got)
			}
		})
	}
}

func TestLoadManifest(t *testing.T) {
	tests := []struct {
		name    string
		mut     func(t *testing.T, fsys fstest.MapFS)
		nilMan  bool
		wantErr bool // problems have errors
	}{
		{"valid", func(t *testing.T, fsys fstest.MapFS) {}, false, false},
		{"lessons are not read", func(t *testing.T, fsys fstest.MapFS) { delete(fsys, lesson1) }, false, false},
		{"missing", func(t *testing.T, fsys fstest.MapFS) { delete(fsys, manifest) }, true, true},
		{"bad shape", setRaw(manifest, "[]"), true, true},
		{"newer format", setRaw(manifest, `{"format": 2}`), true, true},
		{"bad field rule", func(t *testing.T, fsys fstest.MapFS) {
			edit(t, fsys, manifest, func(root map[string]any) { root["title"] = "" })
		}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := minimalFS(t)
			tt.mut(t, fsys)
			m, ps, err := LoadManifest(context.Background(), fsys)
			if err != nil {
				t.Fatal(err)
			}
			if (m == nil) != tt.nilMan {
				t.Errorf("manifest nil = %v, want %v (%v)", m == nil, tt.nilMan, ps)
			}
			if got := problem.HasErrors(ps); got != tt.wantErr {
				t.Errorf("errors = %v, want %v: %v", got, tt.wantErr, ps)
			}
			if !tt.nilMan && m.ID != "org.example.minimal-1" {
				t.Errorf("id = %q", m.ID)
			}
		})
	}

	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, _, err := LoadManifest(ctx, minimalFS(t))
		appErr, ok := errors.FromError(err)
		if !ok || !appErr.HasRoute("LoadManifest->loader.document->loader.readFile.CassetteLoadCanceled") {
			t.Errorf("unexpected failure: %v", err)
		}
	})
}

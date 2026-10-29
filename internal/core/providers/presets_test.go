package providers

import (
	"strings"
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/grading"
)

func TestPresets(t *testing.T) {
	want := map[string]struct {
		baseURL string
		kind    Kind
		mode    grading.StructuredOutput
		key     bool
	}{
		"openai":     {"https://api.openai.com/v1", KindCloud, grading.StructuredJSONSchema, true},
		"ollama":     {"http://localhost:11434/v1", KindLocal, grading.StructuredJSONSchema, false},
		"lmstudio":   {"http://localhost:1234/v1", KindLocal, grading.StructuredJSONSchema, false},
		"llamacpp":   {"http://localhost:8080/v1", KindLocal, grading.StructuredJSONSchema, false},
		"openrouter": {"https://openrouter.ai/api/v1", KindCloud, grading.StructuredJSONSchema, true},
		"gemini":     {"https://generativelanguage.googleapis.com/v1beta/openai", KindCloud, grading.StructuredJSONSchema, true},
		"anthropic":  {"https://api.anthropic.com/v1/", KindCloud, grading.StructuredJSONObject, true},
	}
	presets := Presets()
	if len(presets) != len(want) {
		t.Fatalf("%d presets, want %d", len(presets), len(want))
	}
	names := map[string]bool{}
	for _, p := range presets {
		t.Run(p.ID, func(t *testing.T) {
			w, ok := want[p.ID]
			if !ok {
				t.Fatalf("unexpected preset %q", p.ID)
			}
			if p.BaseURL != w.baseURL || p.Kind != w.kind || p.StructuredOutput != w.mode || p.NeedsAPIKey != w.key {
				t.Errorf("preset = %+v, want %+v", p, w)
			}
			if names[p.Name] {
				t.Errorf("name %q used twice", p.Name)
			}
			names[p.Name] = true
			if p.Model == "" || strings.TrimSpace(p.Description) == "" {
				t.Errorf("model %q or description %q empty", p.Model, p.Description)
			}
			c := p.Config()
			if c.APIKey != "" {
				t.Error("a preset has no API key")
			}
			if _, err := validate(c); err != nil {
				t.Errorf("the preset's config is not valid: %v", err)
			}
			if _, _, err := Build(c, nil); err != nil {
				t.Errorf("the preset does not build: %v", err)
			}
		})
	}
	if p := presets[len(presets)-1]; p.ID != "anthropic" || !strings.Contains(p.Description, "json_object") {
		t.Errorf("the Anthropic preset must explain json_object: %+v", p)
	}
	if got := presets[1].Model; got != "qwen2.5vl:7b" {
		t.Errorf("Ollama model = %q", got)
	}
	// Each call returns its own slice.
	presets[0].Name = "changed"
	if Presets()[0].Name == "changed" {
		t.Error("Presets shares its slice between calls")
	}
}

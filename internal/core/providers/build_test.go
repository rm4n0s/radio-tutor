package providers

import (
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/grading/openai"
)

func TestBuild(t *testing.T) {
	tests := []struct {
		mode        grading.StructuredOutput
		wantMode    grading.StructuredOutput
		wantLenient bool
	}{
		{grading.StructuredJSONSchema, grading.StructuredJSONSchema, false},
		{grading.StructuredJSONObject, grading.StructuredJSONObject, true},
		{grading.StructuredNone, grading.StructuredNone, true},
		{"", grading.StructuredJSONSchema, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			c := validConfig("Local")
			c.StructuredOutput = tt.mode
			p, lenient, err := Build(c, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			if lenient != tt.wantLenient {
				t.Errorf("lenient = %v, want %v", lenient, tt.wantLenient)
			}
			if p.Name() != "Local" {
				t.Errorf("Name = %q", p.Name())
			}
			op, ok := p.(*openai.Provider)
			if !ok {
				t.Fatalf("provider is %T, want *openai.Provider", p)
			}
			if op.StructuredOutput() != tt.wantMode {
				t.Errorf("mode = %q, want %q", op.StructuredOutput(), tt.wantMode)
			}
		})
	}
}

func TestBuildErrors(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Config)
		route  string
	}{
		{"name", func(c *Config) { c.Name = "" }, "Build->normalize.ProviderNameInvalid"},
		{"protocol", func(c *Config) { c.Protocol = "grpc" }, "Build->normalize.ProviderProtocolUnsupported"},
		{"kind", func(c *Config) { c.Kind = "" }, "Build->normalize.ProviderKindInvalid"},
		{"timeout", func(c *Config) { c.TimeoutSeconds = -5 }, "Build->normalize.ProviderTimeoutInvalid"},
		{"model", func(c *Config) { c.Model = "" }, "Build->New.OpenAIConfigModel"},
		{"base url", func(c *Config) { c.BaseURL = "localhost:11434" }, "Build->New.OpenAIConfigBaseURL"},
		{"structured output", func(c *Config) { c.StructuredOutput = "xml" }, "Build->New.OpenAIConfigStructuredOutput"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validConfig("Local")
			tt.change(&c)
			p, lenient, err := Build(c, "", nil)
			if p != nil || lenient {
				t.Errorf("Build returned a provider (%v) or lenient (%v) with an error", p, lenient)
			}
			wantRoute(t, err, tt.route)
		})
	}
}

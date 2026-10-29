package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/grading"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Config)
		route  string // empty: valid
		check  func(*testing.T, Config)
	}{
		{"valid as given", func(*Config) {}, "", nil},
		{"defaults are filled", func(c *Config) { c.Protocol, c.TimeoutSeconds, c.StructuredOutput = "", 0, "" }, "",
			func(t *testing.T, c Config) {
				if c.Protocol != "openai" || c.TimeoutSeconds != 180 || c.StructuredOutput != grading.StructuredJSONSchema {
					t.Errorf("defaults not filled: %+v", c)
				}
			}},
		{"text is trimmed", func(c *Config) { c.Name, c.Model, c.BaseURL, c.APIKey = " A ", " m ", " http://h/v1 ", " k\n" }, "",
			func(t *testing.T, c Config) {
				if c.Name != "A" || c.Model != "m" || c.BaseURL != "http://h/v1" || c.APIKey != "k" {
					t.Errorf("not trimmed: %+v", c)
				}
			}},
		{"empty key is fine", func(c *Config) { c.APIKey = "" }, "", nil},
		{"cloud kind", func(c *Config) { c.Kind = KindCloud }, "", nil},
		{"empty name", func(c *Config) { c.Name = "  " }, "validate->normalize.ProviderNameInvalid", nil},
		{"long name", func(c *Config) { c.Name = strings.Repeat("x", 61) }, "validate->normalize.ProviderNameInvalid", nil},
		{"multi-line name", func(c *Config) { c.Name = "a\nb" }, "validate->normalize.ProviderNameInvalid", nil},
		{"protocol anthropic", func(c *Config) { c.Protocol = "anthropic" }, "validate->normalize.ProviderProtocolUnsupported", nil},
		{"kind missing", func(c *Config) { c.Kind = "" }, "validate->normalize.ProviderKindInvalid", nil},
		{"kind other", func(c *Config) { c.Kind = "lan" }, "validate->normalize.ProviderKindInvalid", nil},
		{"negative timeout", func(c *Config) { c.TimeoutSeconds = -1 }, "validate->normalize.ProviderTimeoutInvalid", nil},
		{"huge timeout", func(c *Config) { c.TimeoutSeconds = 3601 }, "validate->normalize.ProviderTimeoutInvalid", nil},
		{"max timeout", func(c *Config) { c.TimeoutSeconds = 3600 }, "", nil},
		{"no model", func(c *Config) { c.Model = "" }, "validate->New.OpenAIConfigModel", nil},
		{"base url with ftp", func(c *Config) { c.BaseURL = "ftp://h/v1" }, "validate->New.OpenAIConfigBaseURL", nil},
		{"base url empty", func(c *Config) { c.BaseURL = "" }, "validate->New.OpenAIConfigBaseURL", nil},
		{"base url unparsable", func(c *Config) { c.BaseURL = "http://[::1" }, "validate->New.OpenAIConfigBaseURLParse", nil},
		{"structured output unknown", func(c *Config) { c.StructuredOutput = "yaml" }, "validate->New.OpenAIConfigStructuredOutput", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validConfig("Local")
			tt.change(&c)
			got, err := validate(c)
			if tt.route != "" {
				wantRoute(t, err, tt.route)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestConfigJSONNames(t *testing.T) {
	data, err := json.Marshal(validConfig("Local"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"name", "protocol", "base_url", "model", "api_key", "kind", "timeout_seconds", "structured_output"} {
		if _, ok := m[k]; !ok {
			t.Errorf("providers.json field %q missing in %s", k, data)
		}
	}
}

func TestConfigNeverPrintsItsKey(t *testing.T) {
	c := validConfig("Local")
	var logs bytes.Buffer
	slog.New(slog.NewJSONHandler(&logs, nil)).Info("provider", "cfg", c)
	slog.New(slog.NewTextHandler(&logs, nil)).Info("provider", "cfg", c)
	outputs := map[string]string{
		"%v": fmt.Sprintf("%v", c), "%+v": fmt.Sprintf("%+v", c), "%s": fmt.Sprint(c),
		"String": c.String(), "log": logs.String(),
	}
	for what, out := range outputs {
		if strings.Contains(out, c.APIKey) {
			t.Errorf("%s leaks the API key: %s", what, out)
		}
		if !strings.Contains(out, "Local") {
			t.Errorf("%s lost the name: %s", what, out)
		}
	}
	if !strings.Contains(logs.String(), `"has_api_key":true`) {
		t.Errorf("log does not say a key is set: %s", logs.String())
	}
}

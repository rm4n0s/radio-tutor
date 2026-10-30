// Package providers keeps the AI providers of Radio Tutor in providers.json
// (SPEC §11.1 and §11.6): the list, the active provider, ready-made presets to
// fill the form, the one place that turns a stored provider into a
// grading.Provider (Build) and the connection test of the settings screen
// (Test).
//
// providers.json holds API keys, so it is written private to its user (mode 0600
// on Linux; on Windows a protected DACL that grants only the current user, in
// the local, non-roaming folder) and, on Linux, a file that other users can read
// is refused (ProvidersFileTooOpen). A Config is printed
// and logged without its key.
package providers

import (
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/grading/openai"
)

// Limits and defaults of a provider's settings.
const (
	// ProtocolOpenAI is the only protocol of v1: Chat Completions.
	ProtocolOpenAI = "openai"
	// DefaultTimeoutSeconds is the timeout of a provider that does not say
	// (SPEC §11.1).
	DefaultTimeoutSeconds = 180
	// MaxTimeoutSeconds is the longest timeout a provider may have.
	MaxTimeoutSeconds = 3600
	// MaxNameRunes is the most characters of a provider's name.
	MaxNameRunes = 60
)

// Kind says where a provider runs; the parent chooses it and the progress
// screens show it (SPEC §11.1, §11.7).
type Kind string

const (
	// KindCloud is a provider on the internet: it receives the photos.
	KindCloud Kind = "cloud"
	// KindLocal is a provider on this computer or the home network.
	KindLocal Kind = "local"
)

// Valid reports whether k is cloud or local.
func (k Kind) Valid() bool { return k == KindCloud || k == KindLocal }

// Config is one provider (SPEC §11.1), as stored in providers.json.
type Config struct {
	// Name is shown to the parent and identifies the provider: it is unique in
	// the list.
	Name string `json:"name"`
	// Protocol is "openai" (the only one in v1); empty means "openai".
	Protocol string `json:"protocol"`
	// BaseURL is the server's base URL, e.g. https://api.openai.com/v1.
	BaseURL string `json:"base_url"`
	// Model is the model name sent with every request.
	Model string `json:"model"`
	// APIKey is the Bearer token; it may be empty for local servers.
	APIKey string `json:"api_key"`
	// Kind is "cloud" or "local"; it is required.
	Kind Kind `json:"kind"`
	// TimeoutSeconds bounds one grading call; zero means 180.
	TimeoutSeconds int `json:"timeout_seconds"`
	// StructuredOutput is json_schema, json_object or none; empty means
	// json_schema.
	StructuredOutput grading.StructuredOutput `json:"structured_output"`
}

// String describes the provider without its API key, so printing a Config
// never leaks the key.
func (c Config) String() string {
	return "provider " + c.Name + " (" + string(c.Kind) + ", " + c.BaseURL + ", model " + c.Model + ")"
}

// LogValue makes slog log the provider without its API key.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("name", c.Name),
		slog.String("protocol", c.Protocol),
		slog.String("base_url", c.BaseURL),
		slog.String("model", c.Model),
		slog.Bool("has_api_key", c.APIKey != ""),
		slog.String("kind", string(c.Kind)),
		slog.Int("timeout_seconds", c.TimeoutSeconds),
		slog.String("structured_output", string(c.StructuredOutput)),
	)
}

// normalize trims the text fields and fills the defaults (protocol, timeout,
// structured output), then checks the fields the openai package does not check.
// Tags: ProviderNameInvalid, ProviderProtocolUnsupported, ProviderKindInvalid,
// ProviderTimeoutInvalid.
func normalize(c Config) (Config, error) {
	c.Name = strings.TrimSpace(c.Name)
	c.BaseURL = strings.TrimSpace(c.BaseURL)
	c.Model = strings.TrimSpace(c.Model)
	c.APIKey = strings.TrimSpace(c.APIKey)
	if c.Protocol == "" {
		c.Protocol = ProtocolOpenAI
	}
	if c.TimeoutSeconds == 0 {
		c.TimeoutSeconds = DefaultTimeoutSeconds
	}
	if c.StructuredOutput == "" {
		c.StructuredOutput = grading.StructuredJSONSchema
	}
	switch {
	case c.Name == "" || utf8.RuneCountInString(c.Name) > MaxNameRunes || strings.ContainsAny(c.Name, "\r\n\t"):
		return Config{}, errors.New("ProviderNameInvalid", "the provider needs a name of 1 to 60 characters on one line", "name", c.Name)
	case c.Protocol != ProtocolOpenAI:
		return Config{}, errors.New("ProviderProtocolUnsupported", "the protocol must be openai", "name", c.Name, "protocol", c.Protocol)
	case !c.Kind.Valid():
		return Config{}, errors.New("ProviderKindInvalid", "the kind must be cloud or local", "name", c.Name, "kind", string(c.Kind))
	case c.TimeoutSeconds < 1 || c.TimeoutSeconds > MaxTimeoutSeconds:
		return Config{}, errors.New("ProviderTimeoutInvalid", "the timeout must be 1 to 3600 seconds", "name", c.Name, "timeout_seconds", c.TimeoutSeconds)
	}
	return c, nil
}

// validate normalizes c and checks all of it; the base URL, the model and the
// structured output are checked by openai.New. It returns the normalized
// config.
func validate(c Config) (Config, error) {
	c, err := normalize(c)
	if err != nil {
		return Config{}, err
	}
	if _, err := openai.New(openaiConfig(c, ""), nil); err != nil {
		return Config{}, err
	}
	return c, nil
}

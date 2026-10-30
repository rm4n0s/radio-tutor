package providers

import (
	"log/slog"
	"time"

	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/grading/openai"
)

// openaiConfig turns a normalized Config into the openai package's Config;
// version is the app version for the User-Agent.
func openaiConfig(c Config, version string) openai.Config {
	return openai.Config{
		Version:          version,
		Name:             c.Name,
		BaseURL:          c.BaseURL,
		Model:            c.Model,
		APIKey:           c.APIKey,
		Timeout:          time.Duration(c.TimeoutSeconds) * time.Second,
		StructuredOutput: c.StructuredOutput,
	}
}

// Build turns a stored provider into the grading.Provider that talks to it. It
// is the one place that does this, so the submission service and the settings
// screen build providers the same way. lenient says whether the answers of the
// configured structured output mode need the lenient parser; set
// grading.Grader.Lenient to it. version is the app version, which goes into the
// User-Agent of the provider's requests ("radio-tutor/<version>"; empty means
// "dev"). log receives the provider's downgrade notice
// (nil discards it). A provider is built again when its settings change.
//
// It fails with the tags of config checking: ProviderNameInvalid,
// ProviderProtocolUnsupported, ProviderKindInvalid, ProviderTimeoutInvalid and
// the OpenAIConfig* tags of the openai package (model, base URL, structured
// output).
func Build(cfg Config, version string, log *slog.Logger) (provider grading.Provider, lenient bool, err error) {
	c, err := normalize(cfg)
	if err != nil {
		return nil, false, err
	}
	p, err := openai.New(openaiConfig(c, version), log)
	if err != nil {
		return nil, false, err
	}
	return p, c.StructuredOutput.Lenient(), nil
}

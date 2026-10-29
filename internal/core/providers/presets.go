package providers

import "github.com/rm4n0s/radio-tutor/internal/core/grading"

// Preset is a ready-made example of SPEC §11.1 that the settings screen offers
// to fill the form. Every value is a suggestion the parent may edit; the models
// are examples of vision-capable models and can be out of date.
type Preset struct {
	// ID is a stable identifier ("openai", "ollama", ...), also usable as a
	// key for translated texts.
	ID string
	// Name is the provider's suggested name.
	Name string
	// BaseURL is the server's base URL.
	BaseURL string
	// Model is an example of a vision-capable model.
	Model string
	// Kind is local for servers on this computer, cloud otherwise.
	Kind Kind
	// StructuredOutput is the suggested mode.
	StructuredOutput grading.StructuredOutput
	// NeedsAPIKey is false for local servers, which usually need no key.
	NeedsAPIKey bool
	// Description is a short English text for the form: what to put in the
	// fields and what to know.
	Description string
}

// Config returns the preset as a provider to edit: the suggested values, no
// API key.
func (p Preset) Config() Config {
	return Config{
		Name:             p.Name,
		Protocol:         ProtocolOpenAI,
		BaseURL:          p.BaseURL,
		Model:            p.Model,
		Kind:             p.Kind,
		TimeoutSeconds:   DefaultTimeoutSeconds,
		StructuredOutput: p.StructuredOutput,
	}
}

// Presets returns the examples of SPEC §11.1: OpenAI, Ollama, LM Studio,
// llama.cpp server, OpenRouter, Gemini and Anthropic (the last two through their
// OpenAI-compatible endpoints). Each call returns a new slice.
func Presets() []Preset {
	return []Preset{
		{
			ID: "openai", Name: "OpenAI", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o",
			Kind: KindCloud, StructuredOutput: grading.StructuredJSONSchema, NeedsAPIKey: true,
			Description: "OpenAI's cloud service. Needs an API key. The photos are sent to OpenAI. Use a model that can read images.",
		},
		{
			ID: "ollama", Name: "Ollama", BaseURL: "http://localhost:11434/v1", Model: "qwen2.5vl:7b",
			Kind: KindLocal, StructuredOutput: grading.StructuredJSONSchema,
			Description: "Ollama on this computer. No key needed. Pull a vision model first (for example: ollama pull qwen2.5vl:7b). The photos stay on this computer.",
		},
		{
			ID: "lmstudio", Name: "LM Studio", BaseURL: "http://localhost:1234/v1", Model: "qwen2.5-vl-7b-instruct",
			Kind: KindLocal, StructuredOutput: grading.StructuredJSONSchema,
			Description: "LM Studio's local server. No key needed. Load a vision model and start the server; use the model's name as LM Studio shows it.",
		},
		{
			ID: "llamacpp", Name: "llama.cpp server", BaseURL: "http://localhost:8080/v1", Model: "qwen2.5-vl-7b-instruct",
			Kind: KindLocal, StructuredOutput: grading.StructuredJSONSchema,
			Description: "llama.cpp's llama-server on this computer. No key needed. Start it with a vision model and its multimodal projector.",
		},
		{
			ID: "openrouter", Name: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1", Model: "openai/gpt-4o",
			Kind: KindCloud, StructuredOutput: grading.StructuredJSONSchema, NeedsAPIKey: true,
			Description: "OpenRouter, one key for many cloud models. Pick a model that can read images; some models do not support the JSON schema mode, then choose json_object. The photos are sent to the chosen model's host.",
		},
		{
			ID: "gemini", Name: "Gemini", BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai", Model: "gemini-2.5-flash",
			Kind: KindCloud, StructuredOutput: grading.StructuredJSONSchema, NeedsAPIKey: true,
			Description: "Google's Gemini through its OpenAI-compatible endpoint. Needs a Gemini API key. The photos are sent to Google.",
		},
		{
			ID: "anthropic", Name: "Anthropic", BaseURL: "https://api.anthropic.com/v1/", Model: "claude-opus-5-5",
			Kind: KindCloud, StructuredOutput: grading.StructuredJSONObject, NeedsAPIKey: true,
			Description: "Anthropic's Claude through its OpenAI-compatible endpoint. Needs an API key. That endpoint may not enforce the JSON schema, so the mode is json_object (the app checks the answer itself). The photos are sent to Anthropic.",
		},
	}
}

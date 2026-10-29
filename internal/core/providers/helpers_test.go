package providers

import (
	"testing"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/grading"
)

// validConfig is a config that passes every check.
func validConfig(name string) Config {
	return Config{
		Name: name, Protocol: "openai", BaseURL: "http://localhost:11434/v1", Model: "qwen2.5vl:7b",
		APIKey: "sk-secret-123", Kind: KindLocal, TimeoutSeconds: 30, StructuredOutput: grading.StructuredJSONSchema,
	}
}

// wantRoute fails unless err is an *errors.Error whose route contains route; it
// returns the error's metadata.
func wantRoute(t *testing.T, err error, route string) map[string]any {
	t.Helper()
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T: %v", err, err)
	}
	if !appErr.HasRoute(route) {
		t.Errorf("unexpected failure path: %s (want %s)", appErr.Route(), route)
	}
	return appErr.Metadata
}

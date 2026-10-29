//go:build radiotutor_ai

package providers

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestRealProvider runs Test against a real provider named by the environment:
// RADIO_TUTOR_AI_BASE_URL and RADIO_TUTOR_AI_MODEL (required), RADIO_TUTOR_AI_KEY
// and RADIO_TUTOR_AI_KIND (cloud or local; default local; the spec lists only the first three variables). It is not run by
// default (build tag radiotutor_ai); the answer's verdict is logged, not
// asserted, because it depends on the model.
func TestRealProvider(t *testing.T) {
	base, model := os.Getenv("RADIO_TUTOR_AI_BASE_URL"), os.Getenv("RADIO_TUTOR_AI_MODEL")
	if base == "" || model == "" {
		t.Skip("set RADIO_TUTOR_AI_BASE_URL and RADIO_TUTOR_AI_MODEL")
	}
	kind := Kind(os.Getenv("RADIO_TUTOR_AI_KIND"))
	if kind == "" {
		kind = KindLocal
	}
	cfg := Config{Name: "Real", BaseURL: base, Model: model, APIKey: os.Getenv("RADIO_TUTOR_AI_KEY"), Kind: kind}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	res, err := Test(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("the provider did not give a valid reply: %v", res.Err)
	}
	t.Logf("verdict %q from model %q in %v (downgraded to %q)", res.Verdict, res.Model, res.Duration, res.Downgraded)
}

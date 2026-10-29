package providers

import (
	"context"
	"time"

	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/providers/testimage"
)

// TestResult is what the settings screen's Test button shows (SPEC §11.6).
type TestResult struct {
	// OK is true when a valid reply came back (whatever its verdict).
	OK bool
	// Verdict is the verdict for the test exercise when OK; "correct" is what a
	// working vision model answers.
	Verdict grading.Verdict
	// Duration is how long the call took.
	Duration time.Duration
	// Model is the model that answered, as the provider reports it; the
	// configured one when the call failed.
	Model string
	// Downgraded is not empty when the provider had to use a weaker structured
	// output mode than the configured one: the parent should change the
	// setting to this value (SPEC §11.2).
	Downgraded grading.StructuredOutput
	// Err is why the call failed when OK is false: an *errors.Error of the
	// openai or grading package (OpenAIAuth, OpenAINotFound,
	// GradingReplyInvalid, ...), for the screen to explain.
	Err error
}

// Test sends the embedded test image ("2 + 3 = 5") with a one-exercise request
// to the provider and reports whether a valid reply came back, its verdict and
// the time it took (SPEC §11.6). The call is bounded by the provider's timeout
// and by ctx. A provider that fails or answers badly is a normal result (OK
// false, Err set); the returned error is only for a test that could not be made:
// a config that does not pass Build's checks, or an image that cannot be
// prepared. The Grader returns exactly one result for the one exercise.
func Test(ctx context.Context, cfg Config) (TestResult, error) {
	provider, lenient, err := Build(cfg, nil)
	if err != nil {
		return TestResult{}, err
	}
	photo, err := grading.NormalizePhoto(ctx, testimage.SumPNG())
	if err != nil {
		return TestResult{}, err
	}
	req := grading.Request{
		Title:    "Provider test",
		Language: "en",
		Ages:     [2]int{8, 10},
		Exercises: []cassette.Exercise{{
			ID: "sum", Label: "1", Title: "Sum", Prompt: "2 + 3 = ?",
			ReferenceSolution: "5", Grading: "The answer is 5.",
		}},
		Photos: [][]byte{photo},
	}
	g := &grading.Grader{Provider: provider, Lenient: lenient}
	started := time.Now()
	res, err := g.Grade(ctx, req)
	elapsed := time.Since(started)
	if err != nil {
		return TestResult{Duration: elapsed, Model: cfg.Model, Err: err}, nil
	}
	return TestResult{
		OK:         true,
		Verdict:    res.Exercises[0].Verdict,
		Duration:   elapsed,
		Model:      res.Meta.Model,
		Downgraded: res.Meta.Downgraded,
	}, nil
}

// Package fake is a scripted grading.Provider for tests: it answers with the
// replies and errors it was given, one per call, and records every request it
// receives. It never touches the network (CLAUDE.md: tests never call a real
// AI provider).
package fake

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/grading"
)

// Step is one scripted answer of the provider.
type Step struct {
	// Text is the raw answer returned.
	Text string
	// Meta is the call's meta returned (Model, FinishReason).
	Meta grading.Meta
	// Err, when not nil, is returned instead of the answer.
	Err error
	// Before, when not nil, runs when the step is used and before it
	// answers; a test can cancel a context in it.
	Before func()
}

// Call is one request the provider received.
type Call struct {
	// Request is the request.
	Request grading.Request
	// Prior is the retry hint, nil on a first call.
	Prior *grading.RetryHint
}

// Provider is a scripted grading.Provider. It is safe for concurrent use.
type Provider struct {
	name string

	mu    sync.Mutex
	steps []Step
	calls []Call
}

// New returns a provider with the given name that answers with the steps in
// order, one per call. When the steps run out it fails with ScriptExhausted.
func New(name string, steps ...Step) *Provider {
	return &Provider{name: name, steps: append([]Step(nil), steps...)}
}

// Reply is a step that answers with the text, as the model "gpt-fake" with the
// finish reason "stop".
func Reply(text string) Step {
	return Step{Text: text, Meta: grading.Meta{Model: "gpt-fake", FinishReason: "stop"}}
}

// Truncated is a step that answers with the text cut off: the finish reason is
// "length".
func Truncated(text string) Step {
	return Step{Text: text, Meta: grading.Meta{Model: "gpt-fake", FinishReason: "length"}}
}

// Fail is a step that fails with err.
func Fail(err error) Step { return Step{Err: err} }

// JSONReply returns the JSON text of a reply with the page notes and results.
func JSONReply(pageNotes string, results ...grading.ExerciseResult) string {
	if results == nil {
		results = []grading.ExerciseResult{}
	}
	b, err := json.Marshal(grading.Reply{PageNotes: pageNotes, Results: results})
	if err != nil {
		// Unreachable: the reply holds only strings.
		panic("fake: " + err.Error())
	}
	return string(b)
}

// Name returns the provider's name.
func (p *Provider) Name() string { return p.name }

// Grade records the call and answers with the next step.
func (p *Provider) Grade(ctx context.Context, req grading.Request, prior *grading.RetryHint) (string, grading.Meta, error) {
	if err := ctx.Err(); err != nil {
		return "", grading.Meta{}, errors.NewErr("FakeCanceled", err)
	}
	p.mu.Lock()
	p.calls = append(p.calls, Call{Request: req, Prior: prior})
	if len(p.steps) == 0 {
		n := len(p.calls)
		p.mu.Unlock()
		return "", grading.Meta{}, errors.New("FakeScriptExhausted", "the fake provider has no more scripted answers", "calls", n)
	}
	step := p.steps[0]
	p.steps = p.steps[1:]
	p.mu.Unlock()

	if step.Before != nil {
		step.Before()
	}
	if step.Err != nil {
		return "", grading.Meta{}, step.Err
	}
	return step.Text, step.Meta, nil
}

// Calls returns the requests received so far, oldest first.
func (p *Provider) Calls() []Call {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Call(nil), p.calls...)
}

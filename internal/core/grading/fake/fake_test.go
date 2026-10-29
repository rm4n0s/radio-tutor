package fake_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/grading/fake"
)

func TestScriptedAnswersAndRecordedRequests(t *testing.T) {
	boom := errors.New("Boom", "scripted failure")
	p := fake.New("Test provider",
		fake.Reply("one"),
		fake.Truncated("two"),
		fake.Fail(boom),
	)
	if p.Name() != "Test provider" {
		t.Errorf("Name() = %q", p.Name())
	}
	var _ grading.Provider = p

	req1 := grading.Request{Title: "first"}
	req2 := grading.Request{Title: "second"}
	hint := &grading.RetryHint{PreviousAnswer: "x", Problem: "y"}

	text, meta, err := p.Grade(context.Background(), req1, nil)
	if err != nil || text != "one" || meta.Model != "gpt-fake" || meta.FinishReason != "stop" {
		t.Errorf("call 1 = %q %+v %v", text, meta, err)
	}
	text, meta, err = p.Grade(context.Background(), req2, hint)
	if err != nil || text != "two" || meta.FinishReason != "length" {
		t.Errorf("call 2 = %q %+v %v", text, meta, err)
	}
	if _, _, err = p.Grade(context.Background(), req1, nil); err != error(boom) {
		t.Errorf("call 3 error = %v", err)
	}

	_, _, err = p.Grade(context.Background(), req1, nil)
	appErr, ok := errors.FromError(err)
	if !ok || !appErr.HasRoute("Provider.Grade.FakeScriptExhausted") {
		t.Errorf("exhausted script: %v", err)
	}

	calls := p.Calls()
	if len(calls) != 4 {
		t.Fatalf("%d calls recorded", len(calls))
	}
	if calls[0].Request.Title != "first" || calls[0].Prior != nil || calls[1].Request.Title != "second" || calls[1].Prior != hint {
		t.Errorf("calls = %+v", calls[:2])
	}
}

func TestCanceledContextIsNotRecorded(t *testing.T) {
	p := fake.New("p", fake.Reply("x"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := p.Grade(ctx, grading.Request{}, nil)
	appErr, ok := errors.FromError(err)
	if !ok || !appErr.HasRoute("Provider.Grade.FakeCanceled") || !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
	if len(p.Calls()) != 0 {
		t.Error("a canceled call was recorded")
	}
	// The step was not used up.
	if text, _, err := p.Grade(context.Background(), grading.Request{}, nil); err != nil || text != "x" {
		t.Errorf("after cancel: %q %v", text, err)
	}
}

func TestBeforeRunsWhenTheStepIsUsed(t *testing.T) {
	ran := 0
	step := fake.Reply("x")
	step.Before = func() { ran++ }
	p := fake.New("p", step)
	if _, _, err := p.Grade(context.Background(), grading.Request{}, nil); err != nil || ran != 1 {
		t.Errorf("ran = %d, err = %v", ran, err)
	}
}

func TestJSONReply(t *testing.T) {
	text := fake.JSONReply("notes", grading.ExerciseResult{ExerciseID: "a", Verdict: grading.VerdictCorrect, Transcription: "t", FeedbackForChild: "f", NotesForParent: "n"})
	var back grading.Reply
	if err := json.Unmarshal([]byte(text), &back); err != nil {
		t.Fatal(err)
	}
	if back.PageNotes != "notes" || len(back.Results) != 1 || back.Results[0].ExerciseID != "a" {
		t.Errorf("reply = %+v", back)
	}
	empty := fake.JSONReply("")
	if empty != `{"page_notes":"","results":[]}` {
		t.Errorf("empty reply = %s", empty)
	}
	// And the parser accepts it.
	if _, _, err := grading.ParseReply(text, []string{"a"}, false); err != nil {
		t.Error(err)
	}
}

func TestConcurrentUse(t *testing.T) {
	steps := make([]fake.Step, 20)
	for i := range steps {
		steps[i] = fake.Reply("x")
	}
	p := fake.New("p", steps...)
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := p.Grade(context.Background(), grading.Request{}, nil); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := len(p.Calls()); n != 20 {
		t.Errorf("%d calls", n)
	}
}

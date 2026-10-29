package grading_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/grading/fake"
)

func request() grading.Request {
	return grading.Request{
		Title: "Fractions", Language: "en", Ages: [2]int{8, 10},
		Exercises: []cassette.Exercise{
			{ID: "a", Label: "1", Title: "A", Prompt: "p", ReferenceSolution: "r", Grading: "g"},
			{ID: "b", Label: "2", Title: "B", Prompt: "p", ReferenceSolution: "r", Grading: "g"},
		},
		Photos: [][]byte{{1}},
	}
}

func res(id string, v grading.Verdict) grading.ExerciseResult {
	return grading.ExerciseResult{ExerciseID: id, Verdict: v, Transcription: "t", FeedbackForChild: "f", NotesForParent: "n"}
}

func goodReply() string {
	return fake.JSONReply("page", res("a", grading.VerdictCorrect), res("b", grading.VerdictPartial))
}

func wantRoute(t *testing.T, err error, route string) {
	t.Helper()
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T: %v", err, err)
	}
	if !appErr.HasRoute(route) {
		t.Errorf("unexpected failure path: %s (want %s)", appErr.Route(), route)
	}
}

// clock returns a Now function that advances by step on every call.
func clock(step time.Duration) func() time.Time {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	return func() time.Time {
		t := now
		now = now.Add(step)
		return t
	}
}

func TestGraderSuccess(t *testing.T) {
	p := fake.New("Fake cloud", fake.Reply(goodReply()))
	g := &grading.Grader{Provider: p, Now: clock(1500 * time.Millisecond)}
	got, err := g.Grade(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	if got.PageNotes != "page" || len(got.Exercises) != 2 {
		t.Fatalf("result = %+v", got)
	}
	if got.Exercises[0].Verdict != grading.VerdictCorrect || got.Exercises[1].Verdict != grading.VerdictPartial {
		t.Errorf("verdicts = %v, %v", got.Exercises[0].Verdict, got.Exercises[1].Verdict)
	}
	want := grading.Meta{Provider: "Fake cloud", Model: "gpt-fake", FinishReason: "stop", Duration: 1500 * time.Millisecond}
	if got.Meta != want {
		t.Errorf("meta = %+v, want %+v", got.Meta, want)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("warnings = %v", got.Warnings)
	}
	calls := p.Calls()
	if len(calls) != 1 || calls[0].Prior != nil {
		t.Fatalf("calls = %+v", calls)
	}
	if len(calls[0].Request.Exercises) != 2 {
		t.Error("the provider did not get the request")
	}
}

func TestGraderRetry(t *testing.T) {
	tests := []struct {
		name        string
		first       fake.Step
		lenient     bool
		problemPart string
	}{
		{"not JSON", fake.Reply("I think the child did well."), false, "invalid character"},
		{"prose around JSON in strict mode", fake.Reply("Sure! " + goodReply()), false, "invalid character"},
		{"missing field", fake.Reply(`{"page_notes":"p"}`), false, "results"},
		{"unknown verdict", fake.Reply(strings.Replace(goodReply(), `"correct"`, `"great"`, 1)), false, `"great"`},
		{"no object in lenient mode", fake.Reply("no braces here"), true, "no JSON object"},
		{"cut off by the length limit", fake.Truncated(goodReply()[:40]), false, "cut off"},
		{"cut off though the text parses", fake.Truncated(goodReply()), false, "cut off"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := fake.New("p", tt.first, fake.Reply(goodReply()))
			var logs bytes.Buffer
			g := &grading.Grader{Provider: p, Lenient: tt.lenient, Log: slog.New(slog.NewTextHandler(&logs, nil))}
			got, err := g.Grade(context.Background(), request())
			if err != nil {
				t.Fatal(err)
			}
			if !got.Meta.Retried {
				t.Error("Meta.Retried is false")
			}
			if got.Exercises[1].Verdict != grading.VerdictPartial {
				t.Errorf("result of the second answer not used: %+v", got.Exercises)
			}
			if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "asked again") {
				t.Errorf("warnings = %q", got.Warnings)
			}
			calls := p.Calls()
			if len(calls) != 2 {
				t.Fatalf("%d calls, want 2", len(calls))
			}
			if calls[0].Prior != nil {
				t.Error("first call has a retry hint")
			}
			prior := calls[1].Prior
			if prior == nil {
				t.Fatal("second call has no retry hint")
			}
			if prior.PreviousAnswer != tt.first.Text {
				t.Errorf("PreviousAnswer = %q", prior.PreviousAnswer)
			}
			if !strings.Contains(prior.Problem, tt.problemPart) {
				t.Errorf("Problem = %q, want it to contain %q", prior.Problem, tt.problemPart)
			}
			msg := prior.Message()
			if !strings.HasPrefix(msg, "Your previous answer was not valid JSON of the required schema: ") ||
				!strings.HasSuffix(msg, ". Answer again with only the JSON object.") {
				t.Errorf("message = %q", msg)
			}
			if !strings.Contains(logs.String(), "invalid") {
				t.Errorf("the retry was not logged: %s", logs.String())
			}
		})
	}
}

func TestGraderSecondAnswerInvalid(t *testing.T) {
	tests := []struct {
		name   string
		second fake.Step
		cause  string
	}{
		{"not JSON again", fake.Reply("still not JSON"), "GradingReplyNotJSON"},
		{"cut off again", fake.Truncated("{"), "GradingReplyTruncated"},
		{"bad verdict again", fake.Reply(strings.Replace(goodReply(), `"correct"`, `"yes"`, 1)), "GradingReplyVerdictInvalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := fake.New("p", fake.Reply("nope"), tt.second, fake.Reply(goodReply()))
			g := &grading.Grader{Provider: p}
			_, err := g.Grade(context.Background(), request())
			if err == nil {
				t.Fatal("expected an error")
			}
			wantRoute(t, err, "Grader.Grade.GradingReplyInvalid")
			appErr, _ := errors.FromError(err)
			if appErr.Metadata["provider"] != "p" {
				t.Errorf("metadata = %v", appErr.Metadata)
			}
			cause, ok := errors.FromError(errors.Unwrap(err))
			if !ok || cause.Tag != tt.cause {
				t.Errorf("cause = %v, want tag %s", errors.Unwrap(err), tt.cause)
			}
			if n := len(p.Calls()); n != 2 {
				t.Errorf("%d calls; the retry happens once", n)
			}
		})
	}
}

func TestGraderProviderErrorsAreNotRetried(t *testing.T) {
	auth := errors.New("ProviderAuthFailed", "bad key", "status", 401)
	tests := []struct {
		name  string
		steps []fake.Step
		route string
		calls int
	}{
		{"first call fails", []fake.Step{fake.Fail(auth), fake.Reply(goodReply())}, "ProviderAuthFailed", 1},
		{"retry call fails", []fake.Step{fake.Reply("nope"), fake.Fail(auth)}, "ProviderAuthFailed", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := fake.New("p", tt.steps...)
			g := &grading.Grader{Provider: p}
			_, err := g.Grade(context.Background(), request())
			if err != error(auth) {
				// The provider's error comes back untouched.
				t.Fatalf("err = %v, want the provider's error", err)
			}
			if n := len(p.Calls()); n != tt.calls {
				t.Errorf("%d calls, want %d", n, tt.calls)
			}
		})
	}
}

func TestGraderCanceledBeforeRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	first := fake.Reply("not json")
	first.Before = cancel
	p := fake.New("p", first, fake.Reply(goodReply()))
	g := &grading.Grader{Provider: p}
	_, err := g.Grade(ctx, request())
	wantRoute(t, err, "Grader.Grade.GradingCanceled")
	if !errors.Is(err, context.Canceled) {
		t.Error("the context error is lost")
	}
	if n := len(p.Calls()); n != 1 {
		t.Errorf("%d calls after cancel", n)
	}
}

func TestGraderRejectsBadRequests(t *testing.T) {
	p := fake.New("p", fake.Reply(goodReply()))
	g := &grading.Grader{Provider: p}
	req := request()
	req.Photos = nil
	_, err := g.Grade(context.Background(), req)
	wantRoute(t, err, "Request.Validate.GradingPhotoCount")
	if len(p.Calls()) != 0 {
		t.Error("the provider was called for an invalid request")
	}
}

func TestGraderWarningsAreLoggedAndReturned(t *testing.T) {
	text := fake.JSONReply("p", res("a", grading.VerdictCorrect), res("zzz", grading.VerdictCorrect))
	p := fake.New("p", fake.Reply(text))
	var logs bytes.Buffer
	g := &grading.Grader{Provider: p, Log: slog.New(slog.NewTextHandler(&logs, nil))}
	got, err := g.Grade(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Warnings) != 2 {
		t.Fatalf("warnings = %q", got.Warnings)
	}
	if got.Exercises[1].ExerciseID != "b" || got.Exercises[1].Verdict != grading.VerdictNotFound {
		t.Errorf("missing exercise not filled: %+v", got.Exercises[1])
	}
	for _, part := range []string{"zzz", "marked not_found"} {
		if !strings.Contains(logs.String(), part) {
			t.Errorf("log lacks %q: %s", part, logs.String())
		}
	}
	if got.Meta.Retried {
		t.Error("Retried is true without a retry")
	}
}

func TestGraderLenientModeUnwrapsFences(t *testing.T) {
	fenced := "```json\n" + goodReply() + "\n```"
	strict := &grading.Grader{Provider: fake.New("p", fake.Reply(fenced), fake.Reply(fenced))}
	if _, err := strict.Grade(context.Background(), request()); err == nil {
		t.Error("strict mode accepted a fence twice")
	}
	lenient := &grading.Grader{Provider: fake.New("p", fake.Reply(fenced)), Lenient: true}
	got, err := lenient.Grade(context.Background(), request())
	if err != nil || got.Meta.Retried {
		t.Errorf("lenient mode: %+v, %v", got.Meta, err)
	}
}

func TestGraderDefaultClock(t *testing.T) {
	g := &grading.Grader{Provider: fake.New("p", fake.Reply(goodReply()))}
	got, err := g.Grade(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	if got.Meta.Duration < 0 || got.Meta.Duration > time.Minute {
		t.Errorf("duration = %v", got.Meta.Duration)
	}
}

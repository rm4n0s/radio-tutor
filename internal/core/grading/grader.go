package grading

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/rm4n0s/errors"
)

// finishLength is the finish reason of an answer that was cut off.
const finishLength = "length"

// Result is a graded request.
type Result struct {
	// PageNotes are the model's notes about the photos as a whole.
	PageNotes string
	// Exercises has one result per asked exercise, in the order of
	// Request.Exercises.
	Exercises []ExerciseResult
	// Meta says which provider and model answered and how long it took.
	Meta Meta
	// Warnings are things the parser fixed in the reply (dropped or filled
	// results) and the retry, for the log.
	Warnings []string
}

// Grader asks a Provider to grade requests and applies the rules of SPEC §11.5
// to the answers: parsing and validation, and one retry when the answer is
// invalid. The zero value is not usable; set Provider.
type Grader struct {
	// Provider answers the requests.
	Provider Provider
	// Lenient makes the parser take the first JSON object in the answer
	// (needed for providers with structured_output "none" or "json_object").
	Lenient bool
	// Log receives the retry and the warnings. Nil discards them.
	Log *slog.Logger
	// Now is the clock for Meta.Duration. Nil means time.Now.
	Now func() time.Time
}

// Grade validates the request, asks the provider and returns the parsed
// result. When the answer is not valid JSON of the schema, or its finish
// reason is "length", the provider is asked once more with the invalid answer
// and the reason (RetryHint); a second invalid answer fails with
// GradingReplyInvalid, wrapping the parse failure. Errors of the provider are
// returned as they are and are not retried.
func (g *Grader) Grade(ctx context.Context, req Request) (Result, error) {
	if err := req.Validate(); err != nil {
		return Result{}, err
	}
	log := g.Log
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	now := g.Now
	if now == nil {
		now = time.Now
	}
	started := now()
	asked := req.ExerciseIDs()
	name := g.Provider.Name()

	raw, meta, err := g.Provider.Grade(ctx, req, nil)
	if err != nil {
		return Result{}, err
	}
	reply, warnings, checkErr := g.check(raw, meta, asked)
	retried := false
	if checkErr != nil {
		log.Warn("the grader's answer is invalid; asking once more", "provider", name, "err", checkErr)
		if cerr := ctx.Err(); cerr != nil {
			return Result{}, errors.NewErr("GradingCanceled", cerr)
		}
		retried = true
		hint := &RetryHint{PreviousAnswer: raw, Problem: checkErr.Error()}
		raw, meta, err = g.Provider.Grade(ctx, req, hint)
		if err != nil {
			return Result{}, err
		}
		reply, warnings, checkErr = g.check(raw, meta, asked)
		if checkErr != nil {
			return Result{}, errors.NewErr("GradingReplyInvalid", checkErr).SetMetadata("provider", name, "model", meta.Model)
		}
		warnings = append([]string{"the first answer was invalid and was asked again"}, warnings...)
	}
	for _, w := range warnings {
		log.Warn("grading reply fixed", "provider", name, "warning", w)
	}
	meta.Provider = name
	meta.Retried = retried
	meta.Duration = now().Sub(started)
	return Result{PageNotes: reply.PageNotes, Exercises: reply.Results, Meta: meta, Warnings: warnings}, nil
}

// check turns an answer into a Reply, or says why it is invalid.
func (g *Grader) check(raw string, meta Meta, asked []string) (Reply, []string, error) {
	if meta.FinishReason == finishLength {
		return Reply{}, nil, errors.New("GradingReplyTruncated", "the answer was cut off (finish reason \"length\")", "model", meta.Model)
	}
	return ParseReply(raw, asked, g.Lenient)
}

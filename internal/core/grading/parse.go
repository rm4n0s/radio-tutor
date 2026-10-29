package grading

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/rm4n0s/errors"
)

// rawReply and rawResult are the reply as decoded: pointers tell a missing
// field from an empty one, because the schema requires every field.
type rawReply struct {
	PageNotes *string      `json:"page_notes"`
	Results   *[]rawResult `json:"results"`
}

type rawResult struct {
	ExerciseID       *string `json:"exercise_id"`
	Verdict          *string `json:"verdict"`
	Transcription    *string `json:"transcription"`
	FeedbackForChild *string `json:"feedback_for_child"`
	NotesForParent   *string `json:"notes_for_parent"`
}

// missingNotes is the parent's note of an exercise the reply left out.
const missingNotes = "The checker's answer did not mention this exercise."

// ParseReply parses and validates the model's answer text against the schema
// of SPEC §11.5 for the exercises that were asked (their ids, in order).
//
// In strict mode (lenient false) the whole text must be the JSON object. In
// lenient mode the first balanced JSON object in the text is used, so text
// before or after it and a ```json code fence are tolerated; it is for
// providers without structured output.
//
// A valid reply is normalized: results for ids that were not asked are
// dropped, an asked id with no result gets the verdict "not_found", a repeated
// id keeps its first result, and the results come in the order of asked. Each
// of these gives a warning for the log. An invalid reply (not JSON, an
// unknown or missing field, a wrong type, a verdict outside the five) is an
// error; the Grader uses its message for the retry.
func ParseReply(text string, asked []string, lenient bool) (Reply, []string, error) {
	body, err := extractJSON(text, lenient)
	if err != nil {
		return Reply{}, nil, err
	}
	var raw rawReply
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return Reply{}, nil, errors.NewErr("GradingReplySchema", err).SetMetadata("reply", clip(text))
	}
	if raw.PageNotes == nil {
		return Reply{}, nil, errors.New("GradingReplyFieldMissing", "the field page_notes is missing", "field", "page_notes")
	}
	if raw.Results == nil {
		return Reply{}, nil, errors.New("GradingReplyFieldMissing", "the field results is missing", "field", "results")
	}

	askedSet := make(map[string]bool, len(asked))
	for _, id := range asked {
		askedSet[id] = true
	}
	var warnings []string
	found := make(map[string]ExerciseResult, len(asked))
	for i, r := range *raw.Results {
		path := "results[" + strconv.Itoa(i) + "]"
		res, field := r.complete()
		if field != "" {
			return Reply{}, nil, errors.New("GradingReplyFieldMissing", "the field "+path+"."+field+" is missing", "field", path+"."+field)
		}
		if !res.Verdict.Valid() {
			return Reply{}, nil, errors.New("GradingReplyVerdictInvalid",
				path+".verdict is "+strconv.Quote(string(res.Verdict))+"; it must be one of correct, partial, incorrect, unreadable, not_found",
				"field", path+".verdict", "verdict", string(res.Verdict))
		}
		switch _, dup := found[res.ExerciseID]; {
		case !askedSet[res.ExerciseID]:
			warnings = append(warnings, "dropped the result for "+strconv.Quote(res.ExerciseID)+", which was not asked")
		case dup:
			warnings = append(warnings, "dropped a repeated result for "+strconv.Quote(res.ExerciseID)+"; kept the first")
		default:
			found[res.ExerciseID] = res
		}
	}

	reply := Reply{PageNotes: *raw.PageNotes, Results: make([]ExerciseResult, 0, len(asked))}
	for _, id := range asked {
		res, ok := found[id]
		if !ok {
			warnings = append(warnings, "no result for "+strconv.Quote(id)+"; marked not_found")
			res = ExerciseResult{ExerciseID: id, Verdict: VerdictNotFound, NotesForParent: missingNotes}
		}
		reply.Results = append(reply.Results, res)
	}
	return reply, warnings, nil
}

// complete returns the result with all its fields, or the name of the first
// missing field.
func (r rawResult) complete() (ExerciseResult, string) {
	switch {
	case r.ExerciseID == nil:
		return ExerciseResult{}, "exercise_id"
	case r.Verdict == nil:
		return ExerciseResult{}, "verdict"
	case r.Transcription == nil:
		return ExerciseResult{}, "transcription"
	case r.FeedbackForChild == nil:
		return ExerciseResult{}, "feedback_for_child"
	case r.NotesForParent == nil:
		return ExerciseResult{}, "notes_for_parent"
	}
	return ExerciseResult{
		ExerciseID:       *r.ExerciseID,
		Verdict:          Verdict(*r.Verdict),
		Transcription:    *r.Transcription,
		FeedbackForChild: *r.FeedbackForChild,
		NotesForParent:   *r.NotesForParent,
	}, ""
}

// extractJSON returns the bytes of the JSON object in text: all of it (white
// space and a byte order mark trimmed) in strict mode, the first balanced
// object that is valid JSON in lenient mode.
func extractJSON(text string, lenient bool) ([]byte, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(text, "\ufeff"))
	if trimmed == "" {
		return nil, errors.New("GradingReplyEmpty", "the answer is empty")
	}
	if !lenient {
		var syntax json.RawMessage
		if err := json.Unmarshal([]byte(trimmed), &syntax); err != nil {
			return nil, errors.NewErr("GradingReplyNotJSON", err).SetMetadata("reply", clip(text))
		}
		return []byte(trimmed), nil
	}
	for from := 0; ; {
		start := strings.IndexByte(trimmed[from:], '{')
		if start < 0 {
			return nil, errors.New("GradingReplyNoObject", "the answer has no JSON object", "reply", clip(text))
		}
		start += from
		if end := balancedEnd(trimmed, start); end > 0 && json.Valid([]byte(trimmed[start:end])) {
			return []byte(trimmed[start:end]), nil
		}
		from = start + 1
	}
}

// balancedEnd returns the index just after the '}' that closes the '{' at
// start, ignoring braces inside JSON strings (with their escapes), or 0 when
// the object is never closed.
func balancedEnd(s string, start int) int {
	depth := 0
	inString, escaped := false, false
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case inString && escaped:
			escaped = false
		case inString && c == '\\':
			escaped = true
		case inString && c == '"':
			inString = false
		case inString:
		case c == '"':
			inString = true
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return 0
}

// clip shortens a reply for error metadata and logs.
func clip(s string) string {
	const max = 500
	if len(s) <= max {
		return s
	}
	return strings.ToValidUTF8(s[:max], "") + "..."
}

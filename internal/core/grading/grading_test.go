package grading

import (
	"strings"
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
)

func TestVerdicts(t *testing.T) {
	tests := []struct {
		v        Verdict
		valid    bool
		passes   bool
		attempts bool
	}{
		{VerdictCorrect, true, true, false},
		{VerdictPartial, true, false, true},
		{VerdictIncorrect, true, false, true},
		{VerdictUnreadable, true, false, false},
		{VerdictNotFound, true, false, false},
		{"", false, false, false},
		{"Correct", false, false, false},
		{"wrong", false, false, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.v), func(t *testing.T) {
			if got := tt.v.Valid(); got != tt.valid {
				t.Errorf("Valid() = %v", got)
			}
			if got := tt.v.Passes(); got != tt.passes {
				t.Errorf("Passes() = %v", got)
			}
			if got := tt.v.UsesAttempt(); got != tt.attempts {
				t.Errorf("UsesAttempt() = %v", got)
			}
		})
	}
	if n := len(Verdicts()); n != 5 {
		t.Errorf("%d verdicts", n)
	}
}

func TestRequestValidate(t *testing.T) {
	many := func(n int) []cassette.Exercise {
		var out []cassette.Exercise
		for i := range n {
			s := string(rune('a' + i))
			out = append(out, cassette.Exercise{ID: "ex-" + s, Label: s})
		}
		return out
	}
	tests := []struct {
		name  string
		edit  func(*Request)
		route string
	}{
		{"valid", func(*Request) {}, ""},
		{"ten exercises", func(r *Request) { r.Exercises = many(10) }, ""},
		{"six photos", func(r *Request) { r.Photos = make([][]byte, 6) }, ""},
		{"no exercises", func(r *Request) { r.Exercises = nil }, "Request.Validate.GradingExerciseCount"},
		{"eleven exercises", func(r *Request) { r.Exercises = many(11) }, "Request.Validate.GradingExerciseCount"},
		{"no photos", func(r *Request) { r.Photos = nil }, "Request.Validate.GradingPhotoCount"},
		{"seven photos", func(r *Request) { r.Photos = make([][]byte, 7) }, "Request.Validate.GradingPhotoCount"},
		{"same exercise twice", func(r *Request) { r.Exercises[1].ID = r.Exercises[0].ID }, "Request.Validate.GradingExerciseDuplicate"},
		{"same label twice", func(r *Request) { r.Exercises[1].Label = r.Exercises[0].Label }, "Request.Validate.GradingLabelDuplicate"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := sampleRequest()
			tt.edit(&req)
			err := req.Validate()
			if tt.route == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			wantTag(t, err, tt.route)
		})
	}
}

func TestNewRequest(t *testing.T) {
	c := &cassette.Cassette{
		Manifest: cassette.Manifest{Title: "T", Language: "en", Ages: []int{6, 8}, GraderInstructions: "be kind"},
		Exercises: map[string]*cassette.Exercise{
			"a": {ID: "a", Label: "1", Title: "A"},
			"b": {ID: "b", Label: "2", Title: "B"},
		},
	}
	photos := [][]byte{{1}}
	req, err := NewRequest(c, []string{"b", "a"}, photos)
	if err != nil {
		t.Fatal(err)
	}
	if req.Title != "T" || req.Language != "en" || req.Ages != [2]int{6, 8} || req.GraderInstructions != "be kind" {
		t.Errorf("manifest data not copied: %+v", req)
	}
	if got := strings.Join(req.ExerciseIDs(), ","); got != "b,a" {
		t.Errorf("exercise order = %s", got)
	}
	if len(req.Photos) != 1 {
		t.Errorf("photos = %d", len(req.Photos))
	}

	_, err = NewRequest(c, []string{"a", "zzz"}, photos)
	wantTag(t, err, "NewRequest.GradingExerciseUnknown")
}

func TestRetryHintMessage(t *testing.T) {
	got := RetryHint{PreviousAnswer: "x", Problem: "unexpected end"}.Message()
	want := "Your previous answer was not valid JSON of the required schema: unexpected end. Answer again with only the JSON object."
	if got != want {
		t.Errorf("Message() = %q", got)
	}
}

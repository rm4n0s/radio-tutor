package grading

import (
	"flag"
	"testing"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
)

var updateGoldens = flag.Bool("update-goldens", false, "rewrite the golden files")

// sampleRequest is a request with two exercises and two photos.
func sampleRequest() Request {
	return Request{
		Title:              "Fractions with pizza",
		Language:           "el",
		Ages:               [2]int{8, 10},
		GraderInstructions: "Speak kindly. Accept 1/2 and 2/4 as the same answer.",
		Exercises: []cassette.Exercise{
			{
				ID: "shade-the-half", Label: "3", Title: "Shade the half",
				Prompt:            "Shade half of the circle.",
				ReferenceSolution: "Exactly one of two equal parts is shaded.",
				Grading:           "Correct when half is shaded; partial when the parts are unequal.",
				ShowWork:          true,
			},
			{
				ID: "add-halves", Label: "4", Title: "Add the halves",
				Prompt:            "What is 1/2 + 1/2?",
				ReferenceSolution: "1",
				Grading:           "Correct for 1 or 2/2.",
			},
		},
		Photos: [][]byte{{1}, {2}},
	}
}

// wantTag fails the test unless err is an *errors.Error whose route contains
// route.
func wantTag(t *testing.T, err error, route string) {
	t.Helper()
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T: %v", err, err)
	}
	if !appErr.HasRoute(route) {
		t.Errorf("unexpected failure path: %s (want %s)", appErr.Route(), route)
	}
}

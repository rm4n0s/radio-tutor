package progress

import "testing"

func TestValidate(t *testing.T) {
	tests := []struct {
		name   string
		change func(p *Progress)
		tag    string
	}{
		{"valid", func(*Progress) {}, ""},
		{"valid stored states", func(p *Progress) {
			p.Exercises["a"] = ExerciseProgress{State: StatePassed, PassedBy: PassedByParent, LastVerdict: VerdictNotFound}
			p.Exercises["b"] = ExerciseProgress{State: StateNeedsReview, Attempts: 3}
		}, ""},
		{"format", func(p *Progress) { p.Format = 2 }, "ProgressFormatUnsupported"},
		{"format missing", func(p *Progress) { p.Format = 0 }, "ProgressFormatUnsupported"},
		{"other cassette", func(p *Progress) { p.CassetteID = "org.example.other-1" }, "ProgressCassetteMismatch"},
		{"negative plays", func(p *Progress) { p.Lessons["l1"] = LessonProgress{Plays: -1} }, "ProgressLessonCountInvalid"},
		{"state available", func(p *Progress) { p.Exercises["e1"] = ExerciseProgress{State: StateAvailable} }, "ProgressExerciseStateInvalid"},
		{"state garbage", func(p *Progress) { p.Exercises["e1"] = ExerciseProgress{State: "done"} }, "ProgressExerciseStateInvalid"},
		{"negative attempts", func(p *Progress) { p.Exercises["e1"] = ExerciseProgress{Attempts: -1} }, "ProgressExerciseCountInvalid"},
		{"negative hints", func(p *Progress) { p.Exercises["e1"] = ExerciseProgress{HintsShown: -1} }, "ProgressExerciseCountInvalid"},
		{"passed by", func(p *Progress) { p.Exercises["e1"] = ExerciseProgress{PassedBy: "cat"} }, "ProgressPassedByInvalid"},
		{"exercise verdict", func(p *Progress) { p.Exercises["e1"] = ExerciseProgress{LastVerdict: "great"} }, "ProgressExerciseVerdictInvalid"},
		{"submission status", func(p *Progress) { p.Submissions = []Submission{{ID: "s", Status: "lost"}} }, "ProgressSubmissionStatusInvalid"},
		{"result verdict", func(p *Progress) {
			p.Submissions = []Submission{{ID: "s", Status: SubmissionGraded, Results: []Result{{ExerciseID: "e1", Verdict: "great"}}}}
		}, "ProgressResultVerdictInvalid"},
		{"override action", func(p *Progress) { p.Overrides = []Override{{Action: "explode"}} }, "ProgressOverrideActionInvalid"},
		{"override reset_all", func(p *Progress) { p.Overrides = []Override{{Action: ActionResetAll}} }, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newProgress()
			tt.change(p)
			err := p.Validate("org.example.test-1")
			if tt.tag == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			wantTag(t, err, tt.tag, "Progress.Validate."+tt.tag)
		})
	}
}

func TestVerdictsAndStatuses(t *testing.T) {
	verdicts := map[Verdict]struct{ valid, counts bool }{
		VerdictCorrect: {true, true}, VerdictPartial: {true, true}, VerdictIncorrect: {true, true},
		VerdictUnreadable: {true, false}, VerdictNotFound: {true, false},
		"": {false, false}, "Correct": {false, false},
	}
	for v, w := range verdicts {
		if v.Valid() != w.valid || v.CountsAsAttempt() != w.counts {
			t.Errorf("%q: Valid %v CountsAsAttempt %v, want %v %v", v, v.Valid(), v.CountsAsAttempt(), w.valid, w.counts)
		}
	}
	for _, s := range []SubmissionStatus{SubmissionWaitingPhotos, SubmissionGrading, SubmissionGraded, SubmissionFailed, SubmissionCancelled} {
		if !s.Valid() {
			t.Errorf("%q should be valid", s)
		}
	}
	if SubmissionStatus("").Valid() {
		t.Error("the empty status should be invalid")
	}
}

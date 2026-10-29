package grading

import (
	"reflect"
	"strings"
	"testing"
)

const validReply = `{"page_notes":"ok","results":[
 {"exercise_id":"a","verdict":"correct","transcription":"1/2","feedback_for_child":"Bravo","notes_for_parent":"fine"},
 {"exercise_id":"b","verdict":"partial","transcription":"2","feedback_for_child":"Almost","notes_for_parent":"small slip"}]}`

func result(id string, v Verdict) ExerciseResult {
	return ExerciseResult{ExerciseID: id, Verdict: v, Transcription: "t", FeedbackForChild: "f", NotesForParent: "n"}
}

func TestParseReplyValid(t *testing.T) {
	reply, warnings, err := ParseReply(validReply, []string{"a", "b"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v", warnings)
	}
	if reply.PageNotes != "ok" || len(reply.Results) != 2 {
		t.Fatalf("reply = %+v", reply)
	}
	want := ExerciseResult{ExerciseID: "b", Verdict: VerdictPartial, Transcription: "2", FeedbackForChild: "Almost", NotesForParent: "small slip"}
	if reply.Results[1] != want {
		t.Errorf("results[1] = %+v", reply.Results[1])
	}
}

func TestParseReplyModes(t *testing.T) {
	asked := []string{"a", "b"}
	fenced := "Here is my answer:\n```json\n" + validReply + "\n```\nHope it helps!"
	tests := []struct {
		name    string
		text    string
		lenient bool
		route   string // empty: success
	}{
		{"strict plain", validReply, false, ""},
		{"strict with spaces and newlines", "\n  " + validReply + " \n\n", false, ""},
		{"strict with byte order mark", "\xef\xbb\xbf" + validReply, false, ""},
		{"strict refuses a fence", "```json\n" + validReply + "\n```", false, "ParseReply->extractJSON.GradingReplyNotJSON"},
		{"strict refuses extra text", "Sure. " + validReply, false, "ParseReply->extractJSON.GradingReplyNotJSON"},
		{"strict refuses two objects", validReply + validReply, false, "ParseReply->extractJSON.GradingReplyNotJSON"},
		{"strict refuses an array", "[]", false, "ParseReply.GradingReplySchema"},
		{"strict refuses a number", "42", false, "ParseReply.GradingReplySchema"},
		{"strict refuses truncated JSON", validReply[:60], false, "ParseReply->extractJSON.GradingReplyNotJSON"},
		{"empty", "", false, "ParseReply->extractJSON.GradingReplyEmpty"},
		{"blank lenient", " \n\t ", true, "ParseReply->extractJSON.GradingReplyEmpty"},
		{"lenient plain", validReply, true, ""},
		{"lenient fence", fenced, true, ""},
		{"lenient fence without language", "```\n" + validReply + "\n```", true, ""},
		{"lenient text around", "Sure! " + validReply + " Done.", true, ""},
		{"lenient skips a stray brace pair", "I used {curly} braces. " + validReply, true, ""},
		{"lenient skips an unbalanced brace", "Note: { is odd. " + validReply, true, ""},
		{"lenient braces inside strings", `x {"page_notes":"a } b { c \" }","results":[]}`, true, ""},
		{"lenient no object", "I cannot grade this.", true, "ParseReply->extractJSON.GradingReplyNoObject"},
		{"lenient only unbalanced", `{"page_notes": "x"`, true, "ParseReply->extractJSON.GradingReplyNoObject"},
		{"lenient takes the first object", `{"page_notes":"first","results":[]} {"page_notes":"second","results":[]}`, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseReply(tt.text, asked, tt.lenient)
			if tt.route == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected an error")
			}
			wantTag(t, err, tt.route)
		})
	}

	reply, _, err := ParseReply(`{"page_notes":"first","results":[]} {"page_notes":"second","results":[]}`, nil, true)
	if err != nil || reply.PageNotes != "first" {
		t.Errorf("first object: %+v, %v", reply, err)
	}
	reply, _, err = ParseReply(`x {"page_notes":"a } b { c \" }","results":[]}`, nil, true)
	if err != nil || reply.PageNotes != `a } b { c " }` {
		t.Errorf("braces in strings: %+v, %v", reply, err)
	}
}

func TestParseReplySchemaViolations(t *testing.T) {
	item := func(extra string) string {
		return `{"exercise_id":"a","verdict":"correct","transcription":"t","feedback_for_child":"f","notes_for_parent":"n"` + extra + `}`
	}
	wrap := func(results string) string { return `{"page_notes":"p","results":[` + results + `]}` }
	tests := []struct {
		name  string
		text  string
		route string
		part  string // text expected in the error message
	}{
		{"unknown top-level field", `{"page_notes":"p","results":[],"extra":1}`, "ParseReply.GradingReplySchema", "extra"},
		{"unknown result field", wrap(item(`,"score":3`)), "ParseReply.GradingReplySchema", "score"},
		{"page_notes missing", `{"results":[]}`, "ParseReply.GradingReplyFieldMissing", "page_notes"},
		{"results missing", `{"page_notes":"p"}`, "ParseReply.GradingReplyFieldMissing", "results"},
		{"results null", `{"page_notes":"p","results":null}`, "ParseReply.GradingReplyFieldMissing", "results"},
		{"page_notes wrong type", `{"page_notes":5,"results":[]}`, "ParseReply.GradingReplySchema", "page_notes"},
		{"results wrong type", `{"page_notes":"p","results":{}}`, "ParseReply.GradingReplySchema", "results"},
		{"result not an object", `{"page_notes":"p","results":["a"]}`, "ParseReply.GradingReplySchema", ""},
		{"exercise_id missing", wrap(strings.Replace(item(""), `"exercise_id":"a",`, "", 1)), "ParseReply.GradingReplyFieldMissing", "results[0].exercise_id"},
		{"verdict missing", wrap(strings.Replace(item(""), `"verdict":"correct",`, "", 1)), "ParseReply.GradingReplyFieldMissing", "results[0].verdict"},
		{"transcription missing", wrap(strings.Replace(item(""), `"transcription":"t",`, "", 1)), "ParseReply.GradingReplyFieldMissing", "results[0].transcription"},
		{"feedback missing", wrap(strings.Replace(item(""), `"feedback_for_child":"f",`, "", 1)), "ParseReply.GradingReplyFieldMissing", "results[0].feedback_for_child"},
		{"notes missing", wrap(strings.Replace(item(""), `,"notes_for_parent":"n"`, "", 1)), "ParseReply.GradingReplyFieldMissing", "results[0].notes_for_parent"},
		{"second result incomplete", wrap(item("") + `,{"exercise_id":"b"}`), "ParseReply.GradingReplyFieldMissing", "results[1].verdict"},
		{"verdict unknown", wrap(strings.Replace(item(""), `"correct"`, `"great"`, 1)), "ParseReply.GradingReplyVerdictInvalid", `"great"`},
		{"verdict wrong case", wrap(strings.Replace(item(""), `"correct"`, `"Correct"`, 1)), "ParseReply.GradingReplyVerdictInvalid", `"Correct"`},
		{"verdict empty", wrap(strings.Replace(item(""), `"correct"`, `""`, 1)), "ParseReply.GradingReplyVerdictInvalid", "results[0].verdict"},
		{"verdict a number", wrap(strings.Replace(item(""), `"correct"`, `1`, 1)), "ParseReply.GradingReplySchema", "verdict"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseReply(tt.text, []string{"a", "b"}, false)
			if err == nil {
				t.Fatal("expected an error")
			}
			wantTag(t, err, tt.route)
			if !strings.Contains(err.Error(), tt.part) {
				t.Errorf("message %q lacks %q", err.Error(), tt.part)
			}
		})
	}
}

func TestParseReplyNormalizesResults(t *testing.T) {
	text := func(rs ...ExerciseResult) string {
		var parts []string
		for _, r := range rs {
			parts = append(parts, `{"exercise_id":"`+r.ExerciseID+`","verdict":"`+string(r.Verdict)+`","transcription":"t","feedback_for_child":"f","notes_for_parent":"n"}`)
		}
		return `{"page_notes":"p","results":[` + strings.Join(parts, ",") + `]}`
	}
	tests := []struct {
		name      string
		text      string
		asked     []string
		wantIDs   []string
		wantVerd  []Verdict
		warnParts []string
	}{
		{
			name: "reordered", text: text(result("b", VerdictIncorrect), result("a", VerdictCorrect)),
			asked: []string{"a", "b"}, wantIDs: []string{"a", "b"}, wantVerd: []Verdict{VerdictCorrect, VerdictIncorrect},
		},
		{
			name: "unknown id dropped", text: text(result("a", VerdictCorrect), result("zzz", VerdictCorrect)),
			asked: []string{"a"}, wantIDs: []string{"a"}, wantVerd: []Verdict{VerdictCorrect},
			warnParts: []string{`dropped the result for "zzz"`},
		},
		{
			name: "missing id filled", text: text(result("a", VerdictCorrect)),
			asked: []string{"a", "b"}, wantIDs: []string{"a", "b"}, wantVerd: []Verdict{VerdictCorrect, VerdictNotFound},
			warnParts: []string{`no result for "b"`},
		},
		{
			name: "duplicate keeps the first", text: text(result("a", VerdictPartial), result("a", VerdictCorrect)),
			asked: []string{"a"}, wantIDs: []string{"a"}, wantVerd: []Verdict{VerdictPartial},
			warnParts: []string{`repeated result for "a"`},
		},
		{
			name: "empty results", text: text(),
			asked: []string{"a", "b"}, wantIDs: []string{"a", "b"}, wantVerd: []Verdict{VerdictNotFound, VerdictNotFound},
			warnParts: []string{`no result for "a"`, `no result for "b"`},
		},
		{
			name: "everything at once", text: text(result("x", VerdictCorrect), result("b", VerdictIncorrect), result("b", VerdictCorrect)),
			asked: []string{"a", "b"}, wantIDs: []string{"a", "b"}, wantVerd: []Verdict{VerdictNotFound, VerdictIncorrect},
			warnParts: []string{`"x"`, `repeated result for "b"`, `no result for "a"`},
		},
		{
			name: "empty id is not asked", text: text(result("", VerdictCorrect), result("a", VerdictCorrect)),
			asked: []string{"a"}, wantIDs: []string{"a"}, wantVerd: []Verdict{VerdictCorrect},
			warnParts: []string{`dropped the result for ""`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reply, warnings, err := ParseReply(tt.text, tt.asked, false)
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			var verdicts []Verdict
			for _, r := range reply.Results {
				ids = append(ids, r.ExerciseID)
				verdicts = append(verdicts, r.Verdict)
			}
			if !reflect.DeepEqual(ids, tt.wantIDs) || !reflect.DeepEqual(verdicts, tt.wantVerd) {
				t.Errorf("results = %v %v, want %v %v", ids, verdicts, tt.wantIDs, tt.wantVerd)
			}
			if len(warnings) != len(tt.warnParts) {
				t.Fatalf("warnings = %q, want %d", warnings, len(tt.warnParts))
			}
			for i, part := range tt.warnParts {
				if !strings.Contains(warnings[i], part) {
					t.Errorf("warning %d = %q, want it to contain %q", i, warnings[i], part)
				}
			}
			for _, r := range reply.Results {
				if r.Verdict == VerdictNotFound && r.NotesForParent == "" {
					t.Errorf("filled result %q has no note for the parent", r.ExerciseID)
				}
			}
		})
	}
}

func TestClip(t *testing.T) {
	short := "short"
	if clip(short) != short {
		t.Error("short text changed")
	}
	long := strings.Repeat("é", 400)
	got := clip(long)
	if !strings.HasSuffix(got, "...") || len(got) > 510 {
		t.Errorf("clip of long text: %d bytes", len(got))
	}
	if strings.ToValidUTF8(got, "") != got {
		t.Error("clip cut a character in half")
	}
}

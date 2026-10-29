package archive

import (
	"strings"
	"testing"
)

func TestCheckName(t *testing.T) {
	tests := []struct {
		name string
		want string // part of the message; "" for a good name
	}{
		{"cassette.json", ""},
		{"lessons/a-b_c.1/scene.svg", ""},
		{"", "empty"},
		{"/etc/passwd", "relative"},
		{"../evil", ".. part"},
		{"lessons/../../evil", ".. part"},
		{"lessons/a/../b", ".. part"},
		{"a\\b", "backslash"},
		{"..\\evil", "backslash"},
		{"a//b", "empty part"},
		{"a/./b", ". part"},
		{"Upper.json", "characters"},
		{"space name", "characters"},
		{"ünï.svg", "characters"},
		{"c:evil", "characters"},
		{"a\x00b", "characters"},
		{"a\nb", "characters"},
	}
	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.name, "/", "|"), func(t *testing.T) {
			got := checkName(tt.name)
			if (tt.want == "") != (got == "") || !strings.Contains(got, tt.want) {
				t.Errorf("checkName(%q) = %q, want it to contain %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestEntryProblemLayout(t *testing.T) {
	tests := []struct {
		name  string
		isDir bool
		ok    bool
	}{
		{"cassette.json", false, true},
		{"cover.svg", false, true},
		{"lessons", true, true},
		{"exercises", true, true},
		{"lessons/l1", true, true},
		{"exercises/e1", true, true},
		{"lessons/l1/lesson.json", false, true},
		{"lessons/l1/scene.svg", false, true},
		{"lessons/l1/narration.ogg", false, true},
		{"exercises/e1/exercise.json", false, true},
		{"exercises/e1/prompt.svg", false, true},
		{"exercises/e1/prompt.ogg", false, true},
		// Not part of §1.
		{".ds_store", false, false},
		{"readme.md", false, false},
		{"thumbs.db", false, false},
		{"__macosx/._cassette.json", false, false},
		{"__macosx", true, false},
		{"lessons/l1/exercise.json", false, false},
		{"exercises/e1/scene.svg", false, false},
		{"lessons/lesson.json", false, false},
		{"lessons/l1/extra/lesson.json", false, false},
		{"lessons/l1/lesson.json/x", false, false},
		{"other/l1/lesson.json", false, false},
		{"lessons/l1/lesson.json", true, false},
		{"lessons/l1/extra", true, false},
		{"cassette.json", true, false},
		{"cover.svg.bak", false, false},
		{"lessons", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := entryProblem(tt.name, tt.isDir)
			if (msg == "") != tt.ok {
				t.Errorf("entryProblem(%q, dir=%v) = %q", tt.name, tt.isDir, msg)
			}
		})
	}
}

func TestDisplay(t *testing.T) {
	long := strings.Repeat("a", 300)
	tests := []struct {
		name, want string
	}{
		{"lessons/a/scene.svg", "lessons/a/scene.svg"},
		{"Bad Name", `"Bad Name"...`},
		{"a\x00b\n", `"a\x00b\n"...`},
		{long, `"` + long[:120] + `"...`},
		{"\u00fcn\u00ef", "\"\\u00fcn\\u00ef\"..."},
	}
	for _, tt := range tests {
		if got := display(tt.name); got != tt.want {
			t.Errorf("display(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestCollectorCapsProblems(t *testing.T) {
	var c collector
	for i := 0; i < MaxProblems+5; i++ {
		c.errorf("p", "bad %d", i)
	}
	ps := c.problems()
	if len(ps) != MaxProblems+1 {
		t.Fatalf("len = %d", len(ps))
	}
	if last := ps[len(ps)-1]; last.Message != "5 more problems were not listed" || last.Path != "" || last.Warning {
		t.Errorf("last = %+v", last)
	}
	var none collector
	if got := none.problems(); len(got) != 0 {
		t.Errorf("empty collector = %v", got)
	}
}

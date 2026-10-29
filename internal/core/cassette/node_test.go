package cassette

import (
	"strings"
	"testing"
)

func TestParseJSON(t *testing.T) {
	n, ps := parseJSON([]byte(` {"a": [1, 2.5e3, "x", true, null, {"b": {}}], "c": ""} `))
	if len(ps) != 0 || n == nil {
		t.Fatalf("n=%v ps=%v", n, ps)
	}
	if got := strings.Join(n.names, ","); got != "a,c" {
		t.Errorf("names = %s", got)
	}
	a := n.field("a")
	kinds := []nodeKind{nodeNumber, nodeNumber, nodeString, nodeBool, nodeNull, nodeObject}
	if a.kind != nodeArray || len(a.items) != len(kinds) {
		t.Fatalf("a = %+v", a)
	}
	for i, k := range kinds {
		if a.items[i].kind != k {
			t.Errorf("item %d kind = %v, want %v", i, a.items[i].kind, k)
		}
	}
	if a.items[1].str != "2.5e3" || a.items[2].str != "x" {
		t.Errorf("values = %q %q", a.items[1].str, a.items[2].str)
	}
	if n.field("missing") != nil || (*node)(nil).field("a") != nil {
		t.Error("field of a missing name is not nil")
	}
}

func TestNodeKindStrings(t *testing.T) {
	want := []string{"null", "a boolean", "a number", "a string", "an array", "an object"}
	for i, w := range want {
		if got := nodeKind(i).String(); got != w {
			t.Errorf("kind %d = %q, want %q", i, got, w)
		}
	}
}

func TestParseJSONDuplicatesKeepTheFirst(t *testing.T) {
	n, ps := parseJSON([]byte(`{"a": 1, "b": {"c": 1, "c": 2}, "a": 3}`))
	if n == nil || len(ps) != 2 {
		t.Fatalf("n=%v ps=%v", n, ps)
	}
	if ps[0].Path != "b.c" || ps[1].Path != "a" {
		t.Errorf("paths = %v", ps)
	}
	if n.field("a").str != "1" {
		t.Errorf("a = %q", n.field("a").str)
	}
}

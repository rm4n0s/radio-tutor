package scene

import "testing"

func TestCSSNormalize(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Fill: RED", "fill: red"},
		{`anim\61tion`, "animation"},
		{`\61 nimation`, "animation"},
		{`\000061nimation`, "animation"},
		{`\0 x`, "\uFFFDx"},
		{`\110000 x`, "\uFFFDx"},
		{`a\
b`, "ab"},
		{`\a b`, "\nb"},
		{"\\a\r\nb", "\nb"},
		{`\"`, `"`},
		{`end\`, "end"},
		{"a/* c */b", "a b"},
		{"a/* unterminated", "a "},
		{"a/b", "a/b"},
	}
	for _, tc := range tests {
		if got := cssNormalize(tc.in); got != tc.want {
			t.Errorf("cssNormalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestURLProblem(t *testing.T) {
	tests := []struct {
		in   string
		bad  bool
		part string
	}{
		{"fill:url(#a)", false, ""},
		{"fill:url( '#a' )", false, ""},
		{`fill:url("#a") url(#b)`, false, ""},
		{"x:url(data:image/png;base64,AAAA)", false, ""},
		{"x:url('data:image/jpeg,AAAA')", false, ""},
		{"x:url(data:image/gif;base64,AAAA)", true, "data:image/gif"},
		{"x:url(a.png)", true, "a.png"},
		{"x:url(#a) url(b.png)", true, "b.png"},
		{"x:url('b.png", true, "b.png"},
		{"x:url(", true, ")"},
		{"x:url(http://example.com/a-very-long-address-that-gets-cut-in-the-message.png)", true, "..."},
		{"no references", false, ""},
	}
	for _, tc := range tests {
		got := urlProblem(tc.in)
		if (got != "") != tc.bad {
			t.Errorf("urlProblem(%q) = %q, bad = %v", tc.in, got, tc.bad)
		}
	}
}

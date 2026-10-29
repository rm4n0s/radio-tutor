package i18n

import (
	"reflect"
	"strings"
	"testing"
	"unicode"
)

func TestLanguagesHaveTheSameKeysAndPlaceholders(t *testing.T) {
	enKeys, elKeys := Keys(EN), Keys(EL)
	if !reflect.DeepEqual(enKeys, elKeys) {
		onlyEN, onlyEL := diff(enKeys, elKeys), diff(elKeys, enKeys)
		t.Fatalf("the keys differ: only in en %v, only in el %v", onlyEN, onlyEL)
	}
	for _, key := range enKeys {
		en, el := messagesEN[key], messagesEL[key]
		if strings.TrimSpace(en) == "" || strings.TrimSpace(el) == "" {
			t.Errorf("%s: empty text (en %q, el %q)", key, en, el)
		}
		if pe, pl := Placeholders(en), Placeholders(el); !reflect.DeepEqual(pe, pl) {
			t.Errorf("%s: placeholders differ: en %v, el %v", key, pe, pl)
		}
	}
}

func TestGreekMessagesAreGreek(t *testing.T) {
	for key, text := range messagesEL {
		if !strings.ContainsFunc(text, isGreek) {
			t.Errorf("%s: %q has no Greek letter", key, text)
		}
	}
}

func isGreek(r rune) bool { return unicode.Is(unicode.Greek, r) }

func diff(a, b []string) []string {
	in := map[string]bool{}
	for _, k := range b {
		in[k] = true
	}
	var out []string
	for _, k := range a {
		if !in[k] {
			out = append(out, k)
		}
	}
	return out
}

func TestT(t *testing.T) {
	tests := []struct {
		name string
		lang Lang
		key  string
		args []string
		want string
	}{
		{"plain english", EN, "login.back", nil, "Back"},
		{"plain greek", EL, "login.back", nil, "Πίσω"},
		{"placeholder", EN, "home.student.hello", []string{"name", "Mia"}, "Hello, Mia!"},
		{"placeholder greek", EL, "login.throttled", []string{"seconds", "30"}, "Πάρα πολλές λάθος προσπάθειες. Περίμενε 30 δευτερόλεπτα και δοκίμασε ξανά."},
		{"two placeholders", EN, "err.generic_detail", []string{"message", "boom", "tag", "X"}, "Something went wrong: boom (X)"},
		{"missing argument stays", EN, "home.student.hello", nil, "Hello, {name}!"},
		{"extra argument ignored", EN, "login.back", []string{"x", "y"}, "Back"},
		{"odd last argument ignored", EN, "home.student.hello", []string{"name", "Mia", "other"}, "Hello, Mia!"},
		{"unknown key is the key", EN, "no.such.key", nil, "no.such.key"},
		{"unknown language is english", Lang("fr"), "login.back", nil, "Back"},
		{"value with braces is not replaced twice", EN, "err.generic_detail", []string{"message", "{tag}", "tag", "T"}, "Something went wrong: {tag} (T)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := For(tt.lang).T(tt.key, tt.args...); got != tt.want {
				t.Errorf("T(%q, %v) = %q, want %q", tt.key, tt.args, got, tt.want)
			}
		})
	}
}

func TestMsgAndHas(t *testing.T) {
	c := For(EL)
	if got, want := c.Msg(M("home.student.hello", "name", "Μαρία")), "Γεια σου, Μαρία!"; got != want {
		t.Errorf("Msg = %q, want %q", got, want)
	}
	if !c.Has("login.back") || c.Has("nope") {
		t.Error("Has is wrong")
	}
	if c.Lang() != EL || For(Lang("zz")).Lang() != EN {
		t.Error("Lang is wrong")
	}
}

func TestFromLocale(t *testing.T) {
	tests := []struct {
		locale string
		want   Lang
	}{
		{"", EN}, {"C", EN}, {"POSIX", EN}, {"en_US.UTF-8", EN}, {"de_DE.UTF-8", EN},
		{"el", EL}, {"el_GR.UTF-8", EL}, {"el_GR@euro", EL}, {"EL_GR", EL}, {"el.UTF-8", EL}, {" el_CY ", EL},
		{"ell", EN}, {"elx", EN},
	}
	for _, tt := range tests {
		if got := FromLocale(tt.locale); got != tt.want {
			t.Errorf("FromLocale(%q) = %q, want %q", tt.locale, got, tt.want)
		}
	}
}

func TestPlaceholders(t *testing.T) {
	tests := []struct {
		text string
		want []string
	}{
		{"none", []string{}},
		{"{a} and {b} and {a}", []string{"a", "b"}},
		{"open {only", []string{}},
		{"empty {} braces", []string{}},
	}
	for _, tt := range tests {
		if got := Placeholders(tt.text); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Placeholders(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestLangHelpers(t *testing.T) {
	if got := Langs(); !reflect.DeepEqual(got, []Lang{EN, EL}) {
		t.Errorf("Langs = %v", got)
	}
	tests := []struct {
		l     Lang
		valid bool
		label string
	}{{EN, true, "EN"}, {EL, true, "ΕΛ"}, {Lang("fr"), false, "EN"}}
	for _, tt := range tests {
		if tt.l.Valid() != tt.valid || tt.l.Label() != tt.label {
			t.Errorf("%q: Valid %v Label %q", tt.l, tt.l.Valid(), tt.l.Label())
		}
	}
}

// Package i18n holds the UI string catalog of Radio Tutor (SPEC §17): one Go
// map per language (en.go, el.go) from message keys to text, with {name}
// placeholders. The phone page uses the same catalog.
//
// Adding a message means adding its key to every language file; the tests fail
// when the languages do not have the same keys and placeholders.
package i18n

import (
	"sort"
	"strings"
)

// Lang is a language of the UI.
type Lang string

// The languages of the UI (SPEC D19).
const (
	EN Lang = "en"
	EL Lang = "el"
)

// Langs returns the languages in the order the UI lists them.
func Langs() []Lang { return []Lang{EN, EL} }

// Valid reports whether l is one of the languages.
func (l Lang) Valid() bool { return l == EN || l == EL }

// Label is the short name the language switch shows for l: "EN" or "ΕΛ". It is
// the same in every UI language, so that a child can find their own language.
func (l Lang) Label() string {
	if l == EL {
		return "ΕΛ"
	}
	return "EN"
}

// FromLocale returns the language for a locale string such as the LANG
// variable ("el_GR.UTF-8", "el", "en_US.UTF-8", "C"): Greek for a locale that
// starts with "el" followed by nothing or one of "_", "." and "@", English for
// everything else (SPEC §12.4).
func FromLocale(locale string) Lang {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if rest, ok := strings.CutPrefix(locale, "el"); ok {
		if rest == "" || strings.ContainsRune("_.@-", rune(rest[0])) {
			return EL
		}
	}
	return EN
}

// catalogs maps each language to its messages. It is read-only.
var catalogs = map[Lang]map[string]string{
	EN: messagesEN,
	EL: messagesEL,
}

// Message is a message key with the arguments of its placeholders, to be
// translated later, so that text kept for the user (a notice, an error) follows
// the language when it changes.
type Message struct {
	// Key is the message key.
	Key string
	// Args are the placeholder values as name, value pairs (see Catalog.T).
	Args []string
}

// M returns a Message.
func M(key string, args ...string) Message { return Message{Key: key, Args: args} }

// Catalog translates message keys into one language.
type Catalog struct {
	lang Lang
	msgs map[string]string
}

// For returns the catalog of the language; an unknown language gets English.
func For(l Lang) Catalog {
	msgs, ok := catalogs[l]
	if !ok {
		l, msgs = EN, catalogs[EN]
	}
	return Catalog{lang: l, msgs: msgs}
}

// Lang is the language of the catalog.
func (c Catalog) Lang() Lang { return c.lang }

// T returns the text of the key with the placeholders filled in. args are name,
// value pairs: T("login.throttled", "seconds", "30") replaces {seconds}. A
// placeholder without an argument stays as written, an argument without a
// placeholder is ignored, and an odd last argument is ignored. A key the
// catalog does not have gives the key itself, so a missing message is visible
// and never crashes the UI.
func (c Catalog) T(key string, args ...string) string {
	text, ok := c.msgs[key]
	if !ok {
		return key
	}
	if len(args) < 2 || !strings.Contains(text, "{") {
		return text
	}
	pairs := make([]string, 0, len(args))
	for i := 0; i+1 < len(args); i += 2 {
		pairs = append(pairs, "{"+args[i]+"}", args[i+1])
	}
	return strings.NewReplacer(pairs...).Replace(text)
}

// Msg returns the text of the message.
func (c Catalog) Msg(m Message) string { return c.T(m.Key, m.Args...) }

// Has reports whether the catalog has the key.
func (c Catalog) Has(key string) bool {
	_, ok := c.msgs[key]
	return ok
}

// Keys returns the keys of the language, sorted.
func Keys(l Lang) []string {
	keys := make([]string, 0, len(catalogs[l]))
	for k := range catalogs[l] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Placeholders returns the names of the {name} placeholders in text, sorted and
// without repeats.
func Placeholders(text string) []string {
	seen := map[string]bool{}
	for {
		open := strings.IndexByte(text, '{')
		if open < 0 {
			break
		}
		end := strings.IndexByte(text[open:], '}')
		if end < 0 {
			break
		}
		if name := text[open+1 : open+end]; name != "" {
			seen[name] = true
		}
		text = text[open+end+1:]
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

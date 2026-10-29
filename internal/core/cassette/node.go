package cassette

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/rm4n0s/radio-tutor/internal/core/problem"
)

// maxJSONDepth is how deeply a cassette JSON file may nest arrays and objects.
// The format needs 4 levels; the limit only protects the parser.
const maxJSONDepth = 32

type nodeKind int

const (
	nodeNull nodeKind = iota
	nodeBool
	nodeNumber
	nodeString
	nodeArray
	nodeObject
)

func (k nodeKind) String() string {
	return [...]string{"null", "a boolean", "a number", "a string", "an array", "an object"}[k]
}

// node is a JSON value kept as a tree, so that the checks encoding/json
// cannot do are possible: which fields are present (a missing "t" is not the
// same as 0), duplicate names, and exact error paths.
type node struct {
	kind   nodeKind
	str    string // a string's value, or a number's literal
	items  []*node
	names  []string // an object's field names, in file order
	fields map[string]*node
}

func (n *node) field(name string) *node {
	if n == nil {
		return nil
	}
	return n.fields[name]
}

// parser reads a JSON value into a node tree. A failure sets msg and makes
// the parse functions return nil.
type parser struct {
	dec *json.Decoder
	l   problem.List // duplicate field names
	msg string       // the first syntax problem, empty when there is none
}

// parseJSON reads the one JSON value in data into a node tree. A syntax error
// is returned as a problem (and a nil node); so are duplicate field names,
// which encoding/json would silently resolve in favour of the last.
func parseJSON(data []byte) (*node, problem.List) {
	p := &parser{dec: json.NewDecoder(bytes.NewReader(data))}
	p.dec.UseNumber()
	n := p.value("", 0)
	if n != nil {
		if _, err := p.dec.Token(); err != io.EOF {
			n = p.fail("unexpected data after the top-level value")
		}
	}
	if n == nil {
		var l problem.List
		l.Errorf("", "invalid JSON%s: %s", lineOf(data, p.dec.InputOffset()), p.msg)
		return nil, l
	}
	return n, p.l
}

// lineOf returns " on line N" for a byte offset in data.
func lineOf(data []byte, offset int64) string {
	if offset < 0 || offset > int64(len(data)) {
		return ""
	}
	return " on line " + strconv.Itoa(1+bytes.Count(data[:offset], []byte("\n")))
}

func (p *parser) fail(msg string) *node {
	if p.msg == "" {
		p.msg = msg
	}
	return nil
}

func (p *parser) token() (json.Token, bool) {
	tok, err := p.dec.Token()
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	if err != nil {
		p.fail(err.Error())
		return nil, false
	}
	return tok, true
}

func (p *parser) value(path string, depth int) *node {
	tok, ok := p.token()
	if !ok {
		return nil
	}
	switch t := tok.(type) {
	case bool:
		return &node{kind: nodeBool}
	case json.Number:
		return &node{kind: nodeNumber, str: t.String()}
	case string:
		return &node{kind: nodeString, str: t}
	case json.Delim:
		if depth >= maxJSONDepth {
			return p.fail("nested too deeply")
		}
		if t == '[' {
			return p.array(path, depth)
		}
		return p.object(path, depth)
	}
	return &node{kind: nodeNull} // the only token left is null
}

func (p *parser) array(path string, depth int) *node {
	n := &node{kind: nodeArray}
	for p.dec.More() {
		item := p.value(problem.Index(path, len(n.items)), depth+1)
		if item == nil {
			return nil
		}
		n.items = append(n.items, item)
	}
	if _, ok := p.token(); !ok { // the closing ]
		return nil
	}
	return n
}

func (p *parser) object(path string, depth int) *node {
	n := &node{kind: nodeObject, fields: map[string]*node{}}
	for p.dec.More() {
		tok, ok := p.token()
		if !ok {
			return nil
		}
		name, _ := tok.(string) // inside an object the decoder yields a string here
		fp := problem.Join(path, name)
		v := p.value(fp, depth+1)
		if v == nil {
			return nil
		}
		if _, dup := n.fields[name]; dup {
			p.l.Errorf(fp, "duplicate field")
			continue
		}
		n.names = append(n.names, name)
		n.fields[name] = v
	}
	if _, ok := p.token(); !ok { // the closing }
		return nil
	}
	return n
}

// fieldKind is the JSON type a schema field must have.
type fieldKind int

const (
	kindString fieldKind = iota
	kindInt
	kindNumber
	kindBool
	kindInts    // an array of integers
	kindNumbers // an array of numbers
	kindStrings // an array of strings
	kindObjects // an array of objects described by field.elem
)

// field is one field of a JSON object: its name, type, and whether it must be
// present.
type field struct {
	name     string
	kind     fieldKind
	required bool
	elem     *schema             // for kindObjects
	check    func(string) string // for kindString: a message when the value is not acceptable
}

// schema describes the fields of a JSON object. Every name not in it is an
// error ("unknown field"), which is how a typo is caught.
type schema struct {
	fields []field
}

// check reports the structure problems of the object n at path: not an object,
// unknown fields, missing required fields, and fields of the wrong type. When
// it reports nothing, decoding n into the matching struct with
// DisallowUnknownFields cannot fail.
func (s *schema) check(l *problem.List, path string, n *node) {
	if n.kind != nodeObject {
		l.Errorf(path, "must be a JSON object, not %s", n.kind)
		return
	}
	known := make(map[string]field, len(s.fields))
	for _, f := range s.fields {
		known[f.name] = f
	}
	for _, name := range n.names {
		if _, ok := known[name]; !ok {
			l.Errorf(problem.Join(path, name), "unknown field (not part of the format; check the spelling)")
		}
	}
	for _, f := range s.fields {
		v, ok := n.fields[f.name]
		fp := problem.Join(path, f.name)
		switch {
		case !ok:
			if f.required {
				l.Errorf(fp, "is required")
			}
		default:
			f.checkValue(l, fp, v)
		}
	}
}

func (f field) checkValue(l *problem.List, path string, v *node) {
	switch f.kind {
	case kindString:
		if checkKind(l, path, v, nodeString) && f.check != nil {
			if msg := f.check(v.str); msg != "" {
				l.Errorf(path, "%s", msg)
			}
		}
	case kindInt:
		checkInt(l, path, v)
	case kindNumber:
		checkNumber(l, path, v)
	case kindBool:
		checkKind(l, path, v, nodeBool)
	case kindInts, kindNumbers, kindStrings, kindObjects:
		if !checkKind(l, path, v, nodeArray) {
			return
		}
		for i, item := range v.items {
			ip := problem.Index(path, i)
			switch f.kind {
			case kindInts:
				checkInt(l, ip, item)
			case kindNumbers:
				checkNumber(l, ip, item)
			case kindStrings:
				checkKind(l, ip, item, nodeString)
			case kindObjects:
				f.elem.check(l, ip, item)
			}
		}
	}
}

func checkKind(l *problem.List, path string, v *node, want nodeKind) bool {
	if v.kind != want {
		l.Errorf(path, "must be %s, not %s", want, v.kind)
		return false
	}
	return true
}

func checkNumber(l *problem.List, path string, v *node) bool {
	if !checkKind(l, path, v, nodeNumber) {
		return false
	}
	if f, err := strconv.ParseFloat(v.str, 64); err != nil || math.IsInf(f, 0) {
		l.Errorf(path, "%s is too large to be a number", v.str)
		return false
	}
	return true
}

func checkInt(l *problem.List, path string, v *node) bool {
	if !checkKind(l, path, v, nodeNumber) {
		return false
	}
	if strings.ContainsAny(v.str, ".eE") {
		l.Errorf(path, "must be a whole number, not %s", v.str)
		return false
	}
	if n, err := strconv.ParseInt(v.str, 10, 32); err != nil || n != int64(int32(n)) {
		l.Errorf(path, "%s is out of range", v.str)
		return false
	}
	return true
}

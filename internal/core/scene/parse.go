package scene

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/problem"
)

// SVGNamespace is the namespace of the root element.
const SVGNamespace = "http://www.w3.org/2000/svg"

// actorIDPattern is the id rule of CASSETTE-FORMAT §2 (the cassette package
// has the same pattern for lesson and exercise ids).
var actorIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// childKind says what a top-level child of the root is for.
type childKind int

const (
	kindStatic childKind = iota // drawn as it is, merged with its static neighbours
	kindShared                  // <defs> and <style>: copied into every layer
	kindActor                   // a <g id>: a layer of its own
	kindInert                   // not drawn (title, desc, metadata, foreign namespaces): dropped
)

// frame is an open element.
type frame struct {
	name   xml.Name
	path   string
	counts map[string]int // how many children of each name were seen, for paths
	style  *strings.Builder
	line   int

	// For a top-level child of the root.
	start   int64
	kind    childKind
	actorID string
}

// pending is a layer that is being collected: the raw markup of its elements.
type pending struct {
	actorID string
	parts   [][]byte
}

// parser holds the state of one Split or CheckPicture run.
type parser struct {
	data    []byte
	dec     *xml.Decoder
	isScene bool // actor rules and layers; false for a picture

	ps         problem.List
	stopped    bool // too many problems, or a syntax error: the run is cut short
	incomplete bool // the document could not be read to its end

	stack    []*frame
	rootOpen []byte // the root's start tag, as written
	rootDone bool
	viewBox  ViewBox
	hasView  bool

	shared  [][]byte
	layers  []*pending
	current *pending // the static layer being collected, or nil
	actors  []string
	isActor map[string]bool
}

func newParser(data []byte, isScene bool) *parser {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	return &parser{data: data, dec: dec, isScene: isScene, isActor: map[string]bool{}}
}

// errorf records an error. line is the line of the element, 0 when unknown.
func (p *parser) errorf(path string, line int, format string, args ...any) {
	p.add(path, line, false, format, args...)
}

func (p *parser) warnf(path string, line int, format string, args ...any) {
	p.add(path, line, true, format, args...)
}

func (p *parser) add(path string, line int, warning bool, format string, args ...any) {
	if p.stopped {
		return
	}
	if len(p.ps) >= MaxProblems {
		p.ps.Errorf("", "more than %d problems were found; fix these first, the rest are not listed", MaxProblems)
		p.stopped = true
		p.incomplete = true
		return
	}
	msg := fmt.Sprintf(format, args...)
	if line > 0 {
		msg += " (line " + strconv.Itoa(line) + ")"
	}
	if warning {
		p.ps.Warnf(path, "%s", msg)
	} else {
		p.ps.Errorf(path, "%s", msg)
	}
}

// run reads every token of the document.
func (p *parser) run() {
	if bytes.HasPrefix(p.data, []byte{0xEF, 0xBB, 0xBF}) {
		p.errorf("", 0, "the file starts with a byte order mark; save it as UTF-8 without BOM")
		p.incomplete = true
		return
	}
	var prev int64
	for !p.stopped {
		tok, err := p.dec.Token()
		cur := p.dec.InputOffset()
		start := prev
		prev = cur
		line, _ := p.dec.InputPos()
		if err != nil {
			p.readFailed(err)
			return
		}
		switch t := tok.(type) {
		case xml.StartElement:
			p.start(t, start, cur, line)
		case xml.EndElement:
			p.end(cur)
		case xml.CharData:
			p.text(t, line)
		case xml.ProcInst:
			p.procInst(t, start, line)
		case xml.Directive:
			p.errorf("", line, "<!DOCTYPE> and other <!...> declarations are not allowed (they can define entities and load files)")
		case xml.Comment:
		}
	}
}

// readFailed handles the end of the token stream.
func (p *parser) readFailed(err error) {
	if errors.Is(err, io.EOF) {
		// An unclosed element is a syntax error ("unexpected EOF"), so a plain
		// end of the file means no element was open.
		if !p.rootDone {
			p.incomplete = true
			p.errorf("", 0, "the file has no <svg> element")
		}
		return
	}
	p.incomplete = true
	var syn *xml.SyntaxError
	if errors.As(err, &syn) {
		p.errorf("line "+strconv.Itoa(syn.Line), 0, "the file is not well-formed XML: %s", syn.Msg)
	} else {
		p.errorf("", 0, "the file is not well-formed XML: %s", err.Error())
	}
	p.stopped = true
}

func (p *parser) procInst(t xml.ProcInst, start int64, line int) {
	switch {
	case t.Target != "xml":
		p.errorf("", line, "the processing instruction <?%s ...?> is not allowed", t.Target)
	case start != 0:
		p.errorf("", line, "the XML declaration must be the first thing in the file")
	}
}

func (p *parser) text(t xml.CharData, line int) {
	if n := len(p.stack); n > 0 {
		if st := p.stack[n-1].style; st != nil {
			st.Write(t)
		}
		return
	}
	if len(bytes.TrimSpace(t)) > 0 {
		p.errorf("", line, "text outside the <svg> element")
		p.stopped, p.incomplete = true, true
	}
}

// start handles a start tag; startOff and endOff delimit the tag in the data.
func (p *parser) start(se xml.StartElement, startOff, endOff int64, line int) {
	if len(p.stack) == 0 {
		if p.rootDone {
			p.errorf("", line, "there is more than one root element; the file must have one <svg>")
			p.stopped, p.incomplete = true, true
			return
		}
		if se.Name.Local != "svg" || se.Name.Space != SVGNamespace {
			p.errorf("", line, "the root element must be <svg xmlns=\"%s\">", SVGNamespace)
			p.stopped, p.incomplete = true, true
			return
		}
		p.rootOpen = p.data[startOff:endOff]
		p.readViewBox(se, line)
	}
	if len(p.stack) >= MaxDepth {
		p.errorf(p.stack[len(p.stack)-1].path, line, "elements are nested more than %d levels deep", MaxDepth)
		p.stopped, p.incomplete = true, true
		return
	}

	f := &frame{name: se.Name, line: line, start: startOff}
	if len(p.stack) == 0 {
		f.path = "svg"
	} else {
		parent := p.stack[len(p.stack)-1]
		f.path = parent.path + "/" + step(parent, se)
	}
	if se.Name.Local == "style" && se.Name.Space == SVGNamespace {
		f.style = &strings.Builder{}
	}
	p.stack = append(p.stack, f)

	if msg := forbiddenElement(se.Name.Local); msg != "" {
		p.errorf(f.path, line, "%s", msg)
	}
	p.checkAttrs(se, f.path, line)
	if len(p.stack) == 2 {
		p.classify(f, se, line)
	}
}

// step is the path step of a child element: its name, with its id in brackets
// when it has one ("g[@id=pizza]") and its position among the children of the
// same name otherwise ("rect[2]").
func step(parent *frame, se xml.StartElement) string {
	if parent.counts == nil {
		parent.counts = map[string]int{}
	}
	parent.counts[se.Name.Local]++
	if id := attrValue(se, "id"); id != "" {
		return se.Name.Local + "[@id=" + id + "]"
	}
	return se.Name.Local + "[" + strconv.Itoa(parent.counts[se.Name.Local]) + "]"
}

// attrValue returns the value of the attribute with the given name and no
// namespace, and whether it was present.
func attrValue(se xml.StartElement, name string) string {
	v, _ := lookupAttr(se, name)
	return v
}

func lookupAttr(se xml.StartElement, name string) (string, bool) {
	for _, a := range se.Attr {
		if a.Name.Space == "" && a.Name.Local == name {
			return a.Value, true
		}
	}
	return "", false
}

// readViewBox reads and checks the root's viewBox.
func (p *parser) readViewBox(se xml.StartElement, line int) {
	raw, ok := lookupAttr(se, "viewBox")
	if !ok {
		p.errorf("svg", line, "the <svg> element has no viewBox, such as viewBox=\"0 0 1600 900\"")
		return
	}
	vb, msg := parseViewBox(raw)
	if msg != "" {
		p.errorf("svg/@viewBox", line, "%s", msg)
		return
	}
	p.viewBox, p.hasView = vb, true
}

// classify decides what a top-level child of the root is for.
func (p *parser) classify(f *frame, se xml.StartElement, line int) {
	name := se.Name.Local
	switch {
	case se.Name.Space != SVGNamespace, name == "title", name == "desc", name == "metadata":
		f.kind = kindInert
	case name == "defs", name == "style":
		f.kind = kindShared
	case name == "g":
		id, has := lookupAttr(se, "id")
		if !has {
			f.kind = kindStatic
			return
		}
		f.kind, f.actorID = kindActor, id
		if p.isScene {
			p.checkActorID(f, id, line)
		}
	default:
		f.kind = kindStatic
		if p.isScene && isDefinition(name) {
			p.warnf(f.path, line, "a top-level <%s> is only available to the static layer it is in; put it inside <defs> so that every actor can use it", name)
		}
	}
}

// isDefinition reports whether name is an element that only defines something
// for others to reference.
func isDefinition(name string) bool {
	switch name {
	case "linearGradient", "radialGradient", "pattern", "clipPath", "mask", "marker", "symbol", "filter":
		return true
	}
	return false
}

// checkActorID applies the actor id rules: valid, unique, at most MaxActors.
func (p *parser) checkActorID(f *frame, id string, line int) {
	switch {
	case !actorIDPattern.MatchString(id):
		p.errorf(f.path, line, "the actor id \"%s\" is not valid: use 1 to 63 lowercase letters, digits and hyphens, starting with a letter or digit", id)
		return
	case p.isActor[id]:
		p.errorf(f.path, line, "the actor id \"%s\" is used by another top-level <g>; actor ids must be unique", id)
		return
	}
	p.isActor[id] = true
	p.actors = append(p.actors, id)
	if len(p.actors) == MaxActors+1 {
		p.errorf(f.path, line, "the scene has more than %d actors", MaxActors)
	}
}

// end handles an end tag; endOff is the offset after it.
func (p *parser) end(endOff int64) {
	f := p.stack[len(p.stack)-1]
	p.stack = p.stack[:len(p.stack)-1]
	if f.style != nil {
		for _, msg := range cssProblems(cssNormalize(f.style.String())) {
			p.errorf(f.path, f.line, "%s", msg)
		}
	}
	switch len(p.stack) {
	case 0:
		p.rootDone = true
	case 1:
		p.finishChild(f, p.data[f.start:endOff])
	}
}

// finishChild files the raw markup of a top-level child.
func (p *parser) finishChild(f *frame, raw []byte) {
	switch f.kind {
	case kindShared:
		p.shared = append(p.shared, raw)
	case kindStatic:
		if p.current == nil {
			p.current = &pending{}
			p.layers = append(p.layers, p.current)
		}
		p.current.parts = append(p.current.parts, raw)
	case kindActor:
		p.current = nil
		p.layers = append(p.layers, &pending{actorID: f.actorID, parts: [][]byte{raw}})
	case kindInert:
	}
}

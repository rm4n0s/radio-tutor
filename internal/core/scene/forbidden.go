package scene

import (
	"encoding/xml"
	"strings"
)

const xlinkNamespace = "http://www.w3.org/1999/xlink"

// forbiddenElement returns the message for an element name that
// CASSETTE-FORMAT §5.2 does not allow, or "" when the element is allowed. The
// name is compared without regard to case and in any namespace.
func forbiddenElement(local string) string {
	l := strings.ToLower(local)
	switch {
	case l == "script":
		return "<" + local + "> is not allowed: scenes cannot run code"
	case strings.HasPrefix(l, "animate"), l == "set", l == "discard":
		return "<" + local + "> is not allowed: SMIL animation; animate with the timeline in lesson.json"
	case l == "foreignobject":
		return "<" + local + "> is not allowed"
	case l == "feimage", l == "fetile", l == "feturbulence", l == "feconvolvematrix", l == "fedisplacementmap":
		return "the filter primitive <" + local + "> is not allowed"
	case l == "fediffuselighting", l == "fespecularlighting", l == "fedistantlight", l == "fepointlight", l == "fespotlight":
		return "the lighting filter <" + local + "> is not allowed"
	}
	return ""
}

// attrName is how an attribute is named in a path and a message.
func attrName(n xml.Name) string {
	if n.Space == xlinkNamespace {
		return "xlink:" + n.Local
	}
	return n.Local
}

// checkAttrs applies the attribute rules of CASSETTE-FORMAT §5.2 to one
// element: event attributes, textLength, vertical text, hrefs, url() references
// and the CSS of the style attribute. Namespace declarations are skipped.
func (p *parser) checkAttrs(se xml.StartElement, path string, line int) {
	seen := make(map[xml.Name]bool, len(se.Attr))
	local := strings.ToLower(se.Name.Local)
	for _, a := range se.Attr {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		apath := path + "/@" + attrName(a.Name)
		if seen[a.Name] {
			p.errorf(apath, line, "the attribute \"%s\" appears twice", attrName(a.Name))
		}
		seen[a.Name] = true

		name := strings.ToLower(a.Name.Local)
		switch {
		case strings.HasPrefix(name, "on"):
			p.errorf(apath, line, "\"%s\" is an event attribute and is not allowed: scenes cannot run code", attrName(a.Name))
			continue
		case name == "textlength":
			p.errorf(apath, line, "textLength is not allowed")
		case name == "href":
			p.checkHref(apath, line, local, a.Value)
			continue
		}
		css := cssNormalize(a.Value)
		if name == "style" {
			for _, msg := range cssProblems(css) {
				p.errorf(apath, line, "%s", msg)
			}
			continue
		}
		if name == "writing-mode" && verticalWritingMode.MatchString("writing-mode:"+strings.TrimSpace(css)) {
			p.errorf(apath, line, "vertical text (writing-mode) is not allowed")
		}
		if msg := urlProblem(css); msg != "" {
			p.errorf(apath, line, "%s", msg)
		}
	}
}

// checkHref applies the rule for href and xlink:href: a fragment of this
// document ("#id"), and for <image> only a data:image/png or data:image/jpeg
// URL.
func (p *parser) checkHref(path string, line int, element, value string) {
	v := strings.TrimSpace(value)
	switch {
	case element == "image" && !isDataImage(v):
		p.errorf(path, line, "<image> needs a data:image/png or data:image/jpeg URL; external files are not allowed")
	case !allowedReference(v):
		p.errorf(path, line, "href must be \"#id\" or a data:image/png or data:image/jpeg URL; external references are not allowed (\"%s\")", shorten(v))
	}
}

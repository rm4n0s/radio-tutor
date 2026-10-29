package scene

import (
	"bytes"
	"encoding/xml"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/problem"
	"github.com/rm4n0s/radio-tutor/internal/core/timeline"
)

// sample is the example of CASSETTE-FORMAT §5.1 with a few more things in it:
// shared defs and style, an xlink:href, an entity, a comment and a nested group.
const sample = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 1600 900" width="1600" height="900">
  <title>Pizza</title>
  <defs>
    <linearGradient id="sky"><stop offset="0" stop-color="#9cf"/><stop offset="1" stop-color="#fff"/></linearGradient>
  </defs>
  <style>.big { font-size: 72px; }</style>
  <rect width="1600" height="900" fill="url(#sky)"/>
  <circle cx="10" cy="10" r="5"/>
  <g id="pizza">
    <!-- the pizza -->
    <g><circle cx="800" cy="450" r="300" fill="#fc6"/><use xlink:href="#sky"/></g>
  </g>
  <g>
    <rect x="1" y="1" width="2" height="2"/>
  </g>
  <g id="slice"><path d="M0 0 L10 10"/></g>
  <g id="label-half">
    <text class="big" x="800" y="800" text-anchor="middle">Fish &amp; chips 1/2</text>
  </g>
</svg>
`

// parts parses a document and returns the local names of its elements in
// order, failing when it is not well-formed XML.
func elementNames(t *testing.T, doc []byte) []string {
	t.Helper()
	dec := xml.NewDecoder(bytes.NewReader(doc))
	var names []string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return names
		}
		if err != nil {
			t.Fatalf("layer is not well-formed XML: %v\n%s", err, doc)
		}
		if se, ok := tok.(xml.StartElement); ok {
			names = append(names, se.Name.Local)
		}
	}
}

func mustSplit(t *testing.T, data string) (*Scene, []problem.Problem) {
	t.Helper()
	s, ps, err := Split([]byte(data))
	if err != nil {
		t.Fatalf("Split returned an error: %v", err)
	}
	return s, ps
}

func TestSplitSample(t *testing.T) {
	s, ps := mustSplit(t, sample)
	if len(ps) != 0 {
		t.Fatalf("problems: %v", ps)
	}
	if s.ViewBox != (ViewBox{Width: 1600, Height: 900}) {
		t.Errorf("view box: %+v", s.ViewBox)
	}
	if got := strings.Join(s.Actors, ","); got != "pizza,slice,label-half" {
		t.Errorf("actors: %s", got)
	}
	type want struct {
		actor    string
		names    []string // elements in the layer after the root, defs and style
		contains []string // byte-for-byte pieces
	}
	wants := []want{
		{"", []string{"rect", "circle"}, []string{`<rect width="1600" height="900" fill="url(#sky)"/>`}},
		{"pizza", []string{"g", "g", "circle", "use"}, []string{`<use xlink:href="#sky"/>`, `<!-- the pizza -->`}},
		{"", []string{"g", "rect"}, []string{`<rect x="1" y="1" width="2" height="2"/>`}},
		{"slice", []string{"g", "path"}, []string{`<path d="M0 0 L10 10"/>`}},
		{"label-half", []string{"g", "text"}, []string{`Fish &amp; chips 1/2`}},
	}
	if len(s.Layers) != len(wants) {
		t.Fatalf("got %d layers, want %d", len(s.Layers), len(wants))
	}
	root := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 1600 900" width="1600" height="900">`
	for i, w := range wants {
		l := s.Layers[i]
		if l.ActorID != w.actor {
			t.Errorf("layer %d: actor %q, want %q", i, l.ActorID, w.actor)
		}
		svg := string(l.SVG)
		if !strings.HasPrefix(svg, root+"\n") {
			t.Errorf("layer %d does not start with the root tag:\n%s", i, svg)
		}
		if !strings.HasSuffix(svg, "</svg>\n") {
			t.Errorf("layer %d does not end with </svg>", i)
		}
		for _, c := range w.contains {
			if !strings.Contains(svg, c) {
				t.Errorf("layer %d lacks %q:\n%s", i, c, svg)
			}
		}
		names := elementNames(t, l.SVG)
		// svg, defs, linearGradient, stop, stop, style, then the layer's own.
		shared := []string{"svg", "defs", "linearGradient", "stop", "stop", "style"}
		if got := strings.Join(names, ","); got != strings.Join(append(shared, w.names...), ",") {
			t.Errorf("layer %d elements: %s", i, got)
		}
		if strings.Contains(svg, "<title>") || strings.Contains(svg, "<?xml") {
			t.Errorf("layer %d keeps the title or the declaration", i)
		}
	}
	// Defs come before the layer's elements, in every layer.
	for i, l := range s.Layers {
		svg := string(l.SVG)
		if strings.Index(svg, "<defs>") > strings.Index(svg, "<g") && strings.Contains(svg, "<g") {
			t.Errorf("layer %d: defs after content", i)
		}
	}
}

func TestSplitOrderAndMerging(t *testing.T) {
	doc := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 4 3">
<rect id="a"/><circle/><g id="x"/><g id="y"/><rect/><g id="z"/>
</svg>`
	s, ps := mustSplit(t, doc)
	if len(ps) != 0 {
		t.Fatalf("problems: %v", ps)
	}
	var got []string
	for _, l := range s.Layers {
		got = append(got, "["+l.ActorID+"]")
	}
	if strings.Join(got, "") != "[][x][y][][z]" {
		t.Errorf("layers: %v", got)
	}
}

func TestSplitNestedGroupsStayInActor(t *testing.T) {
	doc := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1600 900">
<g id="outer"><g id="inner"><g id="Bad Id"><rect/></g></g></g>
</svg>`
	s, ps := mustSplit(t, doc)
	if len(ps) != 0 {
		t.Fatalf("problems: %v", ps)
	}
	if len(s.Layers) != 1 || s.Layers[0].ActorID != "outer" || len(s.Actors) != 1 {
		t.Fatalf("layers: %+v actors: %v", s.Layers, s.Actors)
	}
	if got := strings.Join(elementNames(t, s.Layers[0].SVG), ","); got != "svg,g,g,g,rect" {
		t.Errorf("elements: %s", got)
	}
}

func TestSplitNamespacedRoot(t *testing.T) {
	doc := `<s:svg xmlns:s="http://www.w3.org/2000/svg" viewBox="0 0 1600 900"><s:g id="a"><s:rect/></s:g></s:svg>`
	s, ps := mustSplit(t, doc)
	if len(ps) != 0 {
		t.Fatalf("problems: %v", ps)
	}
	if len(s.Layers) != 1 || !strings.HasSuffix(string(s.Layers[0].SVG), "</s:svg>\n") {
		t.Fatalf("layers: %s", s.Layers[0].SVG)
	}
	elementNames(t, s.Layers[0].SVG)
}

func TestSplitInertElementsDropped(t *testing.T) {
	doc := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape" viewBox="0 0 1600 900">
<rect/><metadata>m</metadata><inkscape:namedview id="nv"/><desc>d</desc><circle/>
</svg>`
	s, ps := mustSplit(t, doc)
	if len(ps) != 0 {
		t.Fatalf("problems: %v", ps)
	}
	if len(s.Layers) != 1 {
		t.Fatalf("static run split by inert elements: %d layers", len(s.Layers))
	}
	if got := strings.Join(elementNames(t, s.Layers[0].SVG), ","); got != "svg,rect,circle" {
		t.Errorf("elements: %s", got)
	}
}

func TestSplitEmptyScene(t *testing.T) {
	s, ps := mustSplit(t, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1600 900"/>`)
	if len(ps) != 0 || s == nil || len(s.Layers) != 0 {
		t.Fatalf("scene %+v problems %v", s, ps)
	}
}

// has reports whether ps holds exactly one problem with the path and a part of
// the message.
func expectProblems(t *testing.T, ps []problem.Problem, want ...problem.Problem) {
	t.Helper()
	if len(ps) != len(want) {
		t.Fatalf("got %d problems, want %d:\n%v", len(ps), len(want), ps)
	}
	for i, w := range want {
		g := ps[i]
		if g.Path != w.Path || g.Warning != w.Warning || !strings.Contains(g.Message, w.Message) {
			t.Errorf("problem %d:\n got  %v\n want path %q warning %v message containing %q", i, g, w.Path, w.Warning, w.Message)
		}
	}
}

const svgOpen = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 1600 900">`

func TestForbiddenContent(t *testing.T) {
	tests := []struct {
		name string
		body string // children of the root
		path string
		msg  string
	}{
		{"script", `<script>alert(1)</script>`, "svg/script[1]", "<script> is not allowed"},
		{"script nested", `<g id="a"><rect/><script/></g>`, "svg/g[@id=a]/script[1]", "<script> is not allowed"},
		{"script other case", `<SCRIPT/>`, "svg/SCRIPT[1]", "not allowed"},
		{"animate", `<g id="a"><animate attributeName="x"/></g>`, "svg/g[@id=a]/animate[1]", "SMIL"},
		{"animateTransform", `<rect><animateTransform/></rect>`, "svg/rect[1]/animateTransform[1]", "SMIL"},
		{"animateMotion", `<rect><animateMotion/></rect>`, "svg/rect[1]/animateMotion[1]", "SMIL"},
		{"set", `<rect><set/></rect>`, "svg/rect[1]/set[1]", "SMIL"},
		{"discard", `<rect><discard/></rect>`, "svg/rect[1]/discard[1]", "SMIL"},
		{"foreignObject", `<foreignObject/>`, "svg/foreignObject[1]", "not allowed"},
		{"feImage", `<defs><filter id="f"><feImage/></filter></defs>`, "svg/defs[1]/filter[@id=f]/feImage[1]", "filter primitive"},
		{"feTile", `<defs><filter id="f"><feTile/></filter></defs>`, "svg/defs[1]/filter[@id=f]/feTile[1]", "filter primitive"},
		{"feTurbulence", `<defs><filter id="f"><feTurbulence/></filter></defs>`, "svg/defs[1]/filter[@id=f]/feTurbulence[1]", "filter primitive"},
		{"feConvolveMatrix", `<defs><filter id="f"><feConvolveMatrix/></filter></defs>`, "svg/defs[1]/filter[@id=f]/feConvolveMatrix[1]", "filter primitive"},
		{"feDisplacementMap", `<defs><filter id="f"><feDisplacementMap/></filter></defs>`, "svg/defs[1]/filter[@id=f]/feDisplacementMap[1]", "filter primitive"},
		{"feDiffuseLighting", `<defs><filter id="f"><feDiffuseLighting/></filter></defs>`, "svg/defs[1]/filter[@id=f]/feDiffuseLighting[1]", "lighting"},
		{"feSpecularLighting", `<defs><filter id="f"><feSpecularLighting><fePointLight/></feSpecularLighting></filter></defs>`, "svg/defs[1]/filter[@id=f]/feSpecularLighting[1]", "lighting"},
		{"feDistantLight", `<defs><filter id="f"><feDistantLight/></filter></defs>`, "svg/defs[1]/filter[@id=f]/feDistantLight[1]", "lighting"},
		{"feSpotLight", `<defs><filter id="f"><feSpotLight/></filter></defs>`, "svg/defs[1]/filter[@id=f]/feSpotLight[1]", "lighting"},
		{"event attribute", `<g id="a" onclick="x()"/>`, "svg/g[@id=a]/@onclick", "event attribute"},
		{"event attribute upper", `<rect onLoad="x()"/>`, "svg/rect[1]/@onLoad", "event attribute"},
		{"textLength", `<text textLength="100">a</text>`, "svg/text[1]/@textLength", "textLength"},
		{"vertical attribute", `<text writing-mode="vertical-rl">a</text>`, "svg/text[1]/@writing-mode", "vertical"},
		{"vertical tb", `<text writing-mode="tb">a</text>`, "svg/text[1]/@writing-mode", "vertical"},
		{"vertical style", `<text style="fill:red; writing-mode: vertical-lr">a</text>`, "svg/text[1]/@style", "vertical"},
		{"vertical style tb-rl", `<text style="writing-mode:tb-rl">a</text>`, "svg/text[1]/@style", "vertical"},
		{"vertical style element", `<style>text { writing-mode: vertical-rl }</style>`, "svg/style[1]", "vertical"},
		{"external href", `<use href="other.svg#a"/>`, "svg/use[1]/@href", "external references"},
		{"external xlink href", `<use xlink:href="http://example.com/a.svg#a"/>`, "svg/use[1]/@xlink:href", "external references"},
		{"empty href", `<use href=""/>`, "svg/use[1]/@href", "external references"},
		{"image external", `<image href="a.png"/>`, "svg/image[1]/@href", "data:image/png"},
		{"image fragment", `<image href="#a"/>`, "svg/image[1]/@href", "data:image/png"},
		{"image svg data", `<image href="data:image/svg+xml;base64,AAAA"/>`, "svg/image[1]/@href", "data:image/png"},
		{"use data html", `<use href="data:text/html;base64,AAAA"/>`, "svg/use[1]/@href", "external references"},
		{"url attribute", `<rect fill="url(http://example.com/a.svg#g)"/>`, "svg/rect[1]/@fill", "points outside the document"},
		{"url attribute quoted", `<rect filter="url('a.svg#f')"/>`, "svg/rect[1]/@filter", "points outside the document"},
		{"url style attribute", `<rect style="fill: url( other.svg#a )"/>`, "svg/rect[1]/@style", "points outside the document"},
		{"url style element", `<style>.a { background: url(https://example.com/x.png) }</style>`, "svg/style[1]", "points outside the document"},
		{"url second", `<rect style="fill: url(#a); stroke: url(x.svg#b)"/>`, "svg/rect[1]/@style", "points outside the document"},
		{"animation style", `<rect style="animation: spin 2s infinite"/>`, "svg/rect[1]/@style", "CSS animations"},
		{"animation longhand", `<style>.a{animation-name: spin}</style>`, "svg/style[1]", "CSS animations"},
		{"animation prefixed", `<style>.a{-webkit-animation: spin 1s}</style>`, "svg/style[1]", "CSS animations"},
		{"animation uppercase", `<style>.a{ANIMATION : spin 1s}</style>`, "svg/style[1]", "CSS animations"},
		{"animation escaped", `<style>.a{anim\61tion: spin 1s}</style>`, "svg/style[1]", "CSS animations"},
		{"animation escaped uppercase hex", `<style>.a{\41nimation: spin 1s}</style>`, "svg/style[1]", "CSS animations"},
		{"transition", `<style>.a{ transition: all 1s }</style>`, "svg/style[1]", "CSS animations"},
		{"transition style", `<rect style="transition-duration:2s"/>`, "svg/rect[1]/@style", "CSS animations"},
		{"keyframes", `<style>@keyframes spin { to { opacity: 0 } }</style>`, "svg/style[1]", "@keyframes"},
		{"keyframes prefixed", `<style>@-webkit-keyframes spin { to { opacity: 0 } }</style>`, "svg/style[1]", "@keyframes"},
		{"import", `<style>@import "other.css";</style>`, "svg/style[1]", "@import"},
		{"style cdata", `<style><![CDATA[ .a { animation: spin 1s } ]]></style>`, "svg/style[1]", "CSS animations"},
		{"doctype", "<!DOCTYPE svg [<!ENTITY x SYSTEM \"file:///etc/passwd\">]>\n" + svgOpen + `</svg>`, "", "DOCTYPE"},
		{"stylesheet instruction", `<?xml-stylesheet href="a.css"?>` + "\n" + svgOpen + `</svg>`, "", "processing instruction"},
		{"duplicate attribute", `<rect width="1" width="2"/>`, "svg/rect[1]/@width", "twice"},
		{"nth sibling", `<rect/><rect/><rect onclick="a"/>`, "svg/rect[3]/@onclick", "event attribute"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := svgOpen + tc.body + `</svg>`
			if strings.HasPrefix(tc.body, "<!DOCTYPE") || strings.HasPrefix(tc.body, "<?xml-stylesheet") {
				doc = tc.body
			}
			for _, mode := range []string{"scene", "picture"} {
				var ps []problem.Problem
				if mode == "scene" {
					_, p, err := Split([]byte(doc))
					if err != nil {
						t.Fatal(err)
					}
					ps = p
				} else {
					ps = CheckPicture([]byte(doc))
				}
				found := false
				for _, p := range ps {
					if p.Path == tc.path && !p.Warning && strings.Contains(p.Message, tc.msg) {
						found = true
					}
				}
				if !found {
					t.Errorf("%s: no problem at %q containing %q in %v", mode, tc.path, tc.msg, ps)
				}
			}
		})
	}
}

func TestAllowedContent(t *testing.T) {
	bodies := []string{
		`<defs><filter id="f"><feGaussianBlur stdDeviation="2"/><feOffset/><feFlood/><feComposite/><feMerge><feMergeNode/></feMerge><feColorMatrix/><feBlend/><feDropShadow/></filter></defs>`,
		`<image href="data:image/png;base64,iVBORw0KGgo=" width="1" height="1"/>`,
		`<image xlink:href="data:image/jpeg;base64,/9j/4AAQ" width="1" height="1"/>`,
		`<image href="DATA:IMAGE/PNG,abc"/>`,
		`<use href="#a"/><use xlink:href="#a"/>`,
		`<rect fill="url(#g)" stroke="url('#g')" filter='url("#f")' clip-path="url( #c )" style="fill:url(#g); opacity:.5"/>`,
		`<style>.a:hover { fill: red } .transition:hover { fill: blue } text { writing-mode: horizontal-tb } .b{fill:url(#g)}</style>`,
		`<style>/* animation: none */ .a{fill:red}</style>`,
		`<style><![CDATA[ .a > b { fill: red } ]]></style>`,
		`<text font-family="sans-serif" writing-mode="lr-tb" font-size="72">1/2 &#x41; &lt;</text>`,
		`<text class="animation" id="transition">animation: x</text>`,
		`<a href="#top"><rect/></a>`,
		`<rect data-onclick="x" id="one"/>`,
	}
	for i, body := range bodies {
		doc := svgOpen + body + `</svg>`
		if _, ps := mustSplit(t, doc); len(ps) != 0 {
			t.Errorf("body %d: scene problems: %v", i, ps)
		}
		if ps := CheckPicture([]byte(doc)); len(ps) != 0 {
			t.Errorf("body %d: picture problems: %v", i, ps)
		}
	}
}

func TestSyntaxAndRoot(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		path string
		msg  string
	}{
		{"empty", ``, "", "no <svg>"},
		{"only a comment", `<!-- a -->`, "", "no <svg>"},
		{"text", `hello`, "", "text outside"},
		{"not closed", svgOpen + `<g>`, "line 1", "not well-formed"},
		{"mismatched", svgOpen + "\n<g></h></svg>", "line 2", "not well-formed"},
		{"unknown entity", svgOpen + `<text>&nbsp;</text></svg>`, "line 1", "not well-formed"},
		{"bad encoding", `<?xml version="1.0" encoding="ISO-8859-1"?>` + svgOpen + `</svg>`, "", "not well-formed"},
		{"not utf-8", svgOpen + "<text>\xff</text></svg>", "line 1", "not well-formed"},
		{"bom", "\xef\xbb\xbf" + svgOpen + `</svg>`, "", "byte order mark"},
		{"wrong root", `<html/>`, "", "root element must be <svg"},
		{"no namespace", `<svg viewBox="0 0 4 3"/>`, "", "root element must be <svg"},
		{"two roots", svgOpen + `</svg>` + svgOpen + `</svg>`, "", "more than one root"},
		{"text after root", svgOpen + `</svg>x`, "", "text outside"},
		{"xml declaration late", " " + `<?xml version="1.0"?>` + svgOpen + `</svg>`, "", "first thing"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, ps, err := Split([]byte(tc.doc))
			if err != nil {
				t.Fatal(err)
			}
			if tc.name == "xml declaration late" {
				if s == nil {
					t.Error("scene should still be returned")
				}
			} else if s != nil {
				t.Errorf("scene should be nil, got %+v", s)
			}
			found := false
			for _, p := range ps {
				if p.Path == tc.path && strings.Contains(p.Message, tc.msg) && !p.Warning {
					found = true
				}
			}
			if !found {
				t.Errorf("no problem at %q containing %q in %v", tc.path, tc.msg, ps)
			}
			if p := CheckPicture([]byte(tc.doc)); !problem.HasErrors(p) {
				t.Errorf("CheckPicture reports no error")
			}
		})
	}
}

func TestViewBox(t *testing.T) {
	tests := []struct {
		name    string
		attr    string // the root's attributes
		path    string
		msg     string // empty: no problem
		aspects bool
	}{
		{"missing", ``, "svg", "no viewBox", false},
		{"three numbers", `viewBox="0 0 1600"`, "svg/@viewBox", "four numbers", false},
		{"five numbers", `viewBox="0 0 1600 900 1"`, "svg/@viewBox", "four numbers", false},
		{"not a number", `viewBox="0 0 16x0 900"`, "svg/@viewBox", "not a number", false},
		{"inf", `viewBox="0 0 inf 900"`, "svg/@viewBox", "not a number", false},
		{"hex", `viewBox="0 0 0x10 900"`, "svg/@viewBox", "not a number", false},
		{"overflow", `viewBox="0 0 1e999 900"`, "svg/@viewBox", "not a usable number", false},
		{"zero width", `viewBox="0 0 0 900"`, "svg/@viewBox", "greater than 0", false},
		{"negative height", `viewBox="0 0 1600 -900"`, "svg/@viewBox", "greater than 0", false},
		{"too tall", `viewBox="0 0 900 900"`, "svg/@viewBox", "between 4:3", true},
		{"portrait", `viewBox="0 0 300 400"`, "svg/@viewBox", "between 4:3", true},
		{"too wide", `viewBox="0 0 3000 900"`, "svg/@viewBox", "between 4:3", true},
		{"4:3", `viewBox="0 0 400 300"`, "", "", true},
		{"21:9", `viewBox="0 0 2100 900"`, "", "", true},
		{"commas", `viewBox="0,0,1600,900"`, "", "", true},
		{"mixed separators and offset", "viewBox=\" -10 , 5\n1600.5\t9e2 \"", "", "", true},
		{"16:9", `viewBox="0 0 1600 900"`, "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := `<svg xmlns="http://www.w3.org/2000/svg" ` + tc.attr + `><g id="a"/></svg>`
			s, ps := mustSplit(t, doc)
			if tc.msg == "" {
				if len(ps) != 0 || s == nil {
					t.Fatalf("problems %v scene %v", ps, s)
				}
				return
			}
			expectProblems(t, ps, problem.Problem{Path: tc.path, Message: tc.msg})
			if tc.aspects != (s != nil) {
				t.Errorf("scene nil = %v, want %v", s == nil, !tc.aspects)
			}
			// A picture needs a view box but any aspect ratio.
			pics := CheckPicture([]byte(doc))
			if tc.aspects && len(pics) != 0 {
				t.Errorf("picture: %v", pics)
			}
			if !tc.aspects {
				expectProblems(t, pics, problem.Problem{Path: tc.path, Message: tc.msg})
			}
		})
	}
	s, _ := mustSplit(t, `<svg xmlns="http://www.w3.org/2000/svg" viewBox=" -10 , 5
1600.5	9e2 "/>`)
	if s.ViewBox != (ViewBox{X: -10, Y: 5, Width: 1600.5, Height: 900}) {
		t.Errorf("view box: %+v", s.ViewBox)
	}
}

func TestCheckCover(t *testing.T) {
	tests := []struct {
		name    string
		viewBox string
		msg     string
		errs    bool
	}{
		{"3:4", "0 0 300 400", "", false},
		{"3:4 large", "0 0 1500 2000", "", false},
		{"square", "0 0 400 400", "3:4", false},
		{"landscape", "0 0 1600 900", "3:4", false},
		{"invalid", "0 0 0 400", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ps := CheckCover([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="` + tc.viewBox + `"><rect/></svg>`))
			switch {
			case tc.errs:
				if !problem.HasErrors(ps) {
					t.Errorf("want an error: %v", ps)
				}
			case tc.msg == "":
				if len(ps) != 0 {
					t.Errorf("problems: %v", ps)
				}
			default:
				expectProblems(t, ps, problem.Problem{Path: "svg/@viewBox", Message: tc.msg, Warning: true})
				if problem.HasErrors(ps) {
					t.Error("a cover aspect must be a warning")
				}
			}
		})
	}
	// The forbidden-content rules apply to covers too.
	ps := CheckCover([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 300 400"><script/></svg>`))
	expectProblems(t, ps, problem.Problem{Path: "svg/script[1]", Message: "<script>"})
}

func TestPictureHasNoActorRules(t *testing.T) {
	doc := svgOpen
	for i := 0; i < MaxActors+2; i++ {
		doc += `<g id="Same"/>`
	}
	doc += `</svg>`
	if ps := CheckPicture([]byte(doc)); len(ps) != 0 {
		t.Errorf("problems: %v", ps)
	}
	if ps := CheckCover([]byte(strings.Replace(doc, "0 0 1600 900", "0 0 300 400", 1))); len(ps) != 0 {
		t.Errorf("cover problems: %v", ps)
	}
}

func TestActorRules(t *testing.T) {
	many := svgOpen
	for i := 0; i < MaxActors+1; i++ {
		many += `<g id="a` + strconv.Itoa(i) + `"/>`
	}
	many += `</svg>`
	atLimit := svgOpen
	for i := 0; i < MaxActors; i++ {
		atLimit += `<g id="a` + strconv.Itoa(i) + `"/>`
	}
	atLimit += `</svg>`

	tests := []struct {
		name string
		doc  string
		want []problem.Problem
	}{
		{"at the limit", atLimit, nil},
		{"too many", many, []problem.Problem{{Path: "svg/g[@id=a64]", Message: "more than 64 actors"}}},
		{"uppercase", svgOpen + `<g id="Pizza"/></svg>`, []problem.Problem{{Path: "svg/g[@id=Pizza]", Message: "not valid"}}},
		{"underscore", svgOpen + `<g id="a_b"/></svg>`, []problem.Problem{{Path: "svg/g[@id=a_b]", Message: "not valid"}}},
		{"leading hyphen", svgOpen + `<g id="-a"/></svg>`, []problem.Problem{{Path: "svg/g[@id=-a]", Message: "not valid"}}},
		{"empty id", svgOpen + `<g id=""/></svg>`, []problem.Problem{{Path: "svg/g[1]", Message: "not valid"}}},
		{"too long", svgOpen + `<g id="` + strings.Repeat("a", 64) + `"/></svg>`, []problem.Problem{{Path: "svg/g[@id=" + strings.Repeat("a", 64) + "]", Message: "not valid"}}},
		{"longest", svgOpen + `<g id="` + strings.Repeat("a", 63) + `"/></svg>`, nil},
		{"duplicate", svgOpen + `<g id="a"/><rect/><g id="a"/></svg>`, []problem.Problem{{Path: "svg/g[@id=a]", Message: "unique"}}},
		{"inner ids are free", svgOpen + `<g id="a"><g id="A b"/><g id="a"/></g></svg>`, nil},
		{"top-level non-g ids are free", svgOpen + `<rect id="Rect"/><circle id="Rect"/></svg>`, nil},
		{"definition at top level", svgOpen + `<linearGradient id="g"/></svg>`, []problem.Problem{{Path: "svg/linearGradient[@id=g]", Message: "put it inside <defs>", Warning: true}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ps := mustSplit(t, tc.doc)
			expectProblems(t, ps, tc.want...)
		})
	}
}

func TestSizeLimits(t *testing.T) {
	pad := strings.Repeat("a", MaxBytes)
	if s, ps, _ := Split([]byte(svgOpen + `<text>` + pad + `</text></svg>`)); s != nil || len(ps) != 1 || !strings.Contains(ps[0].Message, "5 MiB") {
		t.Errorf("big file: %v %v", s, ps)
	}
	if ps := CheckPicture([]byte(pad + pad)); len(ps) != 1 {
		t.Errorf("big picture: %v", ps)
	}
	// Exactly at the limit is fine.
	body := `<text>`
	tail := `</text></svg>`
	fill := MaxBytes - len(svgOpen) - len(body) - len(tail)
	doc := svgOpen + body + strings.Repeat("a", fill) + tail
	if len(doc) != MaxBytes {
		t.Fatalf("test document is %d bytes", len(doc))
	}
	if s, ps, _ := Split([]byte(doc)); s == nil || len(ps) != 0 {
		t.Errorf("document at the limit: %v", ps)
	}
}

func TestDepthLimit(t *testing.T) {
	deep := func(n int) string {
		return svgOpen + strings.Repeat("<g>", n) + strings.Repeat("</g>", n) + `</svg>`
	}
	if s, ps := mustSplit(t, deep(MaxDepth-1)); s == nil || len(ps) != 0 {
		t.Errorf("deepest allowed: %v", ps)
	}
	s, ps := mustSplit(t, deep(MaxDepth))
	if s != nil || len(ps) != 1 || !strings.Contains(ps[0].Message, "nested more than") {
		t.Errorf("too deep: %v %v", s, ps)
	}
}

func TestLayerBytesLimit(t *testing.T) {
	// Defs of about 1 MB copied into 33 layers are over 32 MiB, 32 layers are not.
	doc := svgOpen + `<defs><text>` + strings.Repeat("a", 1040000) + `</text></defs>`
	for i := 0; i < 33; i++ {
		doc += `<g id="a` + strconv.Itoa(i) + `"/>`
	}
	doc += `</svg>`
	s, ps := mustSplit(t, doc)
	if s == nil || len(s.Layers) != 0 || len(s.Actors) != 33 {
		t.Fatalf("scene: %+v", s)
	}
	expectProblems(t, ps, problem.Problem{Path: "svg", Message: "copied into every layer"})
	// 32 layers are within the limit.
	ok := strings.Replace(doc, `<g id="a32"/>`, "", 1)
	if s, ps := mustSplit(t, ok); len(ps) != 0 || len(s.Layers) != 32 {
		t.Errorf("32 layers: %v", ps)
	}
}

func TestTooManyProblems(t *testing.T) {
	doc := svgOpen + strings.Repeat(`<script/>`, MaxProblems+50) + `</svg>`
	s, ps := mustSplit(t, doc)
	if s != nil {
		t.Error("scene should be nil when the problem list is cut")
	}
	if len(ps) != MaxProblems+1 || !strings.Contains(ps[len(ps)-1].Message, "more than") {
		t.Fatalf("got %d problems, last: %v", len(ps), ps[len(ps)-1])
	}
}

func TestLineNumbers(t *testing.T) {
	doc := svgOpen + "\n\n<rect\n onclick=\"x\"/>\n</svg>"
	_, ps := mustSplit(t, doc)
	expectProblems(t, ps, problem.Problem{Path: "svg/rect[1]/@onclick", Message: "(line 4)"})
}

func TestCheckTracks(t *testing.T) {
	s, ps := mustSplit(t, sample)
	if len(ps) != 0 {
		t.Fatal(ps)
	}
	tracks := []timeline.Track{
		{Actor: "pizza"},
		{Actor: "nobody"},
		{Actor: "label-half", RevealBox: []float64{650, 720, 300, 110}},
		{Actor: "slice", RevealBox: []float64{1500, 0, 101, 10}},
		{Actor: "slice", RevealBox: []float64{-1, 0, 10, 10}},
		{Actor: "slice", RevealBox: []float64{0, 0, 1600, 900}},
		{Actor: "slice", RevealBox: []float64{0, 890, 10, 11}},
		{Actor: "ghost", RevealBox: []float64{0, 0, 2000, 10}},
		{Actor: "slice", RevealBox: []float64{1, 2}},
	}
	got := CheckTracks(s, tracks)
	expectProblems(t, got,
		problem.Problem{Path: "tracks[1].actor", Message: `no actor "nobody"`},
		problem.Problem{Path: "tracks[3].reveal_box", Message: "not inside the view box"},
		problem.Problem{Path: "tracks[4].reveal_box", Message: "not inside the view box"},
		problem.Problem{Path: "tracks[6].reveal_box", Message: "not inside the view box"},
		problem.Problem{Path: "tracks[7].actor", Message: `no actor "ghost"`},
		problem.Problem{Path: "tracks[7].reveal_box", Message: "not inside the view box"},
	)
	if got := CheckTracks(nil, tracks); got != nil {
		t.Errorf("nil scene: %v", got)
	}
	if got := CheckTracks(s, nil); len(got) != 0 {
		t.Errorf("no tracks: %v", got)
	}
}

func TestCheckTracksOffsetViewBox(t *testing.T) {
	s, ps := mustSplit(t, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="-100 -50 400 300"><g id="a"/></svg>`)
	if len(ps) != 0 {
		t.Fatal(ps)
	}
	tracks := []timeline.Track{
		{Actor: "a", RevealBox: []float64{-100, -50, 400, 300}},
		{Actor: "a", RevealBox: []float64{0, 0, 400, 100}},
	}
	expectProblems(t, CheckTracks(s, tracks), problem.Problem{Path: "tracks[1].reveal_box", Message: "not inside"})
}

func TestProblemMessagesAreReadable(t *testing.T) {
	_, ps := mustSplit(t, svgOpen+`<g id="a" onclick="x"/></svg>`)
	got := problem.InFile("lessons/l1/scene.svg", ps)[0].String()
	want := `lessons/l1/scene.svg#svg/g[@id=a]/@onclick: "onclick" is an event attribute and is not allowed: scenes cannot run code (line 1)`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

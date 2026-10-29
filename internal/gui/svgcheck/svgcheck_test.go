package svgcheck

import (
	"context"
	"os"
	"testing"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/glutter/svg"
	"github.com/rm4n0s/radio-tutor/internal/core/archive"
	"github.com/rm4n0s/radio-tutor/internal/core/scene"
)

// A Checker is what the archive package asks for.
var _ archive.RenderChecker = Checker{}

const header = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1600 900">`

func TestCheckSVG(t *testing.T) {
	tests := []struct {
		name  string
		svg   string
		route string // empty when the SVG is drawn
	}{
		{"shapes", header + `<rect width="100" height="50" fill="#336"/><circle cx="50" cy="50" r="20"/></svg>`, ""},
		{"text uses the bundled font", header + `<text x="10" y="40" font-size="32">1/2 + 1/4</text></svg>`, ""},
		{"gradient and group", header + `<defs><linearGradient id="g"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient></defs><g opacity="0.5"><rect width="9" height="9" fill="url(#g)"/></g></svg>`, ""},
		{"empty picture is drawn", header + `</svg>`, ""},
		{"xml not well formed", header + `<g></svg>`, "Checker.CheckSVG.SvgCheckParseFailed"},
		{"not an svg root", `<html xmlns="http://www.w3.org/1999/xhtml"/>`, "Checker.CheckSVG.SvgCheckParseFailed"},
		{"nested viewBox with a negative size", header + `<svg viewBox="0 0 -10 20" width="5" height="5"><rect width="3" height="3"/></svg></svg>`, "Checker.CheckSVG.SvgCheckParseFailed"},
		{"marker viewBox with a negative size", header + `<defs><marker id="m" viewBox="0 0 10 -10"><path d="M0 0"/></marker></defs><path d="M0 0L9 9" marker-end="url(#m)"/></svg>`, "Checker.CheckSVG.SvgCheckParseFailed"},
		{"a viewBox of no area", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 0 0"/>`, "Checker.CheckSVG.SvgCheckNoSize"},
		{"not xml", `this is not xml`, "Checker.CheckSVG.SvgCheckParseFailed"},
		{"empty input", ``, "Checker.CheckSVG.SvgCheckParseFailed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Checker{}.CheckSVG(context.Background(), "test.svg", []byte(tc.svg))
			if tc.route == "" {
				if err != nil {
					t.Fatalf("CheckSVG: %v", err)
				}
				return
			}
			appErr, ok := errors.FromError(err)
			if !ok {
				t.Fatalf("expected *errors.Error, got %T (%v)", err, err)
			}
			if !appErr.HasRoute(tc.route) {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
			if appErr.Metadata["name"] != "test.svg" {
				t.Errorf("name metadata = %v", appErr.Metadata["name"])
			}
		})
	}
}

// TestCheckSVGCancelled checks that a done context stops the check.
func TestCheckSVGCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Checker{}.CheckSVG(ctx, "test.svg", []byte(header+`</svg>`))
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T", err)
	}
	if !appErr.HasRoute("Checker.CheckSVG.SvgCheckCanceled") {
		t.Errorf("unexpected failure path: %s", appErr.Route())
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the error does not wrap context.Canceled")
	}
}

// TestGlutterErrorIsKept checks that the message names Glutter's reason.
func TestGlutterErrorIsKept(t *testing.T) {
	err := Checker{}.CheckSVG(context.Background(), "x.svg", []byte(header+`<svg viewBox="0 0 -1 1"/></svg>`))
	if err == nil {
		t.Fatal("expected an error")
	}
	var inner *errors.Error
	if !errors.As(errors.Unwrap(err), &inner) || inner.Tag != svg.TagSvgInvalidValue {
		t.Errorf("original error = %v, want tag %s", errors.Unwrap(err), svg.TagSvgInvalidValue)
	}
}

// TestCoreAcceptsWhatGlutterRefuses documents the gap this package closes: the
// core's scene checks pass a nested viewBox with a negative size, Glutter
// cannot draw it.
func TestCoreAcceptsWhatGlutterRefuses(t *testing.T) {
	data, err := os.ReadFile("testdata/negative-viewbox.svg")
	if err != nil {
		t.Fatal(err)
	}
	sc, ps, err := scene.Split(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range ps {
		if !p.Warning {
			t.Fatalf("the core refuses the scene: %v", p)
		}
	}
	for _, l := range sc.Layers {
		if err := (Checker{}).CheckSVG(context.Background(), "layer", l.SVG); err == nil {
			t.Errorf("Glutter drew a layer with a negative viewBox: %s", l.SVG)
		}
	}
}

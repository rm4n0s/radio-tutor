package phone

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
)

//go:embed assets
var assets embed.FS

// pageTemplates holds the "page" and "message" templates. html/template
// escapes every value for its place in the HTML.
var pageTemplates = template.Must(template.ParseFS(assets, "assets/page.html"))

// staticFile is a file served under /static/.
type staticFile struct {
	contentType string
	data        []byte
}

// readStatic reads the CSS and JS from fsys (the embedded assets).
func readStatic(fsys fs.FS) (map[string]staticFile, error) {
	files := map[string]string{
		"page.css": "text/css; charset=utf-8",
		"page.js":  "text/javascript; charset=utf-8",
	}
	out := make(map[string]staticFile, len(files))
	for name, ct := range files {
		data, err := fs.ReadFile(fsys, "assets/"+name)
		if err != nil {
			return nil, errors.NewErr("PhoneAssetMissing", err).SetMetadata("name", name)
		}
		out[name] = staticFile{contentType: ct, data: data}
	}
	return out, nil
}

// pageView is what the page template renders for one token.
type pageView struct {
	Lang      string
	Title     string
	MaxPhotos int
	Exercises []exerciseView
	// Text is the "phone." part of the catalog; the template uses it and the
	// script reads it as JSON.
	Text map[string]string
}

type exerciseView struct {
	Label, Title string
	// Lines are the lines of the prompt.
	Lines []string
}

// newPageView prepares the page of a token. The page is rendered from it for
// every GET, so the work is done once.
func newPageView(p PageData) pageView {
	cat := i18n.For(p.Lang)
	text := map[string]string{}
	for _, key := range i18n.Keys(cat.Lang()) {
		if strings.HasPrefix(key, "phone.") {
			text[key] = cat.T(key)
		}
	}
	v := pageView{Lang: string(cat.Lang()), Title: p.Title, MaxPhotos: MaxPhotos, Text: text}
	for _, ex := range p.Exercises {
		v.Exercises = append(v.Exercises, exerciseView{
			Label: ex.Label,
			Title: ex.Title,
			Lines: promptLines(ex.Prompt),
		})
	}
	return v
}

// promptLines splits a prompt into lines, dropping empty text at the ends.
func promptLines(prompt string) []string {
	prompt = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(prompt, "\r\n", "\n"), "\r", "\n"))
	if prompt == "" {
		return nil
	}
	return strings.Split(prompt, "\n")
}

// renderPage writes the page of a token.
func renderPage(t *template.Template, w http.ResponseWriter, v pageView) error {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "page", v); err != nil {
		return errors.NewErr("PhonePageRenderFailed", err)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
	return nil
}

// renderMessage writes a small page with a title and a text, with the status.
func renderMessage(t *template.Template, w http.ResponseWriter, status int, lang i18n.Lang, titleKey, bodyKey string) error {
	cat := i18n.For(lang)
	data := struct{ Lang, Title, Body string }{string(cat.Lang()), cat.T(titleKey), cat.T(bodyKey)}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "message", data); err != nil {
		return errors.NewErr("PhoneMessageRenderFailed", err)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
	return nil
}

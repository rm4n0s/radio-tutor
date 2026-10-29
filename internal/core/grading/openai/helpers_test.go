package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
)

const testKey = "sk-test-SECRET-0123456789"

// seen is one request the test server received.
type seen struct {
	Method string
	Path   string
	Header http.Header
	Body   []byte
}

// recorder keeps the requests a test server received.
type recorder struct {
	mu   sync.Mutex
	reqs []seen
}

func (r *recorder) add(s seen) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reqs = append(r.reqs, s)
	return len(r.reqs)
}

func (r *recorder) all() []seen {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]seen(nil), r.reqs...)
}

// body decodes the JSON body of the i-th request into a generic map.
func (r *recorder) body(t *testing.T, i int) map[string]any {
	t.Helper()
	reqs := r.all()
	if i >= len(reqs) {
		t.Fatalf("only %d requests, want index %d", len(reqs), i)
	}
	var m map[string]any
	if err := json.Unmarshal(reqs[i].Body, &m); err != nil {
		t.Fatalf("request body is not JSON: %v\n%s", err, reqs[i].Body)
	}
	return m
}

// newServer starts a server that records each request and calls h with the
// 1-based request number.
func newServer(t *testing.T, h func(w http.ResponseWriter, r *http.Request, n int)) (*httptest.Server, *recorder) {
	t.Helper()
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading the request body: %v", err)
		}
		n := rec.add(seen{Method: r.Method, Path: r.URL.Path, Header: r.Header.Clone(), Body: body})
		h(w, r, n)
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

// replyJSON writes a Chat Completions answer with the given content.
func replyJSON(w http.ResponseWriter, model, content, finish string) {
	w.Header().Set("Content-Type", "application/json")
	out := map[string]any{
		"choices": []any{map[string]any{
			"message":       map[string]any{"role": "assistant", "content": content},
			"finish_reason": finish,
		}},
	}
	if model != "" {
		out["model"] = model
	}
	_ = json.NewEncoder(w).Encode(out)
}

// okHandler answers every request with a fixed text.
func okHandler(text string) func(http.ResponseWriter, *http.Request, int) {
	return func(w http.ResponseWriter, _ *http.Request, _ int) { replyJSON(w, "", text, "stop") }
}

// testConfig is a config for the server at url.
func testConfig(url string) Config {
	return Config{Name: "Test AI", BaseURL: url, Model: "vision-1", APIKey: testKey, Timeout: 5 * time.Second, Version: "1.2.3"}
}

// newProvider builds a Provider and fails the test on error.
func newProvider(t *testing.T, cfg Config) *Provider {
	t.Helper()
	p, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// testRequest is a request with two exercises and two photos.
func testRequest() grading.Request {
	return grading.Request{
		Title: "Fractions", Language: "en", Ages: [2]int{8, 10},
		Exercises: []cassette.Exercise{
			{ID: "a", Label: "1", Title: "A", Prompt: "p", ReferenceSolution: "r", Grading: "g"},
			{ID: "b", Label: "2", Title: "B", Prompt: "p", ReferenceSolution: "r", Grading: "g"},
		},
		Photos: [][]byte{{0xff, 0xd8, 1, 2, 3}, {0xff, 0xd8, 9, 8}},
	}
}

// grade calls the provider with a background context and no hint.
func grade(t *testing.T, p *Provider) (string, grading.Meta, error) {
	t.Helper()
	return p.Grade(context.Background(), testRequest(), nil)
}

// wantRoute fails unless err is an *errors.Error whose route contains route;
// it returns the error's metadata.
func wantRoute(t *testing.T, err error, route string) map[string]any {
	t.Helper()
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T: %v", err, err)
	}
	if !appErr.HasRoute(route) {
		t.Errorf("unexpected failure path: %s (want %s)", appErr.Route(), route)
	}
	return appErr.Metadata
}

// lockedBuffer is a bytes.Buffer safe for a logger used by several goroutines.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

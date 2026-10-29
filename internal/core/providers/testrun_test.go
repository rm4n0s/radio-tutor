package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rm4n0s/radio-tutor/internal/core/grading"
)

const goodReply = `{"page_notes":"","results":[{"exercise_id":"sum","verdict":"correct","transcription":"2 + 3 = 5","feedback_for_child":"Well done!","notes_for_parent":"The child wrote 5."}]}`

// chatServer is a test server that speaks Chat Completions and records the
// request bodies and headers.
type chatServer struct {
	*httptest.Server
	mu     sync.Mutex
	bodies []map[string]any
	auth   []string
}

func (c *chatServer) requests() ([]map[string]any, []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]map[string]any(nil), c.bodies...), append([]string(nil), c.auth...)
}

// newChatServer starts a server; h answers the n-th request (counted from 1).
func newChatServer(t *testing.T, h func(w http.ResponseWriter, n int)) *chatServer {
	t.Helper()
	cs := &chatServer{}
	cs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading the request: %v", err)
		}
		var body map[string]any
		if err := json.Unmarshal(data, &body); err != nil {
			t.Errorf("request is not JSON: %v", err)
		}
		if r.URL.Path != "/v1/chat/completions" || r.Method != http.MethodPost {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		cs.mu.Lock()
		cs.bodies = append(cs.bodies, body)
		cs.auth = append(cs.auth, r.Header.Get("Authorization"))
		n := len(cs.bodies)
		cs.mu.Unlock()
		h(w, n)
	}))
	t.Cleanup(cs.Close)
	return cs
}

func reply(w http.ResponseWriter, model, content string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"model": model,
		"choices": []any{map[string]any{
			"message":       map[string]any{"role": "assistant", "content": content},
			"finish_reason": "stop",
		}},
	})
}

func serverConfig(cs *chatServer) Config {
	c := validConfig("Test AI")
	c.BaseURL = cs.URL + "/v1"
	c.Model = "vision-1"
	c.TimeoutSeconds = 10
	return c
}

func TestTestSuccess(t *testing.T) {
	cs := newChatServer(t, func(w http.ResponseWriter, _ int) { reply(w, "vision-1-2026", goodReply) })
	res, err := Test(context.Background(), serverConfig(cs))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || res.Err != nil || res.Verdict != grading.VerdictCorrect || res.Model != "vision-1-2026" || res.Downgraded != "" {
		t.Errorf("result = %+v", res)
	}
	if res.Duration <= 0 || res.Duration > 10*time.Second {
		t.Errorf("duration = %v", res.Duration)
	}

	bodies, auth := cs.requests()
	if len(bodies) != 1 {
		t.Fatalf("%d requests, want 1", len(bodies))
	}
	if auth[0] != "Bearer sk-secret-123" {
		t.Errorf("Authorization = %q", auth[0])
	}
	b := bodies[0]
	if b["model"] != "vision-1" {
		t.Errorf("model = %v", b["model"])
	}
	rf, _ := b["response_format"].(map[string]any)
	if rf["type"] != "json_schema" {
		t.Errorf("response_format = %v", b["response_format"])
	}
	msgs, _ := b["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("%d messages", len(msgs))
	}
	user, _ := msgs[1].(map[string]any)
	parts, _ := user["content"].([]any)
	if len(parts) != 2 {
		t.Fatalf("user message has %d parts, want text and one image", len(parts))
	}
	text, _ := parts[0].(map[string]any)["text"].(string)
	for _, want := range []string{`"2 + 3 = ?"`, `"5"`, "Photos: 1"} {
		if !strings.Contains(text, want) {
			t.Errorf("the exercise text lacks %s:\n%s", want, text)
		}
	}
	img, _ := parts[1].(map[string]any)["image_url"].(map[string]any)
	if url, _ := img["url"].(string); !strings.HasPrefix(url, "data:image/jpeg;base64,/9j/") {
		t.Errorf("image url = %.40q", url)
	}
}

func TestTestWithoutKeyAndLenientModes(t *testing.T) {
	tests := []struct {
		name  string
		mode  grading.StructuredOutput
		reply string
	}{
		{"json_object, fenced reply", grading.StructuredJSONObject, "```json\n" + goodReply + "\n```"},
		{"none, text around the reply", grading.StructuredNone, "Here is the grading: " + goodReply + " Done."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := newChatServer(t, func(w http.ResponseWriter, _ int) { reply(w, "", tt.reply) })
			cfg := serverConfig(cs)
			cfg.APIKey, cfg.StructuredOutput = "", tt.mode
			res, err := Test(context.Background(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			if !res.OK || res.Verdict != grading.VerdictCorrect || res.Model != "vision-1" {
				t.Errorf("result = %+v", res)
			}
			if _, auth := cs.requests(); auth[0] != "" {
				t.Errorf("Authorization = %q, want none for an empty key", auth[0])
			}
		})
	}
}

func TestTestDowngrade(t *testing.T) {
	cs := newChatServer(t, func(w http.ResponseWriter, n int) {
		if n == 1 {
			http.Error(w, `{"error":"response_format json_schema is not supported"}`, http.StatusBadRequest)
			return
		}
		reply(w, "", "```json\n"+goodReply+"\n```")
	})
	res, err := Test(context.Background(), serverConfig(cs))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || res.Downgraded != grading.StructuredJSONObject {
		t.Errorf("result = %+v, want OK with a downgrade to json_object", res)
	}
	if bodies, _ := cs.requests(); len(bodies) != 2 {
		t.Errorf("%d requests, want 2", len(bodies))
	}
}

func TestTestProviderFailures(t *testing.T) {
	tests := []struct {
		name    string
		handler func(w http.ResponseWriter, n int)
		route   string
		wantReq int
	}{
		{"wrong key", func(w http.ResponseWriter, _ int) { http.Error(w, `{"error":"bad key"}`, http.StatusUnauthorized) }, "OpenAIAuth", 1},
		{"unknown model", func(w http.ResponseWriter, _ int) { http.Error(w, `{"error":"no such model"}`, http.StatusNotFound) }, "OpenAINotFound", 1},
		{"server error", func(w http.ResponseWriter, _ int) { http.Error(w, "boom", http.StatusInternalServerError) }, "OpenAIServer", 1},
		{"not a chat server", func(w http.ResponseWriter, _ int) { _, _ = io.WriteString(w, "<html>hello</html>") }, "OpenAIBodyInvalid", 1},
		{"model answers nonsense twice", func(w http.ResponseWriter, _ int) { reply(w, "", "I cannot read that") }, "GradingReplyInvalid", 2},
		{"reply for another exercise", func(w http.ResponseWriter, _ int) {
			reply(w, "", `{"page_notes":"","results":[{"exercise_id":"other","verdict":"correct","transcription":"","feedback_for_child":"","notes_for_parent":""}]}`)
		}, "", 1}, // unknown ids are dropped, the asked one becomes not_found: still a valid reply
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := newChatServer(t, tt.handler)
			res, err := Test(context.Background(), serverConfig(cs))
			if err != nil {
				t.Fatalf("a failing provider is a result, not an error: %v", err)
			}
			if bodies, _ := cs.requests(); len(bodies) != tt.wantReq {
				t.Errorf("%d requests, want %d", len(bodies), tt.wantReq)
			}
			if tt.route == "" {
				if !res.OK || res.Verdict != grading.VerdictNotFound {
					t.Errorf("result = %+v, want OK and not_found", res)
				}
				return
			}
			if res.OK || res.Verdict != "" || res.Model != "vision-1" {
				t.Errorf("result = %+v, want a failure that names the configured model", res)
			}
			wantRoute(t, res.Err, tt.route)
			if strings.Contains(res.Err.Error(), "sk-secret-123") {
				t.Error("the error leaks the API key")
			}
		})
	}
}

func TestTestUnreachableServer(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	cfg := validConfig("Gone")
	cfg.BaseURL = srv.URL + "/v1"
	srv.Close() // the port now refuses connections
	res, err := Test(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Fatal("OK for a server that is not there")
	}
	wantRoute(t, res.Err, "OpenAINetwork")
}

func TestTestTimeoutAndCancel(t *testing.T) {
	release := make(chan struct{})
	cs := newChatServer(t, func(w http.ResponseWriter, _ int) {
		select {
		case <-release:
		case <-time.After(5 * time.Second):
		}
		reply(w, "", goodReply)
	})
	t.Cleanup(func() { close(release) })

	t.Run("provider timeout", func(t *testing.T) {
		cfg := serverConfig(cs)
		cfg.TimeoutSeconds = 1
		res, err := Test(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		if res.OK || res.Duration < 900*time.Millisecond || res.Duration > 4*time.Second {
			t.Errorf("result = %+v", res)
		}
		wantRoute(t, res.Err, "OpenAITimeout")
	})
	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		res, err := Test(ctx, serverConfig(cs))
		if err != nil {
			t.Fatal(err)
		}
		if res.OK {
			t.Fatal("OK after the context ended")
		}
		wantRoute(t, res.Err, "OpenAITimeout") // a deadline of the caller's ctx
	})
	t.Run("cancelled before the image is prepared", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := Test(ctx, serverConfig(cs))
		if err == nil {
			t.Fatal("expected an error for a test that could not be made")
		}
		wantRoute(t, err, "Test->NormalizePhoto")
	})
}

func TestTestInvalidConfig(t *testing.T) {
	c := validConfig("X")
	c.Kind = ""
	res, err := Test(context.Background(), c)
	wantRoute(t, err, "Test->Build->normalize.ProviderKindInvalid")
	if res.OK || res.Err != nil {
		t.Errorf("result = %+v", res)
	}
}

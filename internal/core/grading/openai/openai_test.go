package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/grading/fake"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		wantRoute string
		wantURL   string
		wantMode  grading.StructuredOutput
	}{
		{"defaults", Config{Name: "n", BaseURL: "http://localhost:11434/v1", Model: "m"}, "", "http://localhost:11434/v1/chat/completions", grading.StructuredJSONSchema},
		{"trailing slash", Config{BaseURL: "https://api.openai.com/v1/", Model: "m"}, "", "https://api.openai.com/v1/chat/completions", grading.StructuredJSONSchema},
		{"several slashes and spaces", Config{BaseURL: " https://x.test/v1// ", Model: "m", StructuredOutput: grading.StructuredNone}, "", "https://x.test/v1/chat/completions", grading.StructuredNone},
		{"json_object", Config{BaseURL: "http://h", Model: "m", StructuredOutput: grading.StructuredJSONObject}, "", "http://h/chat/completions", grading.StructuredJSONObject},
		{"bad mode", Config{BaseURL: "http://h", Model: "m", StructuredOutput: "xml"}, "New.OpenAIConfigStructuredOutput", "", ""},
		{"no model", Config{BaseURL: "http://h", Model: "  "}, "New.OpenAIConfigModel", "", ""},
		{"unparsable URL", Config{BaseURL: "http://[::1", Model: "m"}, "New.OpenAIConfigBaseURLParse", "", ""},
		{"empty URL", Config{Model: "m"}, "New.OpenAIConfigBaseURL", "", ""},
		{"ftp", Config{BaseURL: "ftp://h/v1", Model: "m"}, "New.OpenAIConfigBaseURL", "", ""},
		{"no host", Config{BaseURL: "http:///v1", Model: "m"}, "New.OpenAIConfigBaseURL", "", ""},
		{"credentials in URL", Config{BaseURL: "http://user:pw@h/v1", Model: "m"}, "New.OpenAIConfigBaseURL", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(tt.cfg, nil)
			if tt.wantRoute != "" {
				meta := wantRoute(t, err, tt.wantRoute)
				if strings.Contains(fmt.Sprint(meta), "pw") {
					t.Errorf("the password is in the metadata: %v", meta)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if p.endpoint != tt.wantURL {
				t.Errorf("endpoint = %q, want %q", p.endpoint, tt.wantURL)
			}
			if p.StructuredOutput() != tt.wantMode {
				t.Errorf("mode = %q, want %q", p.StructuredOutput(), tt.wantMode)
			}
			if p.cfg.Timeout != DefaultTimeout || p.cfg.Version != "dev" {
				t.Errorf("defaults: timeout %v version %q", p.cfg.Timeout, p.cfg.Version)
			}
			if p.Name() != tt.cfg.Name {
				t.Errorf("Name = %q", p.Name())
			}
		})
	}
}

func TestRequestShape(t *testing.T) {
	srv, rec := newServer(t, okHandler("{}"))
	cfg := testConfig(srv.URL + "/v1/")
	p := newProvider(t, cfg)
	req := testRequest()

	text, meta, err := p.Grade(context.Background(), req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if text != "{}" || meta.FinishReason != "stop" || meta.Model != "vision-1" {
		t.Errorf("got %q %+v", text, meta)
	}
	reqs := rec.all()
	if len(reqs) != 1 {
		t.Fatalf("%d requests", len(reqs))
	}
	got := reqs[0]
	if got.Method != http.MethodPost || got.Path != "/v1/chat/completions" {
		t.Errorf("%s %s", got.Method, got.Path)
	}
	for name, want := range map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + testKey,
		"User-Agent":    "radio-tutor/1.2.3",
	} {
		if v := got.Header.Get(name); v != want {
			t.Errorf("%s = %q, want %q", name, v, want)
		}
	}

	body := rec.body(t, 0)
	if body["model"] != "vision-1" || body["max_tokens"] != float64(8000) {
		t.Errorf("model/max_tokens: %v %v", body["model"], body["max_tokens"])
	}
	if _, ok := body["temperature"]; ok {
		t.Error("temperature is sent")
	}
	msgs := body["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("%d messages", len(msgs))
	}
	system, user := grading.BuildPrompt(req)
	sys := msgs[0].(map[string]any)
	if sys["role"] != "system" || sys["content"] != system {
		t.Errorf("system message: %v", sys)
	}
	usr := msgs[1].(map[string]any)
	parts := usr["content"].([]any)
	if usr["role"] != "user" || len(parts) != 1+len(req.Photos) {
		t.Fatalf("user message: %v", usr)
	}
	first := parts[0].(map[string]any)
	if first["type"] != "text" || first["text"] != user {
		t.Errorf("text part: %v", first)
	}
	for i, photo := range req.Photos {
		part := parts[1+i].(map[string]any)
		img := part["image_url"].(map[string]any)
		wantURL := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(photo)
		if part["type"] != "image_url" || img["url"] != wantURL || img["detail"] != "high" {
			t.Errorf("image part %d: %v", i, part)
		}
	}

	rf := body["response_format"].(map[string]any)
	js := rf["json_schema"].(map[string]any)
	if rf["type"] != "json_schema" || js["name"] != "grading" || js["strict"] != true {
		t.Errorf("response_format: %v", rf)
	}
	wantSchema, _ := json.Marshal(grading.ReplySchema())
	gotSchema, _ := json.Marshal(js["schema"])
	if string(wantSchema) != string(gotSchema) {
		t.Errorf("schema differs:\n%s\n%s", gotSchema, wantSchema)
	}
}

func TestNoKeyNoAuthorization(t *testing.T) {
	srv, rec := newServer(t, okHandler("{}"))
	cfg := testConfig(srv.URL)
	cfg.APIKey = ""
	if _, _, err := grade(t, newProvider(t, cfg)); err != nil {
		t.Fatal(err)
	}
	if _, ok := rec.all()[0].Header["Authorization"]; ok {
		t.Error("Authorization is sent without a key")
	}
}

func TestStructuredOutputModes(t *testing.T) {
	tests := []struct {
		mode grading.StructuredOutput
		want any // the response_format member, nil when absent
	}{
		{"", "json_schema"},
		{grading.StructuredJSONSchema, "json_schema"},
		{grading.StructuredJSONObject, map[string]any{"type": "json_object"}},
		{grading.StructuredNone, nil},
	}
	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			srv, rec := newServer(t, okHandler("{}"))
			cfg := testConfig(srv.URL)
			cfg.StructuredOutput = tt.mode
			_, meta, err := grade(t, newProvider(t, cfg))
			if err != nil {
				t.Fatal(err)
			}
			body := rec.body(t, 0)
			rf, present := body["response_format"]
			switch want := tt.want.(type) {
			case nil:
				if present {
					t.Errorf("response_format = %v, want none", rf)
				}
			case string:
				if rf.(map[string]any)["type"] != want {
					t.Errorf("response_format = %v", rf)
				}
			default:
				if !reflect.DeepEqual(rf, want) {
					t.Errorf("response_format = %v, want %v", rf, want)
				}
			}
			wantMode := tt.mode
			if wantMode == "" {
				wantMode = grading.StructuredJSONSchema
			}
			if meta.StructuredOutput != wantMode || meta.Downgraded != "" {
				t.Errorf("meta = %+v", meta)
			}
		})
	}
}

func TestRetryHintMessages(t *testing.T) {
	srv, rec := newServer(t, okHandler("{}"))
	p := newProvider(t, testConfig(srv.URL))
	hint := &grading.RetryHint{PreviousAnswer: "not json", Problem: "bad"}
	if _, _, err := p.Grade(context.Background(), testRequest(), hint); err != nil {
		t.Fatal(err)
	}
	msgs := rec.body(t, 0)["messages"].([]any)
	if len(msgs) != 4 {
		t.Fatalf("%d messages", len(msgs))
	}
	a, u := msgs[2].(map[string]any), msgs[3].(map[string]any)
	if a["role"] != "assistant" || a["content"] != "not json" {
		t.Errorf("assistant message: %v", a)
	}
	if u["role"] != "user" || u["content"] != hint.Message() {
		t.Errorf("user message: %v", u)
	}
}

func TestResponseParsing(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantText   string
		wantModel  string
		wantFinish string
	}{
		{"string content", `{"model":"srv-model","choices":[{"message":{"content":"{\"a\":1}"},"finish_reason":"stop"}]}`, `{"a":1}`, "srv-model", "stop"},
		{"no model in reply", `{"choices":[{"message":{"content":"x"},"finish_reason":"stop"}]}`, "x", "vision-1", "stop"},
		{"length", `{"choices":[{"message":{"content":"{\"a\""},"finish_reason":"length"}]}`, `{"a"`, "vision-1", "length"},
		{"no finish reason", `{"choices":[{"message":{"content":"x"}}]}`, "x", "vision-1", ""},
		{"array of parts", `{"choices":[{"message":{"content":[{"type":"text","text":"{\"a\""},{"type":"image","text":"skip"},{"type":"text","text":":1}"}]},"finish_reason":"stop"}]}`, `{"a":1}`, "vision-1", "stop"},
		{"output_text parts", `{"choices":[{"message":{"content":[{"type":"output_text","text":"hi"}]}}]}`, "hi", "vision-1", ""},
		{"null content", `{"choices":[{"message":{"content":null},"finish_reason":"content_filter"}]}`, "", "vision-1", "content_filter"},
		{"missing content", `{"choices":[{"message":{},"finish_reason":"stop"}]}`, "", "vision-1", "stop"},
		{"first choice only", `{"choices":[{"message":{"content":"one"}},{"message":{"content":"two"}}]}`, "one", "vision-1", ""},
		{"unknown fields", `{"id":"x","usage":{"total_tokens":3},"choices":[{"index":0,"message":{"role":"assistant","content":"x","refusal":null},"finish_reason":"stop"}]}`, "x", "vision-1", "stop"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
				_, _ = w.Write([]byte(tt.body))
			})
			text, meta, err := grade(t, newProvider(t, testConfig(srv.URL)))
			if err != nil {
				t.Fatal(err)
			}
			if text != tt.wantText || meta.Model != tt.wantModel || meta.FinishReason != tt.wantFinish {
				t.Errorf("got %q %+v", text, meta)
			}
		})
	}
}

func TestResponseFailures(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantRoute string
	}{
		{"not JSON", 200, "<html>hello</html>", "Provider.Grade->Provider.send.OpenAIBodyInvalid"},
		{"empty body", 200, "", "Provider.Grade->Provider.send.OpenAIBodyInvalid"},
		{"no choices", 200, `{"choices":[]}`, "Provider.Grade->Provider.send.OpenAINoChoices"},
		{"choices missing", 200, `{"error":"none"}`, "Provider.Grade->Provider.send.OpenAINoChoices"},
		{"content is a number", 200, `{"choices":[{"message":{"content":5}}]}`, "Provider.Grade->Provider.send.OpenAIContentInvalid"},
		{"content is an object", 200, `{"choices":[{"message":{"content":{"a":1}}}]}`, "Provider.Grade->Provider.send.OpenAIContentInvalid"},
		{"content part of wrong shape", 200, `{"choices":[{"message":{"content":[5]}}]}`, "Provider.Grade->Provider.send.OpenAIContentInvalid"},
		{"too large", 200, strings.Repeat("x", MaxResponseBytes+10), "Provider.Grade->Provider.send.OpenAIBodyTooLarge"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})
			_, _, err := grade(t, newProvider(t, testConfig(srv.URL)))
			meta := wantRoute(t, err, tt.wantRoute)
			if body, ok := meta["body"].(string); ok && len(body) > maxSnippet+3 {
				t.Errorf("body snippet has %d bytes", len(body))
			}
		})
	}
}

func TestExactlyMaxBodyIsRead(t *testing.T) {
	// A valid answer padded with spaces to exactly the limit is accepted.
	prefix := `{"choices":[{"message":{"content":"ok"}}]}`
	body := prefix + strings.Repeat(" ", MaxResponseBytes-len(prefix))
	srv, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) { _, _ = w.Write([]byte(body)) })
	text, _, err := grade(t, newProvider(t, testConfig(srv.URL)))
	if err != nil || text != "ok" {
		t.Errorf("%q, %v", text, err)
	}
}

func TestStatusErrors(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		header    map[string]string
		body      string
		wantRoute string
		wantMeta  map[string]any
	}{
		{"401", 401, nil, `{"error":{"message":"bad key"}}`, "send.OpenAIAuth", map[string]any{"status": 401}},
		{"403", 403, nil, "forbidden", "send.OpenAIAuth", map[string]any{"status": 403}},
		{"404", 404, nil, "no such model", "send.OpenAINotFound", map[string]any{"status": 404, "model": "vision-1"}},
		{"429 with seconds", 429, map[string]string{"Retry-After": "17"}, "slow down", "send.OpenAIRateLimited", map[string]any{"status": 429, "retry_after": "17", "retry_after_seconds": 17}},
		{"429 without header", 429, nil, "slow down", "send.OpenAIRateLimited", map[string]any{"retry_after": "", "retry_after_seconds": -1}},
		{"500", 500, nil, "oops", "send.OpenAIServer", map[string]any{"status": 500}},
		{"503", 503, nil, "busy", "send.OpenAIServer", map[string]any{"status": 503}},
		{"400 about something else", 400, nil, `{"error":"max_tokens too large"}`, "send.OpenAIStatus", map[string]any{"status": 400}},
		{"418", 418, nil, "teapot", "send.OpenAIStatus", map[string]any{"status": 418}},
		{"413", 413, nil, "too big", "send.OpenAIStatus", map[string]any{"status": 413}},
		{"304", 304, nil, "", "send.OpenAIStatus", map[string]any{"status": 304}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, rec := newServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
				for k, v := range tt.header {
					w.Header().Set(k, v)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})
			_, _, err := grade(t, newProvider(t, testConfig(srv.URL)))
			meta := wantRoute(t, err, "Provider.Grade->Provider."+tt.wantRoute)
			for k, want := range tt.wantMeta {
				if meta[k] != want {
					t.Errorf("metadata[%s] = %v, want %v", k, meta[k], want)
				}
			}
			if meta["provider"] != "Test AI" {
				t.Errorf("provider = %v", meta["provider"])
			}
			if meta["body"] != tt.body {
				t.Errorf("body = %v", meta["body"])
			}
			if n := len(rec.all()); n != 1 {
				t.Errorf("%d requests, want 1 (no retry)", n)
			}
		})
	}
}

func TestSnippetTruncatesAtRuneBoundary(t *testing.T) {
	body := strings.Repeat("é", 400) // 800 bytes
	srv, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(body))
	})
	_, _, err := grade(t, newProvider(t, testConfig(srv.URL)))
	meta := wantRoute(t, err, "send.OpenAIServer")
	snip := meta["body"].(string)
	if !strings.HasSuffix(snip, "...") || len(snip) > maxSnippet+3 {
		t.Fatalf("snippet has %d bytes, suffix %q", len(snip), snip[len(snip)-3:])
	}
	trimmed := strings.TrimSuffix(snip, "...")
	if strings.ToValidUTF8(trimmed, "?") != trimmed || len(trimmed) < maxSnippet-3 {
		t.Errorf("snippet is cut inside a rune or too short (%d bytes)", len(trimmed))
	}
}

func TestRetryAfterSeconds(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		header string
		want   int
	}{
		{"", -1},
		{"0", 0},
		{" 30 ", 30},
		{"-5", -1},
		{"soon", -1},
		{now.Add(90 * time.Second).UTC().Format(http.TimeFormat), 90},
		{now.Add(-time.Hour).UTC().Format(http.TimeFormat), 0},
	}
	for _, tt := range tests {
		if got := retryAfterSeconds(tt.header, now); got != tt.want {
			t.Errorf("retryAfterSeconds(%q) = %d, want %d", tt.header, got, tt.want)
		}
	}
}

func TestDowngrade(t *testing.T) {
	formatErr := `{"error":{"message":"Invalid parameter: response_format of type json_schema is not supported"}}`
	tests := []struct {
		name         string
		start        grading.StructuredOutput
		rejectBodies []string // answer to request i (400 with this body); later requests succeed
		wantRequests int
		wantFormats  []string // response_format type of each request ("" = absent)
		wantRoute    string   // failure, "" for success
		wantEffect   grading.StructuredOutput
		wantDown     grading.StructuredOutput
	}{
		{"schema to object", grading.StructuredJSONSchema, []string{formatErr}, 2, []string{"json_schema", "json_object"}, "", grading.StructuredJSONObject, grading.StructuredJSONObject},
		{"mentions only response_format", grading.StructuredJSONSchema, []string{"unsupported response_format"}, 2, []string{"json_schema", "json_object"}, "", grading.StructuredJSONObject, grading.StructuredJSONObject},
		{"mentions json_object", grading.StructuredJSONObject, []string{"'json_object' is not supported"}, 2, []string{"json_object", ""}, "", grading.StructuredNone, grading.StructuredNone},
		{"upper case body", grading.StructuredJSONSchema, []string{"RESPONSE_FORMAT unsupported"}, 2, []string{"json_schema", "json_object"}, "", grading.StructuredJSONObject, grading.StructuredJSONObject},
		{"only one downgrade per call", grading.StructuredJSONSchema, []string{formatErr, formatErr}, 2, []string{"json_schema", "json_object"}, "send.OpenAIStatus", grading.StructuredJSONObject, ""},
		{"400 about another thing", grading.StructuredJSONSchema, []string{"max_tokens is too large"}, 1, []string{"json_schema"}, "send.OpenAIStatus", grading.StructuredJSONSchema, ""},
		{"nothing weaker than none", grading.StructuredNone, []string{formatErr}, 1, []string{""}, "send.OpenAIStatus", grading.StructuredNone, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, rec := newServer(t, func(w http.ResponseWriter, _ *http.Request, n int) {
				if n <= len(tt.rejectBodies) {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(tt.rejectBodies[n-1]))
					return
				}
				replyJSON(w, "", "{}", "stop")
			})
			var logs lockedBuffer
			cfg := testConfig(srv.URL)
			cfg.StructuredOutput = tt.start
			p, err := New(cfg, slog.New(slog.NewTextHandler(&logs, nil)))
			if err != nil {
				t.Fatal(err)
			}
			_, meta, err := grade(t, p)
			if tt.wantRoute != "" {
				wantRoute(t, err, tt.wantRoute)
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if meta.Downgraded != tt.wantDown || meta.StructuredOutput != tt.wantEffect {
					t.Errorf("meta = %+v", meta)
				}
				if !strings.Contains(logs.String(), "weaker structured_output") || !strings.Contains(logs.String(), string(tt.wantEffect)) {
					t.Errorf("the downgrade is not logged: %s", logs.String())
				}
			}
			if got := rec.all(); len(got) != tt.wantRequests {
				t.Fatalf("%d requests, want %d", len(got), tt.wantRequests)
			}
			for i, want := range tt.wantFormats {
				rf, present := rec.body(t, i)["response_format"]
				got := ""
				if present {
					got = rf.(map[string]any)["type"].(string)
				}
				if got != want {
					t.Errorf("request %d: response_format %q, want %q", i, got, want)
				}
			}
			if p.StructuredOutput() != tt.wantEffect {
				t.Errorf("effective mode = %q, want %q", p.StructuredOutput(), tt.wantEffect)
			}
			if strings.Contains(logs.String(), testKey) {
				t.Error("the key is in the log")
			}
		})
	}
}

func TestDowngradeIsRemembered(t *testing.T) {
	srv, rec := newServer(t, func(w http.ResponseWriter, r *http.Request, n int) {
		if n == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("response_format not supported"))
			return
		}
		replyJSON(w, "", "{}", "stop")
	})
	p := newProvider(t, testConfig(srv.URL))
	if _, _, err := grade(t, p); err != nil {
		t.Fatal(err)
	}
	// The second call (for example the Grader's retry) goes straight to the
	// weaker mode and still reports the downgrade.
	_, meta, err := grade(t, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.all()) != 3 {
		t.Errorf("%d requests, want 3", len(rec.all()))
	}
	if rf := rec.body(t, 2)["response_format"].(map[string]any); rf["type"] != "json_object" {
		t.Errorf("third request: %v", rf)
	}
	if meta.Downgraded != grading.StructuredJSONObject || meta.StructuredOutput != grading.StructuredJSONObject {
		t.Errorf("meta = %+v", meta)
	}
}

func TestNetworkFailure(t *testing.T) {
	// A listener that is closed: connection refused.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	url := "http://" + l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	_, _, err = grade(t, newProvider(t, testConfig(url)))
	meta := wantRoute(t, err, "Provider.Grade->Provider.send.OpenAINetwork")
	if meta["url"] != url+"/chat/completions" {
		t.Errorf("url = %v", meta["url"])
	}

	// A server that cuts the connection.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, _, herr := w.(http.Hijacker).Hijack()
		if herr == nil {
			_ = conn.Close()
		}
	}))
	t.Cleanup(srv.Close)
	_, _, err = grade(t, newProvider(t, testConfig(srv.URL)))
	wantRoute(t, err, "Provider.Grade->Provider.send.OpenAINetwork")

	// A server that cuts the connection in the middle of the body.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte(`{"choices"`))
		w.(http.Flusher).Flush()
		conn, _, herr := w.(http.Hijacker).Hijack()
		if herr == nil {
			_ = conn.Close()
		}
	}))
	t.Cleanup(srv2.Close)
	_, _, err = grade(t, newProvider(t, testConfig(srv2.URL)))
	wantRoute(t, err, "Provider.Grade->Provider.send.OpenAINetwork")
}

func TestTimeout(t *testing.T) {
	release := make(chan struct{})
	srv, _ := newServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	t.Cleanup(func() { close(release) })
	cfg := testConfig(srv.URL)
	cfg.Timeout = 50 * time.Millisecond
	started := time.Now()
	_, _, err := grade(t, newProvider(t, cfg))
	meta := wantRoute(t, err, "Provider.Grade->Provider.send.OpenAITimeout")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("the cause is not context.DeadlineExceeded")
	}
	if meta["timeout"] != "50ms" {
		t.Errorf("timeout metadata = %v", meta["timeout"])
	}
	if time.Since(started) > 3*time.Second {
		t.Errorf("took %v", time.Since(started))
	}
}

func TestTimeoutWhileReadingBody(t *testing.T) {
	release := make(chan struct{})
	srv, _ := newServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte(`{"choices"`))
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	t.Cleanup(func() { close(release) })
	cfg := testConfig(srv.URL)
	cfg.Timeout = 100 * time.Millisecond
	_, _, err := grade(t, newProvider(t, cfg))
	wantRoute(t, err, "Provider.Grade->Provider.send.OpenAITimeout")
}

func TestParentDeadline(t *testing.T) {
	release := make(chan struct{})
	srv, _ := newServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	t.Cleanup(func() { close(release) })
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, _, err := newProvider(t, testConfig(srv.URL)).Grade(ctx, testRequest(), nil)
	wantRoute(t, err, "Provider.Grade->Provider.send.OpenAITimeout")
}

func TestCancellation(t *testing.T) {
	t.Run("during the request", func(t *testing.T) {
		started := make(chan struct{})
		release := make(chan struct{})
		srv, _ := newServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
			close(started)
			select {
			case <-release:
			case <-r.Context().Done():
			}
		})
		t.Cleanup(func() { close(release) })
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			<-started
			cancel()
		}()
		_, _, err := newProvider(t, testConfig(srv.URL)).Grade(ctx, testRequest(), nil)
		wantRoute(t, err, "Provider.Grade->Provider.send.OpenAICanceled")
		if !errors.Is(err, context.Canceled) {
			t.Error("the cause is not context.Canceled")
		}
	})
	t.Run("before the request", func(t *testing.T) {
		srv, rec := newServer(t, okHandler("{}"))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, _, err := newProvider(t, testConfig(srv.URL)).Grade(ctx, testRequest(), nil)
		wantRoute(t, err, "Provider.Grade->Provider.send.OpenAICanceled")
		if len(rec.all()) != 0 {
			t.Error("a request was sent after cancellation")
		}
	})
}

func TestRequestBuildFailures(t *testing.T) {
	srv, _ := newServer(t, okHandler("{}"))
	t.Run("encode", func(t *testing.T) {
		p := newProvider(t, testConfig(srv.URL))
		p.replySchema = func() map[string]any { return map[string]any{"bad": make(chan int)} }
		_, _, err := grade(t, p)
		wantRoute(t, err, "Provider.Grade->Provider.send.OpenAIRequestEncode")
	})
	t.Run("build", func(t *testing.T) {
		p := newProvider(t, testConfig(srv.URL))
		p.endpoint = "http://[::1"
		_, _, err := grade(t, p)
		wantRoute(t, err, "Provider.Grade->Provider.send.OpenAIRequestBuild")
	})
}

func TestKeyNeverLeaks(t *testing.T) {
	// Servers that echo the key back in their error bodies (some do, masked or
	// not), a failing connection, and a downgrade, with a logger attached.
	echo := func(status int) func(http.ResponseWriter, *http.Request, int) {
		return func(w http.ResponseWriter, r *http.Request, _ int) {
			w.WriteHeader(status)
			_, _ = fmt.Fprintf(w, `{"error":"Incorrect API key provided: %s (header %q)"}`, testKey, r.Header.Get("Authorization"))
		}
	}
	var logs lockedBuffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	check := func(t *testing.T, err error) {
		t.Helper()
		if err == nil {
			t.Fatal("expected an error")
		}
		appErr, ok := errors.FromError(err)
		if !ok {
			t.Fatalf("%T", err)
		}
		log.Error("grading failed", "err", err)
		for name, text := range map[string]string{
			"Error()":  err.Error(),
			"%+v":      fmt.Sprintf("%+v", err),
			"metadata": fmt.Sprint(appErr.Metadata),
			"json":     fmt.Sprint(appErr.ToJson()),
			"log":      logs.String(),
		} {
			if strings.Contains(text, testKey) {
				t.Errorf("the key is in %s: %s", name, text)
			}
		}
	}
	for _, status := range []int{400, 401, 403, 404, 418, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			srv, _ := newServer(t, echo(status))
			p, err := New(testConfig(srv.URL), log)
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = grade(t, p)
			check(t, err)
			if status == 401 {
				meta := wantRoute(t, err, "send.OpenAIAuth")
				if !strings.Contains(meta["body"].(string), "***") {
					t.Errorf("the key was not replaced in the snippet: %v", meta["body"])
				}
			}
		})
	}
	t.Run("downgrade echo", func(t *testing.T) {
		srv, _ := newServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "response_format rejected for %s", testKey)
		})
		p, err := New(testConfig(srv.URL), log)
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = grade(t, p)
		check(t, err)
	})
	t.Run("bad body", func(t *testing.T) {
		srv, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			_, _ = fmt.Fprintf(w, "not json %s", testKey)
		})
		p, err := New(testConfig(srv.URL), log)
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = grade(t, p)
		check(t, err)
	})
	t.Run("a key that appears in a transport error", func(t *testing.T) {
		p := newProvider(t, testConfig("http://127.0.0.1:1"))
		// A hostile transport error that holds the key.
		leaky := &redactedError{msg: "dial failed using " + testKey}
		redacted := p.redactErr(leaky)
		if strings.Contains(redacted.Error(), testKey) || !strings.Contains(redacted.Error(), "***") {
			t.Errorf("redactErr = %q", redacted)
		}
		if got := p.redactErr(&redactedError{msg: "plain"}); got.Error() != "plain" {
			t.Errorf("redactErr changed an error without the key: %v", got)
		}
		if _, _, err := grade(t, p); err == nil {
			t.Fatal("expected a network error")
		} else {
			check(t, err)
		}
	})
}

func TestConcurrentDowngrade(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if rf, _ := body["response_format"].(map[string]any); rf != nil && rf["type"] == "json_schema" {
			w.WriteHeader(400)
			_, _ = w.Write([]byte("response_format"))
			return
		}
		replyJSON(w, "", "{}", "stop")
	}))
	t.Cleanup(srv.Close)
	p := newProvider(t, testConfig(srv.URL))
	done := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			_, _, err := grade(t, p)
			done <- err
		}()
	}
	for i := 0; i < 8; i++ {
		if err := <-done; err != nil {
			t.Error(err)
		}
	}
	if p.StructuredOutput() != grading.StructuredJSONObject {
		t.Errorf("mode = %q", p.StructuredOutput())
	}
}

// TestWithGrader runs the Provider under the Grader, as the submission service
// does: a good reply, a truncated one that is asked again, and a reply wrapped
// in a code fence from a json_object server that the Grader parses leniently.
func TestWithGrader(t *testing.T) {
	good := fake.JSONReply("page",
		grading.ExerciseResult{ExerciseID: "a", Verdict: grading.VerdictCorrect, Transcription: "t", FeedbackForChild: "f", NotesForParent: "n"},
		grading.ExerciseResult{ExerciseID: "b", Verdict: grading.VerdictIncorrect, Transcription: "t", FeedbackForChild: "f", NotesForParent: "n"})

	t.Run("success", func(t *testing.T) {
		srv, _ := newServer(t, okHandler(good))
		g := &grading.Grader{Provider: newProvider(t, testConfig(srv.URL))}
		got, err := g.Grade(context.Background(), testRequest())
		if err != nil {
			t.Fatal(err)
		}
		if got.Meta.Provider != "Test AI" || got.Meta.Model != "vision-1" || got.Meta.Retried || len(got.Exercises) != 2 {
			t.Errorf("result = %+v", got)
		}
	})
	t.Run("length is invalid and asked again with the answer", func(t *testing.T) {
		srv, rec := newServer(t, func(w http.ResponseWriter, _ *http.Request, n int) {
			if n == 1 {
				replyJSON(w, "", good[:20], "length")
				return
			}
			replyJSON(w, "", good, "stop")
		})
		g := &grading.Grader{Provider: newProvider(t, testConfig(srv.URL))}
		got, err := g.Grade(context.Background(), testRequest())
		if err != nil {
			t.Fatal(err)
		}
		if !got.Meta.Retried {
			t.Error("not retried")
		}
		msgs := rec.body(t, 1)["messages"].([]any)
		if len(msgs) != 4 || msgs[2].(map[string]any)["content"] != good[:20] {
			t.Errorf("retry messages: %v", msgs)
		}
	})
	t.Run("fenced reply after a downgrade is parsed leniently", func(t *testing.T) {
		srv, _ := newServer(t, func(w http.ResponseWriter, r *http.Request, n int) {
			if n == 1 {
				w.WriteHeader(400)
				_, _ = w.Write([]byte("json_schema is not supported"))
				return
			}
			replyJSON(w, "", "```json\n"+good+"\n```", "stop")
		})
		g := &grading.Grader{Provider: newProvider(t, testConfig(srv.URL))}
		got, err := g.Grade(context.Background(), testRequest())
		if err != nil {
			t.Fatal(err)
		}
		if got.Meta.Retried || got.Meta.Downgraded != grading.StructuredJSONObject {
			t.Errorf("meta = %+v", got.Meta)
		}
	})
	t.Run("provider errors are not retried", func(t *testing.T) {
		srv, rec := newServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) { w.WriteHeader(401) })
		g := &grading.Grader{Provider: newProvider(t, testConfig(srv.URL))}
		_, err := g.Grade(context.Background(), testRequest())
		wantRoute(t, err, "Provider.Grade->Provider.send.OpenAIAuth")
		if len(rec.all()) != 1 {
			t.Errorf("%d requests", len(rec.all()))
		}
	})
}

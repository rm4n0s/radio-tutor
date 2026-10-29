// Package openai is the grading.Provider for OpenAI-compatible servers (SPEC
// §11.1 and §11.2): OpenAI, Ollama, LM Studio, llama.cpp, OpenRouter and every
// other server that speaks POST <base_url>/chat/completions. It uses net/http
// and encoding/json only (SPEC D8).
//
// A Provider is built from a Config. It sends the prompt of grading.BuildPrompt
// and the photos as data URLs, asks for a JSON reply in the configured
// structured output mode and returns the text of choices[0].message.content
// and its finish reason. When the server rejects the response format with
// status 400, the Provider retries once with the next weaker mode and reports
// the downgrade in grading.Meta (Downgraded), so the UI can tell the parent to
// change the setting.
//
// Setting grading.Grader.Lenient: the Grader parses leniently by itself when
// Meta.StructuredOutput is json_object or none, which covers a downgrade in the
// middle of a call. The submission service (4.3) and the provider test (3.3)
// should still set Lenient from the configured mode,
// cfg.StructuredOutput.Lenient(), or from Provider.StructuredOutput() (the
// effective mode, which a downgrade changes for the rest of the Provider's
// life).
//
// The API key is only ever put in the Authorization header. It is never part of
// an error, a log record or a metadata value; body snippets in errors have it
// replaced by "***".
package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/grading"
)

// Limits and defaults of the provider.
const (
	// DefaultTimeout is used when Config.Timeout is zero (SPEC §11.1).
	DefaultTimeout = 180 * time.Second
	// MaxTokens is the max_tokens of every request (SPEC §11.2).
	MaxTokens = 8000
	// MaxResponseBytes is the most bytes read from a response body.
	MaxResponseBytes = 4 << 20
	// maxSnippet is the longest piece of a response body put in an error.
	maxSnippet = 500
)

// Config describes one provider (SPEC §11.1).
type Config struct {
	// Name is shown to the parent and returned by Provider.Name.
	Name string
	// BaseURL is the server's base URL such as https://api.openai.com/v1; the
	// request goes to BaseURL + "/chat/completions". A trailing slash is
	// ignored. It must be http or https and must not hold a user name or
	// password.
	BaseURL string
	// Model is the model name sent with every request.
	Model string
	// APIKey is sent as a Bearer token when it is not empty (local servers
	// need none).
	APIKey string
	// Timeout bounds one Grade call, including the downgrade retry. Zero
	// means DefaultTimeout.
	Timeout time.Duration
	// StructuredOutput is the mode asked for; empty means json_schema.
	StructuredOutput grading.StructuredOutput
	// Version is the app version for the User-Agent "radio-tutor/<version>";
	// empty means "dev".
	Version string
}

// Provider implements grading.Provider over Chat Completions. It is safe for
// concurrent use.
type Provider struct {
	cfg      Config
	endpoint string
	client   *http.Client
	log      *slog.Logger
	// replySchema returns the schema sent in json_schema mode; tests replace it.
	replySchema func() map[string]any

	mu   sync.Mutex
	mode grading.StructuredOutput // effective mode, weaker than cfg's after a downgrade
}

var _ grading.Provider = (*Provider)(nil)

// New checks cfg and builds the Provider. log receives the downgrade notice;
// nil discards it.
func New(cfg Config, log *slog.Logger) (*Provider, error) {
	if cfg.StructuredOutput == "" {
		cfg.StructuredOutput = grading.StructuredJSONSchema
	}
	if !cfg.StructuredOutput.Valid() {
		return nil, errors.New("OpenAIConfigStructuredOutput", "structured_output must be json_schema, json_object or none",
			"structured_output", string(cfg.StructuredOutput))
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, errors.New("OpenAIConfigModel", "the provider needs a model name", "provider", cfg.Name)
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	u, err := url.Parse(base)
	if err != nil {
		return nil, errors.New("OpenAIConfigBaseURLParse", "the base URL is not a valid URL", "provider", cfg.Name)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return nil, errors.New("OpenAIConfigBaseURL", "the base URL must be http or https with a host and no user name or password",
			"provider", cfg.Name, "base_url", u.Redacted())
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.Version == "" {
		cfg.Version = "dev"
	}
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	cfg.BaseURL = base
	return &Provider{
		cfg:         cfg,
		endpoint:    base + "/chat/completions",
		client:      &http.Client{},
		log:         log,
		replySchema: grading.ReplySchema,
		mode:        cfg.StructuredOutput,
	}, nil
}

// Name returns the provider's name for the parent.
func (p *Provider) Name() string { return p.cfg.Name }

// StructuredOutput returns the mode the next request will use: the configured
// one, or a weaker one after a downgrade. Compare it with the configured mode
// to know whether the parent should change the setting.
func (p *Provider) StructuredOutput() grading.StructuredOutput {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mode
}

// answer is what one successful exchange produced.
type answer struct {
	text, model, finish string
}

// Grade sends the request and returns the model's text. See grading.Provider.
// On a status 400 about the response format it retries once with the next
// weaker mode; the downgrade stays for the Provider's later calls and shows in
// Meta.Downgraded. A call longer than the configured timeout fails with
// OpenAITimeout, a cancelled ctx with OpenAICanceled.
func (p *Provider) Grade(ctx context.Context, req grading.Request, prior *grading.RetryHint) (string, grading.Meta, error) {
	ctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()

	mode := p.StructuredOutput()
	ans, weaker, err := p.send(ctx, req, prior, mode, true)
	if err != nil {
		return "", grading.Meta{}, err
	}
	if weaker != "" {
		p.log.Warn("the provider rejected the response format; using a weaker structured_output",
			"provider", p.cfg.Name, "from", string(mode), "to", string(weaker))
		p.mu.Lock()
		p.mode = weaker
		p.mu.Unlock()
		mode = weaker
		ans, _, err = p.send(ctx, req, prior, mode, false)
		if err != nil {
			return "", grading.Meta{}, err
		}
	}
	meta := grading.Meta{Model: ans.model, FinishReason: ans.finish, StructuredOutput: mode}
	if mode != p.cfg.StructuredOutput {
		meta.Downgraded = mode
	}
	return ans.text, meta, nil
}

// weakerMode returns the next weaker mode, or "" when there is none.
func weakerMode(m grading.StructuredOutput) grading.StructuredOutput {
	switch m {
	case grading.StructuredJSONSchema:
		return grading.StructuredJSONObject
	case grading.StructuredJSONObject:
		return grading.StructuredNone
	}
	return ""
}

// rejectsFormat reports whether a 400 body complains about the response
// format.
func rejectsFormat(body []byte) bool {
	s := strings.ToLower(string(body))
	return strings.Contains(s, "response_format") || strings.Contains(s, "json_schema") || strings.Contains(s, "json_object")
}

// send makes one HTTP exchange. When downgrade is allowed and the server
// rejected the response format, it returns the weaker mode to try (and no
// error).
func (p *Provider) send(ctx context.Context, req grading.Request, prior *grading.RetryHint, mode grading.StructuredOutput, downgrade bool) (answer, grading.StructuredOutput, error) {
	payload, err := json.Marshal(p.buildBody(req, prior, mode))
	if err != nil {
		return answer{}, "", errors.NewErr("OpenAIRequestEncode", err).SetMetadata("provider", p.cfg.Name)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(payload))
	if err != nil {
		return answer{}, "", errors.NewErr("OpenAIRequestBuild", p.redactErr(err)).SetMetadata("provider", p.cfg.Name)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "radio-tutor/"+p.cfg.Version)
	if p.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}

	resp, err := p.client.Do(httpReq)
	var body []byte
	if err == nil {
		defer func() { _ = resp.Body.Close() }() // a close failure after the body was read changes nothing
		body, err = io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	}
	if err != nil {
		cerr := ctx.Err()
		switch {
		case errors.Is(cerr, context.DeadlineExceeded):
			return answer{}, "", errors.NewErr("OpenAITimeout", cerr).SetMetadata(
				"provider", p.cfg.Name, "timeout", p.cfg.Timeout.String())
		case cerr != nil:
			return answer{}, "", errors.NewErr("OpenAICanceled", cerr).SetMetadata("provider", p.cfg.Name)
		}
		return answer{}, "", errors.NewErr("OpenAINetwork", p.redactErr(err)).SetMetadata(
			"provider", p.cfg.Name, "url", p.endpoint)
	}
	if len(body) > MaxResponseBytes {
		return answer{}, "", errors.New("OpenAIBodyTooLarge", "the server's answer is too large",
			"provider", p.cfg.Name, "limit", MaxResponseBytes)
	}

	status := resp.StatusCode
	switch {
	case status >= 200 && status < 300:
		// parsed below
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return answer{}, "", errors.New("OpenAIAuth", "the server refused the API key",
			"provider", p.cfg.Name, "status", status, "body", p.snippet(body))
	case status == http.StatusNotFound:
		return answer{}, "", errors.New("OpenAINotFound", "the server does not know this URL or model",
			"provider", p.cfg.Name, "status", status, "url", p.endpoint, "model", p.cfg.Model, "body", p.snippet(body))
	case status == http.StatusTooManyRequests:
		retryAfter := resp.Header.Get("Retry-After")
		return answer{}, "", errors.New("OpenAIRateLimited", "the server says there are too many requests",
			"provider", p.cfg.Name, "status", status, "retry_after", retryAfter,
			"retry_after_seconds", retryAfterSeconds(retryAfter, time.Now()), "body", p.snippet(body))
	case status >= 500:
		return answer{}, "", errors.New("OpenAIServer", "the server failed",
			"provider", p.cfg.Name, "status", status, "body", p.snippet(body))
	case status == http.StatusBadRequest && downgrade && weakerMode(mode) != "" && rejectsFormat(body):
		return answer{}, weakerMode(mode), nil
	default:
		return answer{}, "", errors.New("OpenAIStatus", "the server answered with an unexpected status",
			"provider", p.cfg.Name, "status", status, "body", p.snippet(body))
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return answer{}, "", errors.NewErr("OpenAIBodyInvalid", err).SetMetadata(
			"provider", p.cfg.Name, "body", p.snippet(body))
	}
	if len(parsed.Choices) == 0 {
		return answer{}, "", errors.New("OpenAINoChoices", "the answer has no choices",
			"provider", p.cfg.Name, "body", p.snippet(body))
	}
	choice := parsed.Choices[0]
	text, err := contentText(choice.Message.Content)
	if err != nil {
		return answer{}, "", errors.NewErr("OpenAIContentInvalid", err).SetMetadata(
			"provider", p.cfg.Name, "body", p.snippet(body))
	}
	model := parsed.Model
	if model == "" {
		model = p.cfg.Model
	}
	return answer{text: text, model: model, finish: choice.FinishReason}, "", nil
}

// chatResponse is the part of a Chat Completions answer that is read.
type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// contentText returns the text of a message content: a string, null (empty), or
// an array of parts of which the text parts are joined. Its error is a plain
// JSON error for the caller to tag.
func contentText(raw json.RawMessage) (string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", err
		}
		return s, nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, part := range parts {
		if part.Type == "text" || part.Type == "output_text" || part.Type == "" {
			sb.WriteString(part.Text)
		}
	}
	return sb.String(), nil
}

// retryAfterSeconds reads a Retry-After header (seconds or an HTTP date) and
// returns the seconds to wait, or -1 when it is absent or unreadable.
func retryAfterSeconds(h string, now time.Time) int {
	h = strings.TrimSpace(h)
	if h == "" {
		return -1
	}
	if n, err := strconv.Atoi(h); err == nil && n >= 0 {
		return n
	}
	if t, err := http.ParseTime(h); err == nil {
		if d := t.Sub(now); d > 0 {
			return int(d.Round(time.Second) / time.Second)
		}
		return 0
	}
	return -1
}

// snippet returns the start of a response body for an error: at most maxSnippet
// bytes, cut at a rune boundary, with the API key replaced by "***".
func (p *Provider) snippet(body []byte) string {
	s := string(body)
	if k := p.cfg.APIKey; k != "" {
		s = strings.ReplaceAll(s, k, "***")
	}
	if len(s) > maxSnippet {
		s = s[:maxSnippet]
		for len(s) > 0 && !utf8.ValidString(s) {
			s = s[:len(s)-1]
		}
		s += "..."
	}
	return s
}

// redactErr returns err, or an error with the same text but the API key
// replaced by "***" when the text holds the key.
func (p *Provider) redactErr(err error) error {
	k := p.cfg.APIKey
	if k == "" || !strings.Contains(err.Error(), k) {
		return err
	}
	return &redactedError{msg: strings.ReplaceAll(err.Error(), k, "***")}
}

// redactedError is a plain error whose text is already free of the API key.
type redactedError struct{ msg string }

func (r *redactedError) Error() string { return r.msg }

// buildBody makes the JSON body of one request (SPEC §11.2).
func (p *Provider) buildBody(req grading.Request, prior *grading.RetryHint, mode grading.StructuredOutput) chatRequest {
	system, user := grading.BuildPrompt(req)
	parts := make([]contentPart, 0, 1+len(req.Photos))
	parts = append(parts, contentPart{Type: "text", Text: user})
	for _, photo := range req.Photos {
		parts = append(parts, contentPart{
			Type:     "image_url",
			ImageURL: &imageURL{URL: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(photo), Detail: "high"},
		})
	}
	msgs := []chatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: parts},
	}
	if prior != nil {
		msgs = append(msgs,
			chatMessage{Role: "assistant", Content: prior.PreviousAnswer},
			chatMessage{Role: "user", Content: prior.Message()},
		)
	}
	body := chatRequest{Model: p.cfg.Model, Messages: msgs, MaxTokens: MaxTokens}
	switch mode {
	case grading.StructuredJSONSchema:
		body.ResponseFormat = &responseFormat{
			Type:       "json_schema",
			JSONSchema: &jsonSchemaFormat{Name: "grading", Strict: true, Schema: p.replySchema()},
		}
	case grading.StructuredJSONObject:
		body.ResponseFormat = &responseFormat{Type: "json_object"}
	}
	return body
}

// chatRequest is the JSON body of POST /chat/completions.
type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	MaxTokens      int             `json:"max_tokens"`
}

// chatMessage is one message; Content is a string or a []contentPart.
type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// contentPart is a text part or an image part of a user message.
type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

// imageURL is the image of an image part.
type imageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail"`
}

// responseFormat is the response_format member.
type responseFormat struct {
	Type       string            `json:"type"`
	JSONSchema *jsonSchemaFormat `json:"json_schema,omitempty"`
}

// jsonSchemaFormat is the json_schema member of a json_schema response format.
type jsonSchemaFormat struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

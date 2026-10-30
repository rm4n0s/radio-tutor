package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/glutter/ui"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/providers"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
	"github.com/rm4n0s/radio-tutor/internal/gui/testutil"
)

// addProvider adds a local provider to the store, and makes it active when
// asked.
func addProvider(t testing.TB, env *app.Env, name, apiKey string, active bool) {
	t.Helper()
	err := env.Providers.Add(context.Background(), providers.Config{
		Name: name, BaseURL: "http://localhost:11434/v1", Model: "qwen2.5vl:7b", APIKey: apiKey,
		Kind: providers.KindLocal, TimeoutSeconds: 10, StructuredOutput: grading.StructuredJSONSchema,
	})
	if err != nil {
		t.Fatal(err)
	}
	if active {
		if err := env.Providers.SetActive(context.Background(), name); err != nil {
			t.Fatal(err)
		}
	}
}

const goodReply = `{"page_notes":"","results":[{"exercise_id":"sum","verdict":"correct","transcription":"2 + 3 = 5","feedback_for_child":"Well done!","notes_for_parent":"The child wrote 5."}]}`

// chatServer is a test server that speaks Chat Completions; h answers the n-th
// request (counted from 1).
func chatServer(t *testing.T, h func(w http.ResponseWriter, n int)) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Errorf("reading the request: %v", err)
		}
		mu.Lock()
		n++
		k := n
		mu.Unlock()
		h(w, k)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// reply answers a Chat Completions request.
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

// addServerProvider adds a provider that talks to the test server.
func addServerProvider(t *testing.T, env *app.Env, name, url string, mode grading.StructuredOutput) {
	t.Helper()
	err := env.Providers.Add(context.Background(), providers.Config{
		Name: name, BaseURL: url + "/v1", Model: "vision-1", APIKey: "sk-test",
		Kind: providers.KindLocal, TimeoutSeconds: 10, StructuredOutput: mode,
	})
	if err != nil {
		t.Fatal(err)
	}
}

// scrollPresets scrolls the list of presets to its end, where Other server is.
func scrollPresets(a *testutil.App) {
	a.Scroll(key(KeyPresetPrefix+"openai"), ui.Offset{Dy: 3000})
	a.PumpAndSettle()
}

func openProviders(t *testing.T, a *testutil.App) {
	t.Helper()
	a.TapKey(KeyProviders)
	waitKey(t, a, KeyProviderAdd)
}

func TestProvidersEmptyList(t *testing.T) {
	a := pump(t, config{})
	openProviders(t, a)
	for _, text := range []string{"There are no providers yet. Add one so that the computer can check the exercises.", "Add a provider"} {
		if !has(a, text) {
			t.Errorf("the screen does not show %q", text)
		}
	}
	if !a.HasKey("providers-privacy") || !has(a, "A cloud provider receives the photos of the exercises, and the exercise text, over the internet. A local provider keeps them on this computer or your home network. The name of the child is never sent.") {
		t.Error("the privacy note is missing")
	}
}

func TestProvidersListShowsKindAndActive(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		addProvider(t, env, "Home AI", "", true)
		err := env.Providers.Add(context.Background(), providers.Config{
			Name: "Cloud AI", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o", APIKey: "sk-1", Kind: providers.KindCloud,
		})
		if err != nil {
			t.Fatal(err)
		}
	}})
	openProviders(t, a)
	for _, text := range []string{"Home AI", "Cloud AI", "Local", "Cloud", "Active", "gpt-4o  -  https://api.openai.com/v1"} {
		if !has(a, text) {
			t.Errorf("the list does not show %q", text)
		}
	}
	if a.HasKey(KeyProviderActivePrefix + "Home AI") {
		t.Error("the active provider has a Set active button")
	}
	if !a.HasKey(KeyProviderActivePrefix + "Cloud AI") {
		t.Error("the other provider has no Set active button")
	}
}

func TestProvidersWarnWhenNoneIsActive(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) { addProvider(t, env, "Home AI", "", false) }})
	openProviders(t, a)
	if !has(a, "No provider is active, so the exercises cannot be checked. Press Set active on one of them.") {
		t.Error("no warning that none is active")
	}
	a.TapKey(KeyProviderActivePrefix + "Home AI")
	tillTrue(t, a, "the provider to be active", func() bool { return a.Env.Providers.ActiveName() == "Home AI" })
	waitGone(t, a, KeyProviderActivePrefix+"Home AI")
	if has(a, "No provider is active, so the exercises cannot be checked. Press Set active on one of them.") {
		t.Error("the warning stays after a provider became active")
	}
}

func TestProvidersAddFromPresets(t *testing.T) {
	tests := []struct {
		id        string
		name      string
		url       string
		model     string
		kind      providers.Kind
		mode      grading.StructuredOutput
		cloudNote bool
	}{
		{"openai", "OpenAI", "https://api.openai.com/v1", "gpt-4o", providers.KindCloud, grading.StructuredJSONSchema, true},
		{"ollama", "Ollama", "http://localhost:11434/v1", "qwen2.5vl:7b", providers.KindLocal, grading.StructuredJSONSchema, false},
		{"anthropic", "Anthropic", "https://api.anthropic.com/v1/", "claude-opus-5-5", providers.KindCloud, grading.StructuredJSONObject, true},
		{"custom", "My server", "http://192.168.1.9:8000/v1", "llava", providers.KindLocal, grading.StructuredJSONSchema, false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			a := pump(t, config{})
			openProviders(t, a)
			a.TapKey(KeyProviderAdd)
			waitKey(t, a, KeyPresetPrefix+"openai")
			scrollPresets(a)
			for _, p := range providers.Presets() {
				if !a.HasKey(KeyPresetPrefix + p.ID) {
					t.Errorf("the preset %s is not offered", p.ID)
				}
			}
			if !a.HasKey(KeyPresetCustom) {
				t.Error("Other server is not offered")
			}
			if tt.id == "custom" {
				scrollPresets(a)
			}
			a.TapKey(KeyPresetPrefix + tt.id)
			waitKey(t, a, KeyProviderName)
			if tt.id == "custom" {
				a.EnterKey(KeyProviderName, tt.name)
				a.EnterKey(KeyProviderURL, tt.url)
				a.EnterKey(KeyProviderModel, tt.model)
			} else if has(a, tt.name) {
				// The fields hold the preset's values (the name is also the
				// text of the field, not a Text widget).
				t.Log("the name is shown as a title")
			}
			a.EnterKey(KeyProviderAPIKey, "sk-new")
			if got := a.HasKey("providers-privacy"); got != tt.cloudNote {
				t.Errorf("privacy note shown = %v, want %v", got, tt.cloudNote)
			}
			a.TapKey(app.KeySubmit)
			waitKey(t, a, KeyProviderAdd)
			waitText(t, a, tt.name)
			c, ok := a.Env.Providers.Get(tt.name)
			if !ok {
				t.Fatalf("the provider %s was not added", tt.name)
			}
			if c.BaseURL != tt.url || c.Model != tt.model || c.Kind != tt.kind || c.StructuredOutput != tt.mode || c.APIKey != "sk-new" || c.TimeoutSeconds != 180 {
				t.Errorf("provider = %+v", c)
			}
			if a.Env.Providers.ActiveName() != "" {
				t.Error("the first provider was made active automatically")
			}
		})
	}
}

func TestProvidersAddCancelFromThePresetList(t *testing.T) {
	a := pump(t, config{})
	openProviders(t, a)
	a.TapKey(KeyProviderAdd)
	waitKey(t, a, KeyDialogCancel)
	a.TapKey(KeyDialogCancel)
	waitGone(t, a, KeyDialogCancel)
	if len(a.Env.Providers.List()) != 0 {
		t.Error("a provider appeared")
	}
}

func TestProviderFormErrors(t *testing.T) {
	tests := []struct {
		name  string
		set   func(a *testutil.App)
		want  string
		setup func(t *testing.T, env *app.Env)
	}{
		{"no name", func(a *testutil.App) { a.EnterKey(KeyProviderName, "  ") }, "The name needs 1 to 60 characters on one line.", nil},
		{"duplicate name", func(a *testutil.App) { a.EnterKey(KeyProviderName, "Home AI") }, "A provider with this name already exists.",
			func(t *testing.T, env *app.Env) { addProvider(t, env, "Home AI", "", false) }},
		{"timeout is not a number", func(a *testutil.App) { a.EnterKey(KeyProviderTimeout, "soon") }, "The timeout must be a number from 1 to 3600.", nil},
		{"timeout too long", func(a *testutil.App) { a.EnterKey(KeyProviderTimeout, "99999") }, "The timeout must be a number from 1 to 3600.", nil},
		{"no model", func(a *testutil.App) { a.EnterKey(KeyProviderModel, "") }, "Write the name of the model.", nil},
		{"bad URL", func(a *testutil.App) { a.EnterKey(KeyProviderURL, "ftp://x") }, "The base URL must start with http:// or https://.", nil},
		{"URL that is not a URL", func(a *testutil.App) { a.EnterKey(KeyProviderURL, "http://[::1") }, "The base URL must start with http:// or https://.", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pump(t, config{setup: tt.setup})
			openProviders(t, a)
			a.TapKey(KeyProviderAdd)
			waitKey(t, a, KeyPresetPrefix+"ollama")
			a.TapKey(KeyPresetPrefix + "ollama")
			waitKey(t, a, KeyProviderName)
			tt.set(a)
			a.TapKey(app.KeySubmit)
			waitText(t, a, tt.want)
			if a.HasKey(KeyProviderAdd) {
				t.Error("the form closed after an error")
			}
		})
	}
}

func TestProviderFormChoices(t *testing.T) {
	a := pump(t, config{})
	openProviders(t, a)
	a.TapKey(KeyProviderAdd)
	waitKey(t, a, KeyPresetCustom)
	scrollPresets(a)
	a.TapKey(KeyPresetCustom)
	waitKey(t, a, KeyProviderName)
	a.EnterKey(KeyProviderName, "Cloudy")
	a.EnterKey(KeyProviderURL, "https://example.com/v1")
	a.EnterKey(KeyProviderModel, "m1")
	a.EnterKey(KeyProviderTimeout, "45")
	a.TapKey(KeyKindCloud)
	if !a.HasKey("providers-privacy") {
		t.Error("choosing Cloud does not show the privacy note")
	}
	a.TapKey(KeyModePrefix + "none")
	if !has(a, "No help from the server: Radio Tutor reads the answer as well as it can.") {
		t.Error("the mode is not explained")
	}
	a.TapKey(app.KeySubmit)
	waitKey(t, a, KeyProviderAdd)
	c, _ := a.Env.Providers.Get("Cloudy")
	if c.Kind != providers.KindCloud || c.StructuredOutput != grading.StructuredNone || c.TimeoutSeconds != 45 || c.APIKey != "" {
		t.Errorf("provider = %+v", c)
	}
}

func TestProviderEdit(t *testing.T) {
	tests := []struct {
		name    string
		rename  string
		key     string
		wantKey string
	}{
		{"blank key keeps the saved key", "", "", "sk-old"},
		{"a new key replaces it", "", "sk-new", "sk-new"},
		{"rename", "Renamed", "", "sk-old"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
				addProvider(t, env, "Home AI", "sk-old", true)
			}})
			openProviders(t, a)
			a.TapKey(KeyProviderEditPrefix + "Home AI")
			waitKey(t, a, KeyProviderName)
			if !has(a, "Edit Home AI") || !has(a, "Leave empty to keep the saved key.") {
				t.Error("the form does not say it edits Home AI and keeps the key")
			}
			name := "Home AI"
			if tt.rename != "" {
				name = tt.rename
				a.EnterKey(KeyProviderName, name)
			}
			a.EnterKey(KeyProviderModel, "other-model")
			if tt.key != "" {
				a.EnterKey(KeyProviderAPIKey, tt.key)
			}
			a.TapKey(app.KeySubmit)
			waitKey(t, a, KeyProviderAdd)
			c, ok := a.Env.Providers.Get(name)
			if !ok {
				t.Fatalf("no provider %s", name)
			}
			if c.APIKey != tt.wantKey || c.Model != "other-model" {
				t.Errorf("provider = %+v", c)
			}
			if a.Env.Providers.ActiveName() != name {
				t.Errorf("active = %q, want %q", a.Env.Providers.ActiveName(), name)
			}
			if len(a.Env.Providers.List()) != 1 {
				t.Errorf("%d providers", len(a.Env.Providers.List()))
			}
		})
	}
}

func TestProviderDelete(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		addProvider(t, env, "Home AI", "", true)
		addProvider(t, env, "Spare", "", false)
	}})
	openProviders(t, a)
	a.TapKey(KeyProviderDeletePrefix + "Spare")
	waitKey(t, a, KeyDialogOK)
	if !has(a, "Delete Spare?") {
		t.Error("the confirmation does not name the provider")
	}
	a.TapKey(KeyDialogCancel)
	waitGone(t, a, KeyDialogOK)
	if len(a.Env.Providers.List()) != 2 {
		t.Fatal("Cancel deleted a provider")
	}
	a.TapKey(KeyProviderDeletePrefix + "Home AI")
	waitKey(t, a, KeyDialogOK)
	a.TapKey(KeyDialogOK)
	waitGone(t, a, KeyProviderPrefix+"Home AI")
	if a.Env.Providers.ActiveName() != "" || len(a.Env.Providers.List()) != 1 {
		t.Errorf("active %q, %d providers", a.Env.Providers.ActiveName(), len(a.Env.Providers.List()))
	}
	waitText(t, a, "No provider is active, so the exercises cannot be checked. Press Set active on one of them.")
}

func TestProviderActionFailureIsShown(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) { addProvider(t, env, "Home AI", "", false) }})
	openProviders(t, a)
	// The provider disappears behind the screen's back.
	if err := a.Env.Providers.Delete(context.Background(), "Home AI"); err != nil {
		t.Fatal(err)
	}
	a.TapKey(KeyProviderActivePrefix + "Home AI")
	waitText(t, a, "Something went wrong: there is no provider with this name (ProviderNotFound)")
}

func TestProvidersFileCannotBeUsed(t *testing.T) {
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		env.Providers = nil
		env.ProvidersErr = errors.New("ProvidersFileTooOpen", "providers.json holds API keys but other users can read it; make it private with: chmod 600 /x")
	}})
	if !has(a, "The file cannot be used") {
		t.Error("the home does not say the file cannot be used")
	}
	a.TapKey(KeyProviders)
	waitText(t, a, "The providers file cannot be used: providers.json holds API keys but other users can read it; make it private with: chmod 600 /x (ProvidersFileTooOpen)")
	if a.HasKey(KeyProviderAdd) {
		t.Error("an Add button is offered without a usable file")
	}
}

// openTest taps Test on the provider and waits for the result.
func openTest(t *testing.T, a *testutil.App, name string) {
	t.Helper()
	tapBusy(a, KeyProviderTestPrefix+name)
	whileBusy(t, a, "the test result", func() bool { return a.HasKey(KeyTestResult) })
}

func TestProviderTest(t *testing.T) {
	tests := []struct {
		name    string
		handler func(w http.ResponseWriter, n int)
		mode    grading.StructuredOutput
		want    []string
		absent  []string
	}{
		{"works", func(w http.ResponseWriter, _ int) { reply(w, "vision-1-2026", goodReply) }, grading.StructuredJSONSchema,
			[]string{"It works.", "Answer for the test photo (2 + 3 = 5): correct", "Model: vision-1-2026"}, []string{"Try another model"}},
		{"wrong verdict", func(w http.ResponseWriter, _ int) {
			reply(w, "vision-1", strings.Replace(goodReply, `"verdict":"correct"`, `"verdict":"incorrect"`, 1))
		}, grading.StructuredJSONSchema, []string{"It works.", "Answer for the test photo (2 + 3 = 5): incorrect", "A model that reads handwriting well says \"correct\" here. Try another model."}, nil},
		{"downgrade", func(w http.ResponseWriter, n int) {
			if n == 1 {
				http.Error(w, `{"error":"response_format json_schema is not supported"}`, http.StatusBadRequest)
				return
			}
			reply(w, "vision-1", goodReply)
		}, grading.StructuredJSONSchema, []string{"It works.", "The server does not accept json_schema, but it works with json_object. Edit the provider and choose json_object."}, nil},
		{"bad key", func(w http.ResponseWriter, _ int) { http.Error(w, "no", http.StatusUnauthorized) }, grading.StructuredJSONSchema,
			[]string{"The server refused the API key. Check the key. (OpenAIAuth)"}, []string{"It works."}},
		{"not found", func(w http.ResponseWriter, _ int) { http.Error(w, "no", http.StatusNotFound) }, grading.StructuredJSONSchema,
			[]string{"The server does not know this address or this model. Check the base URL and the name of the model. (OpenAINotFound)"}, nil},
		{"rate limited", func(w http.ResponseWriter, _ int) { http.Error(w, "slow down", http.StatusTooManyRequests) }, grading.StructuredJSONSchema,
			[]string{"The server says there are too many requests. Wait a little and test again. (OpenAIRateLimited)"}, nil},
		{"server error", func(w http.ResponseWriter, _ int) { http.Error(w, "oops", http.StatusInternalServerError) }, grading.StructuredJSONSchema,
			[]string{"The server has a problem. Try again later. (OpenAIServer)"}, nil},
		{"odd status", func(w http.ResponseWriter, _ int) { http.Error(w, "teapot", http.StatusTeapot) }, grading.StructuredJSONSchema,
			[]string{"The server answered with something unexpected. Check the base URL. (OpenAIStatus)"}, nil},
		{"reply is not JSON", func(w http.ResponseWriter, _ int) { reply(w, "vision-1", "I cannot read this") }, grading.StructuredJSONSchema,
			[]string{"The server answered, but not in a way Radio Tutor understands. Try another model, or the json_object mode. (GradingReplyInvalid)"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := chatServer(t, tt.handler)
			a := pump(t, config{setup: func(t *testing.T, env *app.Env) { addServerProvider(t, env, "Test AI", srv.URL, tt.mode) }})
			openProviders(t, a)
			tapBusy(a, KeyProviderTestPrefix+"Test AI")
			if !has(a, "Sending a test photo to Test AI...") && !a.HasKey(KeyTestResult) {
				t.Error("the dialog shows neither the progress nor a result")
			}
			whileBusy(t, a, "the test result", func() bool { return a.HasKey(KeyTestResult) })
			a.PumpAndSettle()
			if !has(a, "Test Test AI") {
				t.Error("the dialog does not name the provider")
			}
			for _, text := range tt.want {
				if !has(a, text) {
					t.Errorf("the result does not show %q", text)
				}
			}
			for _, text := range tt.absent {
				if has(a, text) {
					t.Errorf("the result shows %q", text)
				}
			}
			if tt.name == "works" && !a.HasKey("test-time") {
				t.Error("the result has no time")
			}
			a.TapKey(KeyDialogOK)
			waitGone(t, a, KeyTestResult)
		})
	}
}

func TestProviderTestUnreachableServer(t *testing.T) {
	srv := chatServer(t, func(w http.ResponseWriter, _ int) {})
	url := srv.URL
	srv.Close()
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		addServerProvider(t, env, "Test AI", url, grading.StructuredJSONSchema)
	}})
	openProviders(t, a)
	openTest(t, a, "Test AI")
	a.PumpAndSettle()
	if !has(a, "The server could not be reached. Check the address and that the server is running. (OpenAINetwork)") {
		t.Error("no message for an unreachable server")
	}
}

func TestProviderTestClosedWhileRunning(t *testing.T) {
	release := make(chan struct{})
	srv := chatServer(t, func(w http.ResponseWriter, _ int) {
		<-release
		reply(w, "vision-1", goodReply)
	})
	t.Cleanup(func() { close(release) })
	a := pump(t, config{setup: func(t *testing.T, env *app.Env) {
		addServerProvider(t, env, "Test AI", srv.URL, grading.StructuredJSONSchema)
	}})
	openProviders(t, a)
	tapBusy(a, KeyProviderTestPrefix+"Test AI")
	whileBusy(t, a, "the progress text", func() bool { return has(a, "Sending a test photo to Test AI...") })
	tapBusy(a, KeyDialogOK)
	whileBusy(t, a, "the dialog to close", func() bool { return !a.HasKey(KeyDialogOK) })
}

func TestTestFailureMessages(t *testing.T) {
	tests := []struct {
		tag string
		key string
	}{
		{"OpenAIAuth", "admin.test.err_auth"},
		{"OpenAINotFound", "admin.test.err_not_found"},
		{"OpenAIRateLimited", "admin.test.err_rate"},
		{"OpenAIServer", "admin.test.err_server"},
		{"OpenAITimeout", "admin.test.err_timeout"},
		{"OpenAINetwork", "admin.test.err_network"},
		{"OpenAIStatus", "admin.test.err_status"},
		{"OpenAICanceled", "admin.test.err_canceled"},
		{"GradingCanceled", "admin.test.err_canceled"},
		{"GradingReplyTruncated", "admin.test.err_reply"},
		{"GradingReplySchema", "admin.test.err_reply"},
		{"OpenAIBodyInvalid", "admin.test.err_reply"},
		{"OpenAINoChoices", "admin.test.err_reply"},
		{"OpenAIContentInvalid", "admin.test.err_reply"},
		{"OpenAIBodyTooLarge", "admin.test.err_reply"},
		{"SomethingElse", "admin.test.failed"},
	}
	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			msg := testFailure(errors.New(tt.tag, "boom"))
			if msg.Key != tt.key {
				t.Errorf("key = %s, want %s", msg.Key, tt.key)
			}
			for _, lang := range []i18n.Lang{i18n.EN, i18n.EL} {
				if text := i18n.For(lang).Msg(msg); !strings.Contains(text, tt.tag) {
					t.Errorf("%s: %q does not show the tag", lang, text)
				}
			}
		})
	}
	if msg := testFailure(io.ErrUnexpectedEOF); msg.Key != "admin.test.failed" {
		t.Errorf("a plain error gives %s", msg.Key)
	}
}

func TestEveryPresetHasADescriptionInBothLanguages(t *testing.T) {
	for _, p := range providers.Presets() {
		for _, lang := range []i18n.Lang{i18n.EN, i18n.EL} {
			if cat := i18n.For(lang); !cat.Has("admin.preset." + p.ID) {
				t.Errorf("%s has no %s description", p.ID, lang)
			}
		}
	}
}

func TestProviderErrorMessages(t *testing.T) {
	tests := []struct {
		tag string
		key string
	}{
		{"ProviderNameDuplicate", "admin.providers.err_duplicate"},
		{"ProviderNameInvalid", "admin.providers.err_name"},
		{"ProviderTimeoutInvalid", "admin.providers.err_timeout"},
		{"OpenAIConfigModel", "admin.providers.err_model"},
		{"OpenAIConfigBaseURL", "admin.providers.err_url"},
		{"OpenAIConfigBaseURLParse", "admin.providers.err_url"},
		{"ProviderNotFound", "err.generic_detail"},
	}
	for _, tt := range tests {
		if got := providerError(errors.New(tt.tag, "x")).Key; got != tt.key {
			t.Errorf("%s: key = %s, want %s", tt.tag, got, tt.key)
		}
	}
}

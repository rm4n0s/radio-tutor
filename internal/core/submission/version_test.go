package submission

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/phone"
	"github.com/rm4n0s/radio-tutor/internal/core/providers"
)

// httpProviders is a ProviderSource that gives one real (OpenAI-compatible)
// provider config.
type httpProviders struct{ cfg providers.Config }

func (p httpProviders) Active() (providers.Config, bool) { return p.cfg, true }

// TestVersionReachesTheUserAgent checks that the default BuildProvider puts
// Options.Version into the User-Agent of the requests to the provider.
func TestVersionReachesTheUserAgent(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"a version", "7.1.0", "radio-tutor/7.1.0"},
		{"no version", "", "radio-tutor/dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			var agents []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				agents = append(agents, r.Header.Get("User-Agent"))
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				content := reply(res("e1", grading.VerdictCorrect))
				_ = json.NewEncoder(w).Encode(map[string]any{
					"model": "vision-1",
					"choices": []any{map[string]any{
						"message":       map[string]any{"role": "assistant", "content": content},
						"finish_reason": "stop",
					}},
				})
			}))
			defer srv.Close()

			e := newEnv(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			svc := New(ctx, Options{
				Paths:    e.paths,
				Progress: e.progress,
				Providers: httpProviders{providers.Config{
					Name: "Test AI", Protocol: "openai", BaseURL: srv.URL + "/v1", Model: "vision-1",
					Kind: providers.KindLocal, TimeoutSeconds: 10, StructuredOutput: grading.StructuredJSONSchema,
				}},
				Settings: e.settings,
				Phone:    phone.New(phone.Options{ListenHost: "127.0.0.1"}),
				Clock:    e.clock,
				Version:  tt.version,
				DetectLAN: func(context.Context, string) (phone.LANAddr, error) {
					return phone.LANAddr{IP: net.ParseIP("127.0.0.1"), Interface: "test0"}, nil
				},
			})
			defer func() {
				if err := svc.Close(context.Background()); err != nil {
					t.Errorf("Close: %v", err)
				}
			}()
			s, err := svc.Open(ctx, "ann", e.cass, []string{"e1"})
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			if status, body := upload(t, s.URL, photo(t, 80, 60)); status != 200 {
				t.Fatalf("upload: %d %s", status, body)
			}
			for {
				ev := next(t, s)
				if _, ok := ev.(Graded); ok {
					break
				}
				if f, ok := ev.(Failed); ok {
					t.Fatalf("grading failed: %v", f.Err)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if len(agents) != 1 || agents[0] != tt.want {
				t.Errorf("User-Agent = %q, want %q", agents, tt.want)
			}
		})
	}
}

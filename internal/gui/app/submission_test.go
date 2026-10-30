package app_test

import (
	"context"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"os"
	"strings"
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

func TestStartCreatesTheSubmissionService(t *testing.T) {
	storagetest.Isolate(t)
	env, err := app.Start(context.Background(), startOptions(nil))
	if err != nil {
		t.Fatal(err)
	}
	if env.Submission == nil {
		t.Error("the submission service was not created")
	}
	if env.Providers == nil {
		t.Error("the provider store was not opened")
	}
	if env.SinkFactory == nil {
		t.Error("there is no default audio sink factory")
	}
	// Closing twice is fine: the service is closed once.
	env.Close()
	env.Close()
}

func TestStartKeepsGoingWhenProvidersCannotBeRead(t *testing.T) {
	storagetest.SkipWithoutUnixModes(t)
	paths := storagetest.Isolate(t)
	write(t, paths.ProvidersFile(), `{"format":1,"active":"","providers":[]}`)
	// Other users could read the API keys: the store refuses the file.
	if err := os.Chmod(paths.ProvidersFile(), 0o644); err != nil {
		t.Fatal(err)
	}
	env, err := app.Start(context.Background(), startOptions(nil))
	if err != nil {
		t.Fatalf("Start failed because of providers.json: %v", err)
	}
	defer env.Close()
	if env.Providers != nil {
		t.Error("an unreadable providers.json gave a provider store")
	}
	if env.Submission == nil {
		t.Error("the submission service must exist even without providers")
	}
	hasRoute(t, env.ProvidersErr, "ProvidersFileTooOpen")
	if len(env.Notices) != 1 || env.Notices[0].Key != "notice.providers_unusable" {
		t.Fatalf("notices = %v", env.Notices)
	}
	text := i18n.For(i18n.EN).Msg(env.Notices[0])
	if !strings.Contains(text, paths.ProvidersFile()) || !strings.Contains(text, "chmod 600") {
		t.Errorf("the notice does not name the file and the fix: %q", text)
	}
	data, err := os.ReadFile(paths.LogFile())
	if err != nil || !strings.Contains(string(data), "providers.json could not be read") {
		t.Errorf("the log has %q, %v", data, err)
	}
}

func TestDefaultSinkFactoryFallsBackToASilentSink(t *testing.T) {
	storagetest.Isolate(t)
	// No audio server: the sink must be the silent one and never make a sound.
	t.Setenv("PULSE_SERVER", "unix:"+t.TempDir()+"/no-such-socket")
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	env, err := app.Start(context.Background(), startOptions(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	sink, err := env.SinkFactory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()
	if sink.Name() != "silent" {
		t.Errorf("sink %q, want silent", sink.Name())
	}
}

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/rm4n0s/radio-tutor/internal/core/audio"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

func TestClaimNoSoundNoticeIsOncePerEnv(t *testing.T) {
	var first, second app.Env
	for i, want := range []bool{true, false, false} {
		if got := first.ClaimNoSoundNotice(); got != want {
			t.Errorf("claim %d = %v, want %v", i, got, want)
		}
	}
	// The flag lives in the Env, not in a global: another run starts over.
	if !second.ClaimNoSoundNotice() {
		t.Error("a new Env did not get the notice")
	}
}

func TestSilentSinkNameIsTheFallbackSinksName(t *testing.T) {
	if got := audio.NewSilentSink(nil).Name(); got != app.SilentSinkName {
		t.Errorf("the silent sink is named %q, SilentSinkName is %q", got, app.SilentSinkName)
	}
}

func TestStartAudioClock(t *testing.T) {
	fake := audio.NewFakeClock(time.Unix(0, 0))
	tests := []struct {
		name  string
		clock audio.Clock
		check func(c audio.Clock) bool
	}{
		{"the options' clock is kept", fake, func(c audio.Clock) bool { return c == audio.Clock(fake) }},
		{"the system clock is the default", nil, func(c audio.Clock) bool { return c != nil && c != audio.Clock(fake) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storagetest.Isolate(t)
			opts := startOptions(nil)
			opts.AudioClock = tt.clock
			env, err := app.Start(context.Background(), opts)
			if err != nil {
				t.Fatal(err)
			}
			defer env.Close()
			if !tt.check(env.AudioClock) {
				t.Errorf("Env.AudioClock = %v", env.AudioClock)
			}
		})
	}
}

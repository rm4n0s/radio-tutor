package audio

import (
	"context"
	"testing"

	"github.com/rm4n0s/errors"
)

// These tests run on Windows, and under Wine with its audio drivers disabled
// (scripts/wine-exec.sh). They open no device and play nothing.

func TestWinmmProcsLoad(t *testing.T) {
	tests := []struct {
		name  string
		dll   string
		route string // empty: no error
	}{
		{"winmm", "winmm.dll", ""},
		{"missing dll", "radio-tutor-no-such-library.dll", "loadWinmm.WaveOutLoadFailed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := loadWinmm(tc.dll)
			if tc.route == "" {
				if err != nil {
					t.Fatal(err)
				}
				if p.getNumDevs == nil || p.open == nil || p.getPosition == nil {
					t.Fatal("the table is not filled")
				}
				return
			}
			assertRoute(t, err, tc.route)
		})
	}
}

func TestNewWaveOutSinkCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewWaveOutSink(ctx)
	assertRoute(t, err, "NewWaveOutSink.WaveOutStartCanceled")
}

// TestOpenDefaultSinkWithoutDevice is the fallback under Wine, whose audio
// drivers are off: there is no output device, so the app plays without sound.
// On a PC that has a device the test is skipped (it would only open it).
func TestOpenDefaultSinkWithoutDevice(t *testing.T) {
	procs, err := loadWinmm("winmm.dll")
	if err != nil {
		t.Fatal(err)
	}
	b := &winmmBackend{p: procs}
	if n := b.deviceCount(); n > 0 {
		t.Skipf("this machine has %d sound output device(s); the real-device test carries the radiotutor_audio tag", n)
	}
	sink, reason := OpenDefaultSink(context.Background())
	sink.Close() // the silent sink holds nothing; the name is read after
	if sink.Name() != "silent" {
		t.Fatalf("sink = %q, want the silent fallback", sink.Name())
	}
	assertRoute(t, reason, "NewWaveOutSink.WaveOutNoDevice")
	if _, ok := errors.FromError(reason); !ok {
		t.Fatalf("reason is %T", reason)
	}
	// A sink made before the device went away says the same at Start.
	late := newWaveOutSink(b, WaveOutOptions{})
	defer late.Close()
	err = late.Start(context.Background(), clipSourceOf(rampClip(100)), Format{SampleRate: 1000, Channels: 1})
	assertRoute(t, err, "WaveOutSink.Start.WaveOutNoDevice")
}

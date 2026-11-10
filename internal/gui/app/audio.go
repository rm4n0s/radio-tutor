package app

import (
	"context"

	gaudio "github.com/rm4n0s/glutter/audio"

	"github.com/rm4n0s/radio-tutor/internal/core/audio"
)

// SinkFactory opens an audio sink for a screen that plays sound (the prompt
// audio of the worksheet, later the lesson player). The caller closes the sink.
// Env.SinkFactory is the default one; tests replace it with a factory that
// returns an audio.FakeSink, so no test makes a sound (CLAUDE.md).
type SinkFactory func(ctx context.Context) (audio.Sink, error)

// DefaultSinkFactory opens audio.OpenDefaultSink: the PulseAudio (or PipeWire)
// server on Linux or the waveOut device on Windows when one answers, otherwise a silent sink. On macOS it is the sink of Glutter's
// audio (Core Audio; glutterSink), which opens its device when the first sound plays and plays silence, with an error in the log,
// when there is none. It never fails; env logs the reason for the fallback.
func (e *Env) DefaultSinkFactory(ctx context.Context) (audio.Sink, error) {
	if useGlutterAudio {
		if b := e.binding.Load(); b != nil {
			return newGlutterSink(gaudio.ForBinding(b)), nil
		}
	}
	sink, reason := audio.OpenDefaultSink(ctx)
	if reason != nil {
		e.Log.Warn("no sound device; playing without sound", "err", reason)
	}
	return sink, nil
}

// SilentSinkName is the name of the sink audio.OpenDefaultSink falls back to
// when no sound server answers (audio.SilentSink.Name). A screen that sees it
// tells the child that the lesson plays without sound (SPEC §8.1).
const SilentSinkName = "silent"

// ClaimNoSoundNotice reports whether the caller is the first this run to ask,
// and so should show "No sound device found" (SPEC §8.1: once per run).
func (e *Env) ClaimNoSoundNotice() bool {
	return e.noSoundShown.CompareAndSwap(false, true)
}

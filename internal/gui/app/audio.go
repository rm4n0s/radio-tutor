package app

import (
	"context"

	"github.com/rm4n0s/radio-tutor/internal/core/audio"
)

// SinkFactory opens an audio sink for a screen that plays sound (the prompt
// audio of the worksheet, later the lesson player). The caller closes the sink.
// Env.SinkFactory is the default one; tests replace it with a factory that
// returns an audio.FakeSink, so no test makes a sound (CLAUDE.md).
type SinkFactory func(ctx context.Context) (audio.Sink, error)

// DefaultSinkFactory opens audio.OpenDefaultSink: the PulseAudio (or PipeWire)
// server when one answers, otherwise a silent sink. It never fails; env logs
// the reason for the fallback.
func (e *Env) DefaultSinkFactory(ctx context.Context) (audio.Sink, error) {
	sink, reason := audio.OpenDefaultSink(ctx)
	if reason != nil {
		e.Log.Warn("no sound device; playing without sound", "err", reason)
	}
	return sink, nil
}

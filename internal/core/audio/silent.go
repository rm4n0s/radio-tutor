package audio

import (
	"context"

	"github.com/rm4n0s/errors"
)

// SilentSink plays nothing: its position advances with the wall clock while
// it is playing, as if the samples were played in real time. It is used when
// no audio server answers (SPEC §8.1), so that a lesson can still be played
// and completed without sound.
type SilentSink struct {
	track *tracker
}

// NewSilentSink returns a SilentSink on clock c; nil means SystemClock().
func NewSilentSink(c Clock) *SilentSink {
	if c == nil {
		c = SystemClock()
	}
	return &SilentSink{track: newTracker(c)}
}

// Name returns "silent".
func (s *SilentSink) Name() string { return "silent" }

// Start starts the clock at 0. The samples of src are not read. It fails
// only when ctx is already cancelled.
func (s *SilentSink) Start(ctx context.Context, _ Source, _ Format) error {
	if err := ctx.Err(); err != nil {
		return errors.NewErr("SilentStartCanceled", err)
	}
	s.track.start()
	return nil
}

// Pause holds the clock.
func (s *SilentSink) Pause() { s.track.pause() }

// Resume lets the clock run again.
func (s *SilentSink) Resume(context.Context) error {
	s.track.resume()
	return nil
}

// Stop stops the clock and sets it to 0.
func (s *SilentSink) Stop() { s.track.stop() }

// Position returns the seconds the clock has run since Start, not counting
// the time paused.
func (s *SilentSink) Position() float64 { return s.track.position() }

// Close stops the clock.
func (s *SilentSink) Close() { s.track.stop() }

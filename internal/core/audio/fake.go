package audio

import (
	"context"
	"sync"

	"github.com/rm4n0s/errors"
)

// FakeSink is the Sink for tests. It makes no sound. Its position follows
// its Clock like SilentSink's, and it simulates the device: as the position
// advances it pulls the samples that have "been played" from the Source and
// keeps them, so a test can check what a Player delivered and when. It also
// records every call. It is safe for concurrent use.
type FakeSink struct {
	track *tracker

	mu       sync.Mutex
	failure  error
	calls    []string
	src      Source
	format   Format
	pulled   int64 // frames pulled from src
	played   []float32
	srcEnded bool
}

// NewFakeSink returns a FakeSink on clock c, which tests usually make a
// FakeClock. A nil c means SystemClock().
func NewFakeSink(c Clock) *FakeSink {
	if c == nil {
		c = SystemClock()
	}
	return &FakeSink{track: newTracker(c)}
}

// FailStartWith makes the next and every later Start and Resume return err
// (nil undoes it).
func (s *FakeSink) FailStartWith(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failure = err
}

// Name returns "fake".
func (s *FakeSink) Name() string { return "fake" }

// Start records the call, resets the played samples and starts the clock at
// 0.
func (s *FakeSink) Start(ctx context.Context, src Source, f Format) error {
	if err := ctx.Err(); err != nil {
		return errors.NewErr("FakeStartCanceled", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, "start")
	if s.failure != nil {
		return s.failure
	}
	s.src, s.format = src, f
	s.pulled, s.played, s.srcEnded = 0, nil, false
	s.track.start()
	return nil
}

// Pause records the call and holds the clock.
func (s *FakeSink) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, "pause")
	s.track.pause()
}

// Resume records the call and lets the clock run again. It fails with the
// error given to FailStartWith, like Start.
func (s *FakeSink) Resume(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, "resume")
	if s.failure != nil {
		return s.failure
	}
	s.track.resume()
	return nil
}

// Stop records the call and stops the clock at 0. The played samples stay
// until the next Start.
func (s *FakeSink) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, "stop")
	s.track.stop()
}

// Close records the call and stops the clock.
func (s *FakeSink) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, "close")
	s.track.stop()
}

// Position returns the seconds played on the clock and pulls the samples up
// to that point from the Source.
func (s *FakeSink) Position() float64 {
	pos := s.track.position()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.drain(pos)
	return pos
}

// drain pulls from the source until the frames played cover pos. s.mu must
// be held.
func (s *FakeSink) drain(pos float64) {
	if s.src == nil || s.srcEnded || s.format.SampleRate <= 0 || s.format.Channels <= 0 {
		return
	}
	want := int64(pos * float64(s.format.SampleRate))
	buf := make([]float32, 1024*s.format.Channels)
	for s.pulled < want {
		n := min(int64(len(buf)/s.format.Channels), want-s.pulled)
		got := s.src.Read(buf[:n*int64(s.format.Channels)])
		if got == 0 {
			s.srcEnded = true
			return
		}
		s.played = append(s.played, buf[:got]...)
		s.pulled += int64(got / s.format.Channels)
	}
}

// Calls returns the names of the calls received so far, in order: "start",
// "pause", "resume", "stop", "close".
func (s *FakeSink) Calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.calls...)
}

// Played returns the samples the simulated device has taken from the Source
// since the last Start, up to the current position.
func (s *FakeSink) Played() []float32 {
	pos := s.track.position()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.drain(pos)
	return append([]float32(nil), s.played...)
}

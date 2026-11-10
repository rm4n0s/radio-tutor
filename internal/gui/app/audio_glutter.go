package app

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/rm4n0s/errors"
	gaudio "github.com/rm4n0s/glutter/audio"

	"github.com/rm4n0s/radio-tutor/internal/core/audio"
)

// glutterSink plays through Glutter's audio (audio.System of the window's binding), which is Core Audio on macOS. The core's own
// sinks speak PulseAudio and winmm, so on a Mac the sink is made here, where the binding is. It opens the playback device when the
// first sound starts and keeps it until Close; each Start makes a stream that pulls from the source on the audio thread.
type glutterSink struct {
	sys *gaudio.System

	mu     sync.Mutex // Start, Pause, Resume, Stop and Close
	dev    *gaudio.Device
	stream *gaudio.Stream
	paused bool
	closed bool

	// feed is what the audio thread reads; replaced by Start and Stop.
	feed atomic.Pointer[glutterFeed]
	// played is the frames of the last feed that were handed to the device, the position's source once the feed is gone.
	played atomic.Int64
	rate   atomic.Int64
}

// glutterFeed is one run of the source, with the frames the audio thread took from it.
type glutterFeed struct {
	src      audio.Source
	channels int
	frames   atomic.Int64
}

func newGlutterSink(sys *gaudio.System) *glutterSink { return &glutterSink{sys: sys} }

// Name is "glutter".
func (s *glutterSink) Name() string { return "glutter" }

// Start plays src from its current point in format f. It fails with the tag GlutterSinkOpenFailed when the playback device cannot
// be opened; the screen then plays without sound.
func (s *glutterSink) Start(ctx context.Context, src audio.Source, f audio.Format) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("GlutterSinkClosed", "the sink is closed")
	}
	s.stopLocked()
	if s.dev == nil {
		dev, err := s.sys.OpenPlayback(ctx)
		if err != nil {
			return errors.NewErr("GlutterSinkOpenFailed", err)
		}
		s.dev = dev
	}
	feed := &glutterFeed{src: src, channels: f.Channels}
	s.feed.Store(feed)
	s.played.Store(0)
	s.rate.Store(int64(f.SampleRate))
	s.stream = s.dev.NewCallbackStream(gaudio.Format{Rate: f.SampleRate, Channels: f.Channels}, func(out []float32) {
		n := feed.src.Read(out)
		clear(out[n:])
		feed.frames.Add(int64(n / feed.channels))
		s.played.Store(feed.frames.Load())
	})
	if s.paused {
		return s.resumeLocked()
	}
	return nil
}

// Pause holds playback and the position.
func (s *glutterSink) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dev == nil || s.stream == nil || s.paused {
		return
	}
	if err := s.dev.Pause(); err != nil {
		return // the device then keeps playing; there is nobody to tell and the next Stop ends it
	}
	s.paused = true
}

// Resume continues after Pause. A device that cannot be resumed leaves the sink paused, and the error has the tag
// GlutterSinkResumeFailed.
func (s *glutterSink) Resume(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.paused {
		return nil
	}
	return s.resumeLocked()
}

func (s *glutterSink) resumeLocked() error {
	if s.dev == nil {
		return nil
	}
	if err := s.dev.Resume(); err != nil {
		return errors.NewErr("GlutterSinkResumeFailed", err)
	}
	s.paused = false
	return nil
}

// Stop ends playback and sets the position to 0.
func (s *glutterSink) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
}

func (s *glutterSink) stopLocked() {
	if s.stream != nil {
		s.stream.Close()
		s.stream = nil
	}
	s.feed.Store(nil)
	s.played.Store(0)
}

// Position is the seconds of the source that were handed to the device since Start: the device plays them a buffer later, which is
// a few tens of milliseconds.
func (s *glutterSink) Position() float64 {
	rate := s.rate.Load()
	if rate == 0 {
		return 0
	}
	return float64(s.played.Load()) / float64(rate)
}

// Close stops playback and closes the device.
func (s *glutterSink) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
	if s.dev != nil {
		_ = s.dev.Close() //nolint:errcheck // the sink is going away; the device is closed with the app anyway
		s.dev = nil
	}
	s.closed = true
}

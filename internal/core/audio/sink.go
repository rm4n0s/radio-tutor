package audio

import (
	"context"
	"sync"
	"time"
)

// Format describes the samples a Source delivers.
type Format struct {
	// SampleRate is in Hz.
	SampleRate int
	// Channels is 1 or 2.
	Channels int
}

// Source supplies the samples a Sink plays. A sink pulls from its source
// when the device wants more data (PulseAudio asks for data; it is not
// pushed), on a goroutine of its own, and only one goroutine reads at a
// time.
type Source interface {
	// Read copies up to len(dst) interleaved float32 samples into dst and
	// returns how many it copied, always a whole number of frames. It returns
	// 0 when the source is at its end.
	Read(dst []float32) int
	// SeekFrame moves the next Read to the given frame (clamped to the
	// length of the source). A sink that pauses by closing its stream would use
	// it to continue where it was (PulseSink corks its stream and does not
	// need it); the user can never seek (SPEC D5).
	SeekFrame(frame int64)
}

// Sink plays the samples of a Source and reports how much of them has
// actually been played. It is what the Player drives. The implementations
// are PulseSink (real sound), SilentSink (no sound, a wall clock) and
// FakeSink (tests).
//
// Start, Pause, Resume, Stop and Close are called by one goroutine at a time
// (the Player serializes them) and may block for a round trip to the audio
// server. Position may be called from any goroutine, at any time, and never
// blocks on the server.
type Sink interface {
	// Name is "pulse", "silent" or "fake".
	Name() string
	// Start stops any playback in progress and starts playing src from its
	// current point, in format f. The position restarts at 0.
	Start(ctx context.Context, src Source, f Format) error
	// Pause holds playback and the position. It does nothing when nothing is
	// playing.
	Pause()
	// Resume continues after Pause. It does nothing when not paused. It can
	// fail (a sink may have to ask the audio server); the sink then stays
	// paused.
	Resume(ctx context.Context) error
	// Stop ends playback and sets the position to 0. The sink can be started
	// again.
	Stop()
	// Position returns the seconds of the source that have been played since
	// Start, not counting the time paused. It never decreases until Stop or
	// Start.
	Position() float64
	// Close stops playback and releases the sink. The sink cannot be used
	// afterwards.
	Close()
}

// tracker turns a start/pause/resume/stop history and a Clock into a playback
// position, for the sinks that have no device to ask (SilentSink, FakeSink)
// or that ask it only now and then (PulseSink, which feeds it measurements
// through observe). All its methods are safe for concurrent use and cheap.
type tracker struct {
	clock Clock

	mu      sync.Mutex
	running bool
	base    float64   // seconds played at anchor
	anchor  time.Time // clock reading at which the position was base
	high    float64   // the highest position reported since start
	epoch   uint64    // changes whenever running or base is changed by a call
}

func newTracker(c Clock) *tracker { return &tracker{clock: c} }

// raw is the position without the monotonic guard. mu must be held.
func (t *tracker) raw() float64 {
	p := t.base
	if t.running {
		p += t.clock.Now().Sub(t.anchor).Seconds()
	}
	if p < 0 {
		p = 0
	}
	return p
}

func (t *tracker) start() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.epoch++
	t.running = true
	t.base = 0
	t.high = 0
	t.anchor = t.clock.Now()
}

func (t *tracker) pause() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.running {
		return
	}
	t.epoch++
	t.base = t.raw()
	t.high = max(t.high, t.base)
	t.running = false
}

func (t *tracker) resume() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.running {
		return
	}
	t.epoch++
	t.running = true
	t.base = t.high
	t.anchor = t.clock.Now()
}

func (t *tracker) stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.epoch++
	t.running = false
	t.base = 0
	t.high = 0
}

// position returns the played seconds. It never goes down between a start and
// a stop: a measurement that puts the position behind what was already
// reported holds the position until time catches up.
func (t *tracker) position() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.high = max(t.high, t.raw())
	return t.high
}

// token returns the current epoch and whether the clock is running. A
// measurement taken now is valid for observe only while the epoch is the
// same.
func (t *tracker) token() (epoch uint64, running bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.epoch, t.running
}

// observe records that the position was pos seconds at the clock reading at.
// It is ignored (and false is returned) when the sink was started, paused,
// resumed or stopped since the measurement began (epoch is the token taken
// before it).
func (t *tracker) observe(epoch uint64, pos float64, at time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if epoch != t.epoch || !t.running {
		return false
	}
	t.base = pos
	t.anchor = at
	return true
}

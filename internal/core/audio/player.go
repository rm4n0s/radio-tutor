package audio

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rm4n0s/errors"
)

// State is the state of a Player.
type State int

const (
	// StateStopped means nothing is playing and the position is 0. It is
	// also the state of a Player with no clip loaded.
	StateStopped State = iota
	// StatePlaying means the audio is playing and the position advances.
	StatePlaying
	// StatePaused means the position is held where Pause left it.
	StatePaused
	// StateEnded means the clip played to its end; the position stays at the
	// duration until Play (which restarts from 0) or Stop.
	StateEnded
)

// String returns the lower-case name of the state.
func (s State) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StatePlaying:
		return "playing"
	case StatePaused:
		return "paused"
	case StateEnded:
		return "ended"
	}
	return "unknown"
}

// endSlack is how close to the duration the position has to be to count as
// the end, in seconds. It absorbs float rounding.
const endSlack = 1e-6

// minRecheck is the shortest wait before the Player looks at the position
// again when a sink's position is behind the Player's end timer.
const minRecheck = 5 * time.Millisecond

// Player plays one Clip at a time through a Sink: Load, Play, Pause, Stop.
// There is no seeking (SPEC D5). It is the lesson clock of SPEC §8.2: the
// UI reads Position once per frame.
//
// Concurrency. All methods are safe for concurrent use. Position and State
// take a mutex that is held only for short, non-blocking work (the sink's
// Position never waits for the audio server), so they are cheap enough to
// call every frame from the UI goroutine. Play, Pause, Stop and Load may
// wait for a round trip to the audio server (milliseconds, bounded by a
// timeout); the UI calls them from event handlers.
//
// The end of the clip is detected by a timer of the Player's Clock, which
// fires on its own goroutine (the Clock's: a runtime timer goroutine for the
// SystemClock, the caller of Advance for a FakeClock). That goroutine moves
// the Player to StateEnded, stops the sink and signals Ended. The signal
// carries no widget work: the UI subscribes to Ended from its own goroutine
// and hands the news over to the UI thread with SetState or the binding's
// Post. Position never changes the state, so it is always safe to call
// from the UI goroutine.
type Player struct {
	sink  Sink
	clock Clock
	ended chan struct{}

	mu     sync.Mutex
	clip   *Clip
	state  State
	held   float64 // position while paused
	open   bool    // the sink has a stream (Start succeeded, not Stopped since)
	timer  Timer
	gen    uint64 // changes whenever the end timer is replaced or cancelled
	closed bool

	// level is the gain the sink's goroutine applies to the samples it reads,
	// the bits of a float32, changed by SetVolume and SetMuted while the clip
	// plays. volMu guards the setting behind it; neither needs mu, so that the
	// UI can change the volume while a Play waits for the sound server.
	level atomic.Uint32
	volMu sync.Mutex
	vol   float32 // the gain of the volume setting (SetVolume)
	muted bool    // SetMuted: the level is 0, whatever the volume
}

// NewPlayer returns a Player that plays through sink and reads time from
// clock (nil means SystemClock()). The Player does not own the sink: Close
// does not close it, so that one sink can serve many players.
func NewPlayer(sink Sink, clock Clock) *Player {
	if clock == nil {
		clock = SystemClock()
	}
	p := &Player{sink: sink, clock: clock, ended: make(chan struct{}, 1), vol: 1}
	p.level.Store(math.Float32bits(1))
	return p
}

// Ended returns a channel that receives one value each time a clip plays to
// its end (not when it is stopped or paused). The channel has room for one
// value and the Player never blocks on it, so ends that nobody read are
// merged into one. It is closed by nobody and is the same channel for the
// life of the Player. The value is sent from the goroutine of the end timer
// (see the Player's documentation), after State is StateEnded.
func (p *Player) Ended() <-chan struct{} { return p.ended }

// Load replaces the clip. It stops whatever was playing; the Player is
// StateStopped at position 0. The clip must not be changed afterwards.
func (p *Player) Load(clip *Clip) error {
	if clip == nil || clip.SampleRate <= 0 || (clip.Channels != 1 && clip.Channels != 2) || len(clip.Samples)%clip.Channels != 0 {
		return errors.New("PlayerClipInvalid", "the clip needs a sample rate, one or two channels and whole frames")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return errors.New("PlayerClosed", "the player is closed")
	}
	p.stopLocked()
	p.clip = clip
	return nil
}

// Play starts the clip at the beginning, or continues it after Pause. After
// the end it starts again from 0 (SPEC §8). It does nothing while playing.
// An empty clip ends at once. It fails when no clip is loaded, when the
// Player is closed, when ctx is cancelled, or with the sink's error when the
// sink cannot start.
func (p *Player) Play(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return errors.New("PlayerClosed", "the player is closed")
	}
	if p.clip == nil {
		return errors.New("PlayerNoClip", "no clip is loaded")
	}
	if err := ctx.Err(); err != nil {
		return errors.NewErr("PlayerCanceled", err)
	}
	switch p.state {
	case StatePlaying:
		return nil
	case StatePaused:
		if err := p.sink.Resume(ctx); err != nil {
			return err
		}
		p.state = StatePlaying
		p.scheduleLocked()
		return nil
	}
	// Stopped or Ended: start from 0.
	p.stopLocked()
	if p.clip.Frames() == 0 {
		p.state = StateEnded
		p.signalEnded()
		return nil
	}
	f := Format{SampleRate: p.clip.SampleRate, Channels: p.clip.Channels}
	src := &clipSource{samples: p.clip.Samples, channels: p.clip.Channels, level: &p.level}
	if err := p.sink.Start(ctx, src, f); err != nil {
		return err
	}
	p.open = true
	p.state = StatePlaying
	p.scheduleLocked()
	return nil
}

// Pause holds the clip and its position. It does nothing unless the Player
// is playing.
func (p *Player) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state != StatePlaying {
		return
	}
	pos := p.positionLocked()
	if pos >= p.durationLocked()-endSlack {
		p.finishLocked()
		return
	}
	p.cancelTimerLocked()
	p.sink.Pause()
	p.held = pos
	p.state = StatePaused
}

// Stop ends playback and goes back to 0 (SPEC §8: "Stop pauses and goes
// back to 0"). It is the only call that makes Position decrease.
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
}

// Close stops the Player. Afterwards Load and Play fail. It does not close
// the sink.
func (p *Player) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
	p.closed = true
}

// State returns the current state.
func (p *Player) State() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// Position returns the seconds of the clip that have been played: the lesson
// time of SPEC §8.2. It is 0 when stopped, holds when paused, advances while
// playing, and stays at Duration after the end. Between a Play and the next
// Stop it never decreases. It is cheap and never blocks on the audio server.
func (p *Player) Position() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.positionLocked()
}

// Duration returns the length of the loaded clip in seconds (0 with no clip).
func (p *Player) Duration() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.durationLocked()
}

func (p *Player) durationLocked() float64 {
	if p.clip == nil {
		return 0
	}
	return p.clip.Seconds()
}

func (p *Player) positionLocked() float64 {
	switch p.state {
	case StatePlaying:
		return math.Min(p.sink.Position(), p.durationLocked())
	case StatePaused:
		return p.held
	case StateEnded:
		return p.durationLocked()
	}
	return 0
}

// stopLocked is Stop. p.mu must be held.
func (p *Player) stopLocked() {
	p.cancelTimerLocked()
	if p.open {
		p.sink.Stop()
		p.open = false
	}
	p.held = 0
	p.state = StateStopped
}

// finishLocked moves a playing clip to the end. p.mu must be held.
func (p *Player) finishLocked() {
	p.cancelTimerLocked()
	if p.open {
		p.sink.Stop()
		p.open = false
	}
	p.held = 0
	p.state = StateEnded
	p.signalEnded()
}

func (p *Player) signalEnded() {
	select {
	case p.ended <- struct{}{}:
	default:
	}
}

func (p *Player) cancelTimerLocked() {
	p.gen++
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
}

// scheduleLocked arms the end timer for the time that remains. p.mu must be
// held.
func (p *Player) scheduleLocked() {
	p.cancelTimerLocked()
	remaining := p.durationLocked() - p.sink.Position()
	d := time.Duration(math.Ceil(remaining * float64(time.Second)))
	if d < minRecheck {
		d = minRecheck
	}
	gen := p.gen
	p.timer = p.clock.AfterFunc(d, func() { p.onTimer(gen) })
}

// onTimer runs on the clock's goroutine when the end timer fires.
func (p *Player) onTimer(gen uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if gen != p.gen || p.state != StatePlaying {
		return
	}
	if p.sink.Position() >= p.durationLocked()-endSlack {
		p.finishLocked()
		return
	}
	// The sink's position is behind the estimate (a late measurement, a
	// device that started late): look again when the rest is due.
	p.scheduleLocked()
}

// clipSource is the Source of a Clip: the samples from the start, multiplied by
// the gain in level, which may change between two reads. A nil level plays the
// samples as they are.
type clipSource struct {
	samples  []float32
	channels int
	pos      int
	level    *atomic.Uint32
}

func (s *clipSource) SeekFrame(frame int64) {
	s.pos = int(min(max(frame, 0), int64(len(s.samples)/s.channels))) * s.channels
}

func (s *clipSource) Read(dst []float32) int {
	n := min(len(dst), len(s.samples)-s.pos)
	n -= n % s.channels
	if n <= 0 {
		return 0
	}
	gain := float32(1)
	if s.level != nil {
		gain = math.Float32frombits(s.level.Load())
	}
	if gain == 1 {
		copy(dst, s.samples[s.pos:s.pos+n])
	} else {
		for i, v := range s.samples[s.pos : s.pos+n] {
			// Clamp after the gain so a gain above 1 could not overflow.
			dst[i] = min(max(v*gain, -1), 1)
		}
	}
	s.pos += n
	return n
}

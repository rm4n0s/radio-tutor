package audio

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rm4n0s/errors"
)

// Defaults of the waveOut sink: the device is fed through a ring of
// waveOutBuffers buffers of waveOutBufferTime each, 200 ms of sound in all. A
// change of the volume is heard when the buffers that were already filled have
// played, so the ring is short; it is long enough that a goroutine that is
// late by a few tens of milliseconds does not make the sound stutter.
const (
	waveOutBuffers    = 4
	waveOutBufferTime = 50 * time.Millisecond
	// waveOutWaitSlice is the longest the fill goroutine waits for the device
	// before it looks again (and measures the position).
	waveOutWaitSlice = 100 * time.Millisecond
	// waveOutJoinTimeout is how long Stop waits for the fill goroutine.
	waveOutJoinTimeout = 2 * time.Second
)

// WaveOutSink plays through winmm's waveOut API on Windows (waveOutOpen and
// the calls around it, from winmm.dll, in pure Go with no cgo), which every
// Windows 10 and 11 has and Wine implements. The type exists on every system
// so that its logic is tested on Linux with a fake winmm; NewWaveOutSink, which
// talks to the real one, exists on Windows only.
//
// The sound is 16-bit PCM at the sample rate and the channel count of the
// Source (WAVE_MAPPER converts to whatever the device wants). The gain stays
// in the Player's Source, as for every sink. The device is fed from a ring of
// a few buffers of 50 ms. A goroutine of the sink waits on a Windows event
// that winmm signals whenever it has finished a buffer (CALLBACK_EVENT; a
// callback function has strict rules about what it may call and runs on a
// thread of winmm), takes the finished buffers back, fills them with the
// Source's next samples (silence after the Source has ended, so that the
// stream keeps running until the Player stops it, as PulseSink does) and
// queues them again. Only that goroutine reads the Source, after Start has
// filled the ring.
//
// Pause pauses the device (waveOutPause: it stops at once, keeps the buffers
// that are queued and its position), and Resume restarts it (waveOutRestart),
// which goes on with exactly the samples that were queued, so nothing is
// skipped or repeated and the Source is never moved. The played position is
// asked from the device (waveOutGetPosition counts the samples the device has
// played since it was opened, not counting a pause) at every wake-up of the
// goroutine, and extrapolated with the Clock in between, which is held while
// paused, through the same tracker as PulseSink: the position never goes back,
// and it goes on by the Clock when the device stops answering, so a lesson
// still finishes. Stop resets and closes the device (the next Start opens it
// again, at position 0).
type WaveOutSink struct {
	backend waveBackend
	clock   Clock
	track   *tracker
	log     *slog.Logger

	buffers     int
	bufferTime  time.Duration
	waitSlice   time.Duration
	joinTimeout time.Duration

	mu       sync.Mutex // serializes Start, Pause, Resume, Stop and Close
	run      *waveRun
	closed   bool
	paused   bool  // the device is paused
	pauseErr error // the error of a Pause that failed: Resume returns it
}

// WaveOutOptions configures NewWaveOutSink. The zero value is fine.
type WaveOutOptions struct {
	// Clock is the time source; nil means SystemClock().
	Clock Clock
	// Logger receives the errors the sink cannot return (a failed Pause, a
	// failed write in the background); nil means slog.Default().
	Logger *slog.Logger
}

func newWaveOutSink(b waveBackend, o WaveOutOptions) *WaveOutSink {
	if o.Clock == nil {
		o.Clock = SystemClock()
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	return &WaveOutSink{
		backend:     b,
		clock:       o.Clock,
		track:       newTracker(o.Clock),
		log:         o.Logger,
		buffers:     waveOutBuffers,
		bufferTime:  waveOutBufferTime,
		waitSlice:   waveOutWaitSlice,
		joinTimeout: waveOutJoinTimeout,
	}
}

// waveBackend is the part of winmm the sink uses, so that its logic is tested
// without it. The real one is in winmm_windows.go; errors it returns are
// *waveError.
type waveBackend interface {
	// deviceCount is waveOutGetNumDevs: how many output devices there are.
	deviceCount() int
	// open opens the default output device (WAVE_MAPPER) for 16-bit PCM in
	// format f, with buffers buffers of bufBytes bytes each prepared for it,
	// and leaves the device paused and the position at 0.
	open(f Format, buffers, bufBytes int) (waveDevice, error)
}

// waveDevice is one open output device with its ring of buffers. The memory
// of a buffer is out of the reach of the garbage collector and does not move
// while the device is open (winmm keeps its address).
type waveDevice interface {
	// buffer returns the memory of buffer i, bufBytes bytes long.
	buffer(i int) []byte
	// write queues the first n bytes of buffer i for playing (waveOutWrite).
	write(i, n int) error
	// done reports whether the device has finished buffer i (WHDR_DONE) since
	// it was written.
	done(i int) bool
	// wait returns when the device has finished a buffer, when wake is called
	// or when d has passed, whichever comes first.
	wait(d time.Duration)
	// wake makes a wait that is running, or the next one, return at once.
	wake()
	// pause stops the device where it is, restart goes on (waveOutPause,
	// waveOutRestart).
	pause() error
	restart() error
	// position returns the frames the device has played since it was opened,
	// not counting a pause. ok is false when the device does not tell.
	position() (frames int64, ok bool)
	// reset stops the device and gives every queued buffer back as done
	// (waveOutReset); the position goes to 0.
	reset() error
	// close releases the buffers, the device and the event. Call it after
	// reset and when no other goroutine is using the device. When it fails,
	// the memory of the buffers is not released (winmm may still use it).
	close() error
}

// waveError is the failure of a winmm call.
type waveError struct {
	// Func is the name of the call, such as "waveOutOpen".
	Func string
	// Code is its MMRESULT.
	Code uint32
}

func (e *waveError) Error() string {
	return fmt.Sprintf("%s failed: %s (%d)", e.Func, mmResultName(e.Code), e.Code)
}

// mmResultName names the MMRESULT codes winmm's waveOut calls return.
func mmResultName(code uint32) string {
	switch code {
	case 0:
		return "MMSYSERR_NOERROR"
	case 2:
		return "MMSYSERR_BADDEVICEID"
	case 4:
		return "MMSYSERR_ALLOCATED"
	case 5:
		return "MMSYSERR_INVALHANDLE"
	case 6:
		return "MMSYSERR_NODRIVER"
	case 7:
		return "MMSYSERR_NOMEM"
	case 10:
		return "MMSYSERR_HANDLEBUSY"
	case 11:
		return "MMSYSERR_INVALPARAM"
	case 32:
		return "WAVERR_BADFORMAT"
	case 33:
		return "WAVERR_STILLPLAYING"
	case 34:
		return "WAVERR_UNPREPARED"
	case 35:
		return "WAVERR_SYNC"
	}
	return "unknown MMRESULT"
}

// waveRun is one play: the device and the goroutine that feeds it.
type waveRun struct {
	dev      waveDevice
	src      Source
	scratch  []float32 // one buffer of samples from src
	bufBytes int
	channels int
	rate     int

	head  int // the oldest buffer in the device
	count int // how many buffers are in the device

	stop atomic.Bool
	done chan struct{} // closed when the goroutine has ended
}

// Name returns "waveout".
func (s *WaveOutSink) Name() string { return "waveout" }

// Start opens the device for src in format f and starts it; an earlier play is
// stopped first. It fails, tagged WaveOutClosed, WaveOutFormatUnsupported,
// WaveOutStartCanceled, WaveOutNoDevice (no output device), WaveOutOpenFailed,
// WaveOutWriteFailed or WaveOutRestartFailed.
func (s *WaveOutSink) Start(ctx context.Context, src Source, f Format) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("WaveOutClosed", "the sink is closed")
	}
	if f.SampleRate <= 0 || (f.Channels != 1 && f.Channels != 2) {
		return errors.New("WaveOutFormatUnsupported", "the sink plays mono or stereo at a positive sample rate",
			"sample_rate", f.SampleRate, "channels", f.Channels)
	}
	if err := ctx.Err(); err != nil {
		return errors.NewErr("WaveOutStartCanceled", err)
	}
	s.stopLocked()

	if n := s.backend.deviceCount(); n <= 0 {
		return errors.New("WaveOutNoDevice", "there is no sound output device")
	}
	bufFrames := max(int(int64(f.SampleRate)*int64(s.bufferTime)/int64(time.Second)), 1)
	bufBytes := bufFrames * f.Channels * 2
	dev, err := s.backend.open(f, s.buffers, bufBytes)
	if err != nil {
		return errors.NewErr("WaveOutOpenFailed", err).SetMetadata("sample_rate", f.SampleRate, "channels", f.Channels)
	}
	r := &waveRun{
		dev:      dev,
		src:      src,
		scratch:  make([]float32, bufFrames*f.Channels),
		bufBytes: bufBytes,
		channels: f.Channels,
		rate:     f.SampleRate,
		done:     make(chan struct{}),
	}
	// The ring is filled while the device is paused, so that it starts with
	// all of it queued and cannot run dry before the first refill.
	for r.count < s.buffers {
		if err := r.fill((r.head + r.count) % s.buffers); err != nil {
			s.release(r)
			return errors.NewErr("WaveOutWriteFailed", err).SetMetadata("buffer", r.count)
		}
		r.count++
	}
	if err := dev.restart(); err != nil {
		s.release(r)
		return errors.NewErr("WaveOutRestartFailed", err)
	}
	s.run = r
	s.track.start()
	go s.feed(r)
	return nil
}

// fill takes the next samples of the source into buffer i and queues it. The
// goroutine that feeds the device is the only caller after Start.
func (r *waveRun) fill(i int) error {
	n := r.src.Read(r.scratch)
	clear(r.scratch[n:]) // after the end: silence; the Player stops the sink
	encodePCM16(r.dev.buffer(i), r.scratch)
	return r.dev.write(i, r.bufBytes)
}

// encodePCM16 writes samples (-1 to 1) as little-endian 16-bit PCM into dst,
// which is 2*len(samples) bytes long. A value outside the range is clamped and
// a NaN is silence.
func encodePCM16(dst []byte, samples []float32) {
	for i, v := range samples {
		var q int16
		switch {
		case v != v: // NaN
		case v >= 1:
			q = math.MaxInt16
		case v <= -1:
			q = -math.MaxInt16
		default:
			q = int16(math.Round(float64(v) * math.MaxInt16))
		}
		binary.LittleEndian.PutUint16(dst[2*i:], uint16(q))
	}
}

// feed is the goroutine of a run: it takes finished buffers back and fills them
// again until the run is stopped or the device fails.
func (s *WaveOutSink) feed(r *waveRun) {
	defer close(r.done)
	for {
		r.dev.wait(s.waitSlice)
		if r.stop.Load() {
			return
		}
		for r.count > 0 && r.dev.done(r.head) {
			r.head = (r.head + 1) % s.buffers
			r.count--
		}
		for r.count < s.buffers {
			if err := r.fill((r.head + r.count) % s.buffers); err != nil {
				// The device runs dry and the position goes on by the
				// clock; the Player still ends the clip.
				s.log.Error("the sound device stopped taking samples",
					"err", errors.NewErr("WaveOutWriteFailed", err).SetMetadata("buffer", r.count))
				return
			}
			r.count++
		}
		s.observe(r)
	}
}

// observe asks the device how much it has played and corrects the tracker.
func (s *WaveOutSink) observe(r *waveRun) {
	epoch, running := s.track.token()
	if !running {
		return
	}
	frames, ok := r.dev.position()
	if !ok {
		return
	}
	at := s.clock.Now()
	s.track.observe(epoch, float64(frames)/float64(r.rate), at)
}

// Pause holds the position and pauses the device. A failure is logged and
// makes the next Resume fail with WaveOutPauseFailed (the sink stays paused).
func (s *WaveOutSink) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.run == nil || s.paused {
		return
	}
	s.track.pause()
	if err := s.run.dev.pause(); err != nil {
		s.pauseErr = errors.NewErr("WaveOutPauseFailed", err)
		s.log.Error("the sound device did not pause", "err", s.pauseErr)
	}
	s.paused = true
}

// Resume restarts the device Pause paused; it goes on from where it stopped. It
// fails with WaveOutStartCanceled when ctx is done, with WaveOutPauseFailed
// when the Pause before it failed, and with WaveOutRestartFailed when the
// device does not restart; the sink then stays paused.
func (s *WaveOutSink) Resume(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.paused || s.closed {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return errors.NewErr("WaveOutStartCanceled", err)
	}
	if s.pauseErr != nil {
		return s.pauseErr
	}
	if err := s.run.dev.restart(); err != nil {
		return errors.NewErr("WaveOutRestartFailed", err)
	}
	s.paused = false
	s.track.resume()
	return nil
}

// Stop resets and closes the device; the position goes to 0.
func (s *WaveOutSink) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
}

// Close stops the play and releases the sink.
func (s *WaveOutSink) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.stopLocked()
	s.closed = true
}

// Position returns the played seconds (see the WaveOutSink documentation). It
// never waits for the device.
func (s *WaveOutSink) Position() float64 { return s.track.position() }

// stopLocked ends the play: the goroutine, the device and the position.
func (s *WaveOutSink) stopLocked() {
	if r := s.run; r != nil {
		s.run = nil
		r.stop.Store(true)
		r.dev.wake()
		timer := time.NewTimer(s.joinTimeout)
		select {
		case <-r.done:
			timer.Stop()
			s.release(r)
		case <-timer.C:
			// The goroutine is stuck in the source, which is not ours to
			// fix. Silence the device now and release it when the goroutine
			// comes back: closing it earlier would free memory that the
			// goroutine may still write to.
			s.logReset(r)
			go func() {
				<-r.done
				s.release(r)
			}()
		}
	}
	s.track.stop()
	s.paused = false
	s.pauseErr = nil
}

// release resets and closes the device of a run whose goroutine has ended.
func (s *WaveOutSink) release(r *waveRun) {
	s.logReset(r)
	if err := r.dev.close(); err != nil {
		s.log.Error("the sound device did not close", "err", errors.NewErr("WaveOutCloseFailed", err))
	}
}

// logReset resets the device, which gives the queued buffers back and silences
// it, and logs a failure.
func (s *WaveOutSink) logReset(r *waveRun) {
	if err := r.dev.reset(); err != nil {
		s.log.Error("the sound device did not reset", "err", errors.NewErr("WaveOutResetFailed", err))
	}
}

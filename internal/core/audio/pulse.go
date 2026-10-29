package audio

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jfreymuth/pulse"
	"github.com/jfreymuth/pulse/proto"
	"github.com/rm4n0s/errors"
)

// PulseSink plays through a PulseAudio server over the PulseAudio protocol,
// with github.com/jfreymuth/pulse (pure Go, no cgo). PipeWire serves the same
// protocol (pipewire-pulse), so this is the sink on a current Linux desktop.
//
// How the library works, and what the sink does with it. The library is
// pull-based: it creates a playback stream and calls a reader function
// whenever the server wants more data. The sink hands it the samples of the
// Source and, after the Source has ended, silence, so that the stream stays
// running until the Player stops it (which lets Pause work to the last
// moment and keeps the library out of its end-of-data states).
//
// Pause closes the stream and Resume opens a new one that continues from the
// played position (the Source is asked to SeekFrame there), instead of
// corking and uncorking one stream. The library deadlocks on cork and uncork:
// the server answers an uncork with a "started" event, which the library
// hands to a channel nobody reads, which blocks its connection loop, and
// every later request times out (found against PipeWire's PulseAudio server).
// A new stream costs a few milliseconds.
//
// The library has no played-position call. The sink asks the server for it:
// every pollInterval it sends a GetPlaybackLatency request and computes
//
//	played = read index / (rate * bytes per frame) - latency
//
// where the read index counts the bytes that have left the stream's buffer
// toward the device and latency is the time from there to the speakers
// (equivalently: the samples written, minus the ones still in the buffer,
// minus the device latency). Between two answers the position is
// extrapolated with the Clock. The answers only correct the extrapolation;
// the position never goes backwards (a correction that would move it back
// holds it until time catches up). If the server stops answering, the
// position goes on by the Clock, so a lesson still finishes.
type PulseSink struct {
	backend pulseBackend
	clock   Clock
	track   *tracker

	pollInterval time.Duration
	startTimeout time.Duration

	mu       sync.Mutex // serializes Start, Pause, Resume, Stop and Close
	stream   pulseStream
	stopPoll chan struct{}
	closed   bool
	src      Source // the source of the current play, kept while paused
	format   Format // its format
	paused   bool
}

// PulseOptions configures NewPulseSink. The zero value is fine.
type PulseOptions struct {
	// Server is a PulseAudio server string. Empty means PULSE_SERVER, or
	// $XDG_RUNTIME_DIR/pulse/native when that is not set.
	Server string
	// AppName is the name the server shows for this client (volume
	// controls). Empty means "Radio Tutor".
	AppName string
	// Clock is the time source; nil means SystemClock().
	Clock Clock
}

// connectTimeout is how long NewPulseSink waits for the server to answer.
const connectTimeout = 5 * time.Second

// NewPulseSink connects to the PulseAudio server. It fails, tagged
// PulseNoServer, PulseConnectFailed, PulseConnectTimeout or
// PulseConnectCanceled, when no server can be reached, and the caller may fall
// back to a SilentSink (OpenDefaultSink does). The library reads the
// authentication cookie from $PULSE_COOKIE or ~/.config/pulse/cookie, as
// every PulseAudio client does; a server that allows anonymous clients needs
// none.
//
// The connection itself is not cancellable in the library, so on a cancelled
// ctx or a timeout NewPulseSink returns at once and a goroutine closes the
// connection if it ever completes.
func NewPulseSink(ctx context.Context, opts ...PulseOptions) (*PulseSink, error) {
	return dialPulse(ctx, connectTimeout, opts...)
}

func dialPulse(ctx context.Context, timeout time.Duration, opts ...PulseOptions) (*PulseSink, error) {
	var o PulseOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	if o.Clock == nil {
		o.Clock = SystemClock()
	}
	if o.AppName == "" {
		o.AppName = "Radio Tutor"
	}
	server := o.Server
	if server == "" {
		xdg, ok := defaultServer(os.Getenv)
		if !ok {
			return nil, errors.New("PulseNoServer", "neither PULSE_SERVER nor XDG_RUNTIME_DIR is set")
		}
		server = xdg
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.NewErr("PulseConnectCanceled", err).SetMetadata("server", server)
	}

	type result struct {
		c   *pulse.Client
		err error
	}
	done := make(chan result, 1)
	go func() {
		c, err := pulse.NewClient(pulse.ClientApplicationName(o.AppName), pulse.ClientServerString(server), pulse.ClientTimeout(2*time.Second))
		done <- result{c, err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	abandon := func() {
		go func() {
			if r := <-done; r.c != nil {
				r.c.Close()
			}
		}()
	}
	select {
	case r := <-done:
		if r.err != nil {
			return nil, errors.NewErr("PulseConnectFailed", r.err).SetMetadata("server", server)
		}
		return newPulseSink(&libBackend{client: r.c, name: o.AppName}, o.Clock), nil
	case <-ctx.Done():
		abandon()
		return nil, errors.NewErr("PulseConnectCanceled", ctx.Err()).SetMetadata("server", server)
	case <-timer.C:
		abandon()
		return nil, errors.New("PulseConnectTimeout", "the PulseAudio server did not answer", "server", server)
	}
}

func newPulseSink(b pulseBackend, c Clock) *PulseSink {
	return &PulseSink{
		backend:      b,
		clock:        c,
		track:        newTracker(c),
		pollInterval: 100 * time.Millisecond,
		startTimeout: 3 * time.Second,
	}
}

// defaultServer returns the server string to use when none is given:
// PULSE_SERVER if set and not empty, else the socket in XDG_RUNTIME_DIR. env
// reads an environment variable.
func defaultServer(env func(string) string) (string, bool) {
	if s := env("PULSE_SERVER"); s != "" {
		return s, true
	}
	if dir := env("XDG_RUNTIME_DIR"); dir != "" {
		return "unix:" + filepath.Join(dir, "pulse", "native"), true
	}
	return "", false
}

// Name returns "pulse".
func (s *PulseSink) Name() string { return "pulse" }

// Start opens a playback stream for src and starts it; any earlier stream is
// closed first. It fails, tagged PulseClosed, PulseFormatUnsupported,
// PulseStreamFailed, PulseStartTimeout or PulseStartCanceled.
func (s *PulseSink) Start(ctx context.Context, src Source, f Format) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("PulseClosed", "the sink is closed")
	}
	if f.SampleRate <= 0 || (f.Channels != 1 && f.Channels != 2) {
		return errors.New("PulseFormatUnsupported", "the sink plays mono or stereo at a positive sample rate",
			"sample_rate", f.SampleRate, "channels", f.Channels)
	}
	if err := ctx.Err(); err != nil {
		return errors.NewErr("PulseStartCanceled", err)
	}
	s.stopLocked()

	stream, err := s.openStream(ctx, src, f)
	if err != nil {
		return err
	}
	s.src, s.format = src, f
	s.stream = stream
	s.track.start()
	s.beginPolling(stream, 0)
	return nil
}

// openStream creates a stream that pulls from src and waits for it to run. The
// caller keeps s.mu.
func (s *PulseSink) openStream(ctx context.Context, src Source, f Format) (pulseStream, error) {
	read := func(buf []float32) (int, error) {
		n := src.Read(buf)
		clear(buf[n:]) // after the end: silence, the Player stops the stream
		return len(buf), nil
	}
	stream, err := s.backend.open(read, f)
	if err != nil {
		return nil, errors.NewErr("PulseStreamFailed", err)
	}

	// The library's Start waits for the server's "started" event, which may
	// never come; do not let that hold the caller.
	started := make(chan struct{})
	go func() {
		stream.Start()
		close(started)
	}()
	timer := time.NewTimer(s.startTimeout)
	defer timer.Stop()
	select {
	case <-started:
		return stream, nil
	case <-ctx.Done():
		stream.Close()
		return nil, errors.NewErr("PulseStartCanceled", ctx.Err())
	case <-timer.C:
		stream.Close()
		return nil, errors.New("PulseStartTimeout", "the PulseAudio stream did not start", "timeout", s.startTimeout.String())
	}
}

// beginPolling takes the first measurement and starts the poll loop. The
// caller keeps s.mu.
//
// offset is the seconds of the source before the stream's first sample (the
// server counts from the stream's own start).
func (s *PulseSink) beginPolling(stream pulseStream, offset float64) {
	s.observe(stream, offset)
	s.stopPoll = make(chan struct{})
	go s.pollLoop(stream, offset, s.stopPoll)
}

// Pause holds the position and closes the stream (see the PulseSink
// documentation for why it is not corked).
func (s *PulseSink) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stream == nil {
		return
	}
	s.track.pause()
	s.closeStreamLocked()
	s.paused = true
}

// Resume opens a new stream that continues at the position Pause held. It
// fails like Start does (PulseStreamFailed, PulseStartTimeout,
// PulseStartCanceled) and then stays paused.
func (s *PulseSink) Resume(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.paused || s.closed {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return errors.NewErr("PulseStartCanceled", err)
	}
	held := s.track.position()
	frame := int64(math.Round(held * float64(s.format.SampleRate)))
	s.src.SeekFrame(frame)
	stream, err := s.openStream(ctx, s.src, s.format)
	if err != nil {
		return err
	}
	s.paused = false
	s.stream = stream
	s.track.resume()
	s.beginPolling(stream, float64(frame)/float64(s.format.SampleRate))
	return nil
}

// Stop closes the stream; the position goes to 0.
func (s *PulseSink) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
}

// Close closes the stream and the connection.
func (s *PulseSink) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.stopLocked()
	s.closed = true
	s.backend.close()
}

// Position returns the played seconds (see the PulseSink documentation). It
// does not wait for the server.
func (s *PulseSink) Position() float64 { return s.track.position() }

// stopLocked ends the play: the stream, the source and the position.
func (s *PulseSink) stopLocked() {
	s.closeStreamLocked()
	s.track.stop()
	s.src, s.paused = nil, false
}

// closeStreamLocked stops polling and closes the stream, if there is one.
func (s *PulseSink) closeStreamLocked() {
	if s.stopPoll != nil {
		close(s.stopPoll)
		s.stopPoll = nil
	}
	if s.stream != nil {
		s.stream.Close()
		s.stream = nil
	}
}

func (s *PulseSink) pollLoop(stream pulseStream, offset float64, stop <-chan struct{}) {
	t := time.NewTicker(s.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			s.observe(stream, offset)
		}
	}
}

// observe asks the server how much has been played and corrects the tracker.
func (s *PulseSink) observe(stream pulseStream, offset float64) {
	epoch, running := s.track.token()
	if !running {
		return
	}
	pos, at, ok := stream.played(s.clock)
	if !ok {
		return
	}
	s.track.observe(epoch, pos+offset, at)
}

// pulseBackend is the part of the library the sink uses, so that the sink's
// logic is tested without a server.
type pulseBackend interface {
	// open creates a stopped playback stream that pulls its data from read.
	open(read func([]float32) (int, error), f Format) (pulseStream, error)
	close()
}

// pulseStream is one playback stream.
type pulseStream interface {
	// Start blocks until the stream runs (or the library gives up waiting).
	Start()
	// Close ends the stream. It is safe to call it once.
	Close()
	// played asks the server for the played position in seconds, and the
	// clock reading it applies to. ok is false when there is no answer.
	played(c Clock) (pos float64, at time.Time, ok bool)
}

// libBackend is the pulseBackend over the library.
type libBackend struct {
	client *pulse.Client
	name   string
}

func (b *libBackend) open(read func([]float32) (int, error), f Format) (pulseStream, error) {
	channels := pulse.PlaybackMono
	if f.Channels == 2 {
		channels = pulse.PlaybackStereo
	}
	// The options that depend on others come last: the latency is computed
	// from the rate and the channels.
	s, err := b.client.NewPlayback(pulse.Float32Reader(read),
		pulse.PlaybackSampleRate(f.SampleRate), channels,
		pulse.PlaybackLatency(0.1), pulse.PlaybackMediaName(b.name))
	if err != nil {
		return nil, err
	}
	return &libStream{client: b.client, s: s, rate: f.SampleRate, bytesPerFrame: 4 * f.Channels}, nil
}

func (b *libBackend) close() { b.client.Close() }

type libStream struct {
	client        *pulse.Client
	s             *pulse.PlaybackStream
	rate          int
	bytesPerFrame int
}

func (l *libStream) Start() { l.s.Start() }

// Close corks the stream first, so that the server does not ask for data
// while the stream is being deleted (the library does not expect that).
func (l *libStream) Close() {
	l.s.Pause()
	l.s.Stop()
	l.s.Close()
}

func (l *libStream) played(c Clock) (float64, time.Time, bool) {
	t0 := c.Now()
	var rep proto.GetPlaybackLatencyReply
	err := l.client.RawRequest(&proto.GetPlaybackLatency{StreamIndex: l.s.StreamIndex()}, &rep)
	t1 := c.Now()
	if err != nil || !rep.Running {
		return 0, time.Time{}, false
	}
	read := float64(rep.ReadIndex) / float64(l.bytesPerFrame) / float64(l.rate)
	pos := max(read-float64(rep.Latency)/1e6, 0)
	return pos, t0.Add(t1.Sub(t0) / 2), true
}

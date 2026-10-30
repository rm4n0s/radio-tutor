// Copied from github.com/jfreymuth/pulse v0.1.3 and changed for Radio Tutor. MIT License, Copyright (c) 2019 Johann Freymuth: see internal/core/pulse/LICENSE.

package pulse

import (
	"sync"
	"sync/atomic"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/pulse/proto"
)

// A PlaybackStream is used for playing audio.
// When creating a stream, the user must provide a callback that will be used to buffer audio data.
type PlaybackStream struct {
	c *Client

	index     uint32
	state     *stateMachine
	underflow atomic.Bool

	// The connection's read loop must never wait for a stream (see
	// Client.handle), so what it hands over is kept here: a count of bytes the
	// server asked for, and channels with room for one signal that are written
	// with a non-blocking send.
	mu        sync.Mutex // protects err and requested
	err       error
	requested int           // bytes the server asked for and the stream goroutine has not written yet
	wake      chan struct{} // requested grew, the state changed, or the stream ended
	started   chan struct{} // the server said "started"; may hold a stale signal, Start drains it
	done      chan struct{} // closed when the stream ends: closed, or the server was lost
	doneOnce  sync.Once
	closeOnce sync.Once

	events        chan struct{}
	eventsLock    sync.Mutex
	volumeChanges chan proto.ChannelVolumes

	r Reader

	createRequest  proto.CreatePlaybackStream
	createReply    proto.CreatePlaybackStreamReply
	bytesPerSample int
}

// EndOfData is a special error value that can be returned by a reader to stop the stream.
var EndOfData error = &errors.Error{Tag: "PulseEndOfData", Message: "end of data"}

// NewPlayback creates a playback stream.
// The created stream wil not be running, it must be started with Start().
// If the reader returns any error, the stream will be stopped. The special error value EndOfData
// can be used to intentionally stop the stream from within the callback.
// The order of options is important in some cases, see the documentation of the individual PlaybackOptions.
func (c *Client) NewPlayback(r Reader, opts ...PlaybackOption) (*PlaybackStream, error) {
	p := &PlaybackStream{
		c: c,
		createRequest: proto.CreatePlaybackStream{
			SinkIndex:             proto.Undefined,
			ChannelMap:            proto.ChannelMap{proto.ChannelMono},
			SampleSpec:            proto.SampleSpec{Format: r.Format(), Channels: 1, Rate: 44100},
			BufferMaxLength:       proto.Undefined,
			Corked:                true,
			BufferTargetLength:    proto.Undefined,
			BufferPrebufferLength: proto.Undefined,
			BufferMinimumRequest:  proto.Undefined,
			Properties:            proto.PropList{},
		},
		bytesPerSample: bytes(r.Format()),
		r:              r,
	}

	for _, opt := range opts {
		opt(p)
	}

	if p.createRequest.ChannelVolumes == nil {
		cvol := make(proto.ChannelVolumes, len(p.createRequest.ChannelMap))
		for i := range cvol {
			cvol[i] = 0x100
		}
		p.createRequest.ChannelVolumes = cvol
	}

	// Listen for changes in the sink input if the application wants to be
	// notified of volume changes.
	if p.volumeChanges != nil {
		p.events = make(chan struct{}, 1)
		go p.handleEvents(p.events)
	}

	err := c.c.Request(&p.createRequest, &p.createReply)
	if err != nil {
		return nil, err
	}
	p.index = p.createReply.StreamIndex
	p.state = newStateMachine()
	p.wake = make(chan struct{}, 1)
	p.started = make(chan struct{}, 1)
	p.done = make(chan struct{})
	c.mu.Lock()
	c.playback[p.index] = p
	c.mu.Unlock()
	go p.run()
	return p, nil
}

// addRequested records that the server asked for n more bytes and wakes the
// stream's goroutine. It never blocks: the connection's read loop calls it.
func (p *PlaybackStream) addRequested(n int) {
	p.mu.Lock()
	p.requested += n
	p.mu.Unlock()
	p.signalWake()
}

// signalWake wakes the stream's goroutine, if it is not awake already.
func (p *PlaybackStream) signalWake() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// signalStarted records the server's "started" event. It never blocks: nobody
// may be waiting (the server also sends it after Resume and after an
// underflow), and then the signal is dropped or stays in the channel until
// Start drains it.
func (p *PlaybackStream) signalStarted() {
	select {
	case p.started <- struct{}{}:
	default:
	}
}

// finish tells the stream's goroutine and any Start that waits that the stream
// has ended.
func (p *PlaybackStream) finish() {
	p.doneOnce.Do(func() { close(p.done) })
}

// setErr remembers err as the stream's error.
func (p *PlaybackStream) setErr(err error) {
	p.mu.Lock()
	p.err = err
	p.mu.Unlock()
}

// call sends a request that has no reply data; a failure becomes the stream's
// error (the methods of the stream do not return one).
func (p *PlaybackStream) call(req proto.RequestArgs) bool {
	if err := p.c.c.Request(req, nil); err != nil {
		p.setErr(err)
		return false
	}
	return true
}

// resetRequested forgets the bytes the server asked for.
func (p *PlaybackStream) resetRequested() {
	p.mu.Lock()
	p.requested = 0
	p.mu.Unlock()
}

// pendingRequested returns the bytes the server asked for and that are not
// written yet.
func (p *PlaybackStream) pendingRequested() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.requested
}

// consumeRequested subtracts n written bytes from the pending request.
func (p *PlaybackStream) consumeRequested(n int) {
	p.mu.Lock()
	p.requested -= n
	p.mu.Unlock()
}

func (p *PlaybackStream) run() {
	front := make([]byte, p.createReply.BufferMaxLength)
	back := make([]byte, p.createReply.BufferMaxLength)

	for {
		select {
		case <-p.wake:
		case <-p.done:
			return
		}
		switch {
		case p.state.is(running):
		case p.state.is(paused):
			// Corked: the server's request stays pending, and Resume wakes this
			// goroutine to fulfill it (the server does not ask again for bytes
			// it has already asked for).
			continue
		default:
			p.resetRequested()
			continue
		}
		for p.state.is(running) {
			requested := p.pendingRequested()
			if requested <= 0 {
				break
			}
			front, back = ensurePlaybackBuffers(front, back, requested)
			readCount, err := p.r.Read(front[:requested])
			if err != nil {
				if err != EndOfData {
					p.setErr(err)
				}
				p.state.set(idle)
				p.resetRequested()
				break
			}
			if readCount > 0 {
				if err := p.c.c.Send(p.index, front[:readCount]); err != nil {
					p.setErr(err)
					p.state.set(idle)
					p.resetRequested()
					break
				}
				p.consumeRequested(readCount)
				front, back = back, front
			}
		}
	}
}

func ensurePlaybackBuffers(front, back []byte, requested int) ([]byte, []byte) {
	if requested <= cap(front) && requested <= cap(back) {
		return front, back
	}
	return make([]byte, requested), make([]byte, requested)
}

// Handle events for this playback stream in a goroutine.
// Event notifications are received through the events channel.
func (p *PlaybackStream) handleEvents(events chan struct{}) {
	volume := make(proto.ChannelVolumes, len(p.createRequest.ChannelMap))

	for range events {
		// We got an event that something about our sink input changed, so read
		// the sink input information.
		reply := proto.GetSinkInputInfoReply{}
		err := p.c.c.Request(&proto.GetSinkInputInfo{
			SinkInputIndex: p.createReply.SinkInputIndex,
		}, &reply)
		if err != nil {
			if p.Closed() {
				// Most likely this error is caused by the stream getting
				// closed. So exit the goroutine.
				break
			}
			// This should not normally happen (a request timed out); the next
			// event asks again.
			continue
		}

		// Check whether the volume changed, and if so, report it to the
		// application.
		volumeChanged := false
		for i, val := range reply.ChannelVolumes {
			if volume[i] != val {
				volume[i] = val
				volumeChanged = true
			}
		}
		if volumeChanged {
			volumeToSend := append(proto.ChannelVolumes(nil), volume...) // copy volume

			// Drop last volume change, if not received by the application.
			// This way, if p.volumeChanges is a buffered channel, some updates
			// might get lost when the receiver is slow but it will always
			// receive the latest volume eventually.
			select {
			case <-p.volumeChanges:
				// Dropped, so there was something in the buffered channel.
			default:
				// Not dropped, so if the channel is buffered, it should have
				// room now.
			}

			// Send the new volume.
			select {
			case p.volumeChanges <- volumeToSend:
				// Succeeded in sending!
			default:
				// Somehow couldn't send the new volume value. Perhaps the
				// channel is unbuffered, and the receiving goroutine is doing
				// other things? There's not much we can do about it here.
			}
		}
	}

	// Playback stream was closed, so close the volume changes channel.
	close(p.volumeChanges)
}

// Start starts playing audio. It returns when the server reports that the
// stream started, or when the stream ends (Close, or the connection is lost).
func (p *PlaybackStream) Start() {
	if p.state.is(idle) {
		p.call(&proto.FlushPlaybackStream{StreamIndex: p.index})
		// A "started" event of an earlier run is not the answer to this one.
		select {
		case <-p.started:
		default:
		}
		p.resetRequested()
		p.state.set(running)
		p.setErr(nil)
		p.addRequested(int(p.createReply.BufferTargetLength))
		p.underflow.Store(false)
		if !p.call(&proto.CorkPlaybackStream{StreamIndex: p.index, Corked: false}) {
			return
		}
		select {
		case <-p.started:
		case <-p.done:
		}
	}
}

// Stop stops playing audio; the callback will no longer be called.
// If the buffer size/latency is large, audio may continue to play for some time after the call to Stop.
func (p *PlaybackStream) Stop() {
	if p.state.is(running, paused) {
		p.state.set(idle)
		p.signalWake()
	}
}

// Pause stops playing audio immediately: the server corks the stream, and the
// callback is no longer called until Resume.
func (p *PlaybackStream) Pause() {
	if p.state.is(running) {
		p.call(&proto.CorkPlaybackStream{StreamIndex: p.index, Corked: true})
		p.state.set(paused)
	}
}

// Resume resumes a paused stream. It does not wait for the server's "started"
// event.
func (p *PlaybackStream) Resume() {
	if p.state.is(paused) {
		p.call(&proto.CorkPlaybackStream{StreamIndex: p.index, Corked: false})
		p.state.set(running)
		p.underflow.Store(false)
		p.signalWake()
	}
}

// Drain waits until the playback has ended.
// Drain does not return when the stream is paused.
func (p *PlaybackStream) Drain() {
	if p.state.is(running) {
		p.call(&proto.DrainPlaybackStream{StreamIndex: p.index})
	}
}

// Volume returns the volume of each channel in the playback.
func (p *PlaybackStream) Volume() (proto.ChannelVolumes, error) {
	reply := proto.GetSinkInputInfoReply{}
	err := p.c.c.Request(&proto.GetSinkInputInfo{
		SinkInputIndex: p.createReply.SinkInputIndex,
	}, &reply)
	if err != nil {
		return nil, err
	}
	return reply.ChannelVolumes, nil
}

// SetVolume changes the volume of each channel in the playback.
//
// Do not set the volume when opening a playback stream, PulseAudio will pick an
// appropriate volume for the stream automatically (and may save it for next
// time). In particular, don't set it to 100% because depending on the
// configuration this could actually set the volume to the maximum volume the
// hardware is capable of which is usually way too loud. Instead, only change
// the volume as a direct result of user input.
//
// If you use this API, you should also query the volume on startup and listen
// for playback volume changes from the system mixer (many desktops allow users
// to change application volume from the system tray). That way, you can keep
// the volume slider in the application synchronized with the system volume
// mixer.
func (p *PlaybackStream) SetVolume(volumes proto.ChannelVolumes) error {
	return p.c.c.Request(&proto.SetSinkInputVolume{
		SinkInputIndex: p.createReply.SinkInputIndex,
		ChannelVolumes: volumes,
	}, nil)
}

// Close closes the stream.
func (p *PlaybackStream) Close() {
	if p.Closed() {
		return
	}
	p.closeOnce.Do(func() {
		p.call(&proto.DeletePlaybackStream{StreamIndex: p.index})
		p.state.set(closed)
		p.finish()

		p.c.mu.Lock()
		delete(p.c.playback, p.index)
		p.c.mu.Unlock()

		p.eventsLock.Lock()
		if p.events != nil {
			close(p.events)
			p.events = nil
		}
		p.eventsLock.Unlock()
	})
}

// Closed returns wether the stream was closed.
func (p *PlaybackStream) Closed() bool { return p.state.is(closed, serverLost) }

// Running returns wether the stream is currently playing.
func (p *PlaybackStream) Running() bool { return p.state.is(running) }

// Underflow returns true if any underflows happened since the last call to Start or Resume.
// Underflows usually happen because the latency/buffer size is too low or because the callback
// takes too long to run.
func (p *PlaybackStream) Underflow() bool { return p.underflow.Load() }

// Error returns the last error returned by the stream's reader, or the last
// failure of a request the stream made (the stream's methods do not return one).
func (p *PlaybackStream) Error() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

// SampleRate returns the stream's sample rate (samples per second).
func (p *PlaybackStream) SampleRate() int {
	return int(p.createReply.Rate)
}

// Channels returns the number of channels.
func (p *PlaybackStream) Channels() int {
	return int(p.createReply.Channels)
}

// BufferSize returns the size of the server-side buffer in samples.
func (p *PlaybackStream) BufferSize() int {
	s := int(p.createReply.BufferTargetLength) / int(p.createReply.Channels)
	return s / p.bytesPerSample
}

// BufferSizeBytes returns the size of the server-side buffer in bytes.
func (p *PlaybackStream) BufferSizeBytes() int {
	return int(p.createReply.BufferTargetLength)
}

// StreamIndex returns the stream index.
// This should only be used together with (*Cient).RawRequest.
func (p *PlaybackStream) StreamIndex() uint32 {
	return p.index
}

// StreamInputIndex returns the index of the stream's sink input.
// This should only be used together with (*Cient).RawRequest.
func (p *PlaybackStream) StreamInputIndex() uint32 {
	return p.createReply.SinkInputIndex
}

// A PlaybackOption supplies configuration when creating streams.
type PlaybackOption func(*PlaybackStream)

// PlaybackMono sets a stream to a single channel.
var PlaybackMono PlaybackOption = func(p *PlaybackStream) {
	p.createRequest.ChannelMap = proto.ChannelMap{proto.ChannelMono}
	p.createRequest.Channels = 1
}

// PlaybackStereo sets a stream to two channels.
var PlaybackStereo PlaybackOption = func(p *PlaybackStream) {
	p.createRequest.ChannelMap = proto.ChannelMap{proto.ChannelLeft, proto.ChannelRight}
	p.createRequest.Channels = 2
}

// PlaybackChannels sets a stream to use a custom channel map.
func PlaybackChannels(m proto.ChannelMap) PlaybackOption {
	if len(m) == 0 {
		panic("pulse: invalid channel map")
	}
	return func(p *PlaybackStream) {
		p.createRequest.ChannelMap = m
		p.createRequest.Channels = byte(len(m))
	}
}

// PlaybackSampleRate sets the stream's sample rate.
func PlaybackSampleRate(rate int) PlaybackOption {
	return func(p *PlaybackStream) {
		p.createRequest.Rate = uint32(rate)
	}
}

// PlaybackBufferSize sets the size of the server-side buffer.
// Setting the buffer size too small causes underflows, resulting in audible artifacts.
//
// Buffer size and latency should not be set at the same time.
func PlaybackBufferSize(samples int) PlaybackOption {
	return func(p *PlaybackStream) {
		p.createRequest.BufferTargetLength = uint32(samples * p.bytesPerSample)
		p.createRequest.AdjustLatency = false
	}
}

// PlaybackLatency sets the stream's latency in seconds.
// Setting the latency too low causes underflows, resulting in audible artifacts.
// Applications should generally use the highest acceptable latency.
//
// This should be set after sample rate and channel options.
//
// Buffer size and latency should not be set at the same time.
func PlaybackLatency(seconds float64) PlaybackOption {
	return func(p *PlaybackStream) {
		p.createRequest.BufferTargetLength = uint32(seconds*float64(p.createRequest.Rate)) * uint32(p.createRequest.Channels) * uint32(p.bytesPerSample)
		p.createRequest.BufferMaxLength = 2 * p.createRequest.BufferTargetLength
		p.createRequest.AdjustLatency = true
	}
}

// PlaybackSink sets the sink the stream should send audio to.
func PlaybackSink(sink *Sink) PlaybackOption {
	return func(p *PlaybackStream) {
		p.createRequest.SinkIndex = sink.info.SinkIndex
	}
}

// PlaybackMediaName sets the streams media name.
// This will e.g. be displayed by a volume control application to identity the stream.
func PlaybackMediaName(name string) PlaybackOption {
	return func(p *PlaybackStream) {
		p.createRequest.Properties["media.name"] = proto.PropListString(name)
	}
}

// PlaybackMediaIconName sets the streams media icon using an xdg icon name.
// This will e.g. be displayed by a volume control application to identity the stream.
func PlaybackMediaIconName(name string) PlaybackOption {
	return func(p *PlaybackStream) {
		p.createRequest.Properties["media.icon_name"] = proto.PropListString(name)
	}
}

// PlaybackVolumeChanges sets a channel to receive volume changes on.
// These changes can come either from changing the volume directly (through
// SetVolume) or from the system volume mixer.
//
// The channel should be buffered (1 element is sufficient) to avoid losing
// volume changes due to race conditions or a slow receiver. It will be closed
// when the playback is closed.
func PlaybackVolumeChanges(changes chan proto.ChannelVolumes) PlaybackOption {
	return func(p *PlaybackStream) {
		p.volumeChanges = changes
	}
}

// PlaybackRawOption can be used to create custom options.
//
// This is an advanced function, similar to (*Client).RawRequest.
func PlaybackRawOption(o func(*proto.CreatePlaybackStream)) PlaybackOption {
	return func(p *PlaybackStream) {
		o(&p.createRequest)
	}
}

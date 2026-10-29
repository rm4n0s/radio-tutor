package audio

import "context"

// OpenDefaultSink returns the sink the app plays through: a PulseSink when a
// PulseAudio (or PipeWire) server answers at PULSE_SERVER or
// $XDG_RUNTIME_DIR/pulse/native, and otherwise a SilentSink. The sink is
// never nil. When reason is not nil the SilentSink is a fallback, and reason
// says why the server could not be used (an error tagged PulseNoServer,
// PulseConnectFailed, PulseConnectTimeout or PulseConnectCanceled): the UI
// shows "No sound device found. The lesson plays without sound." once per
// run (SPEC §8.1) and logs reason. The caller closes the sink.
func OpenDefaultSink(ctx context.Context) (sink Sink, reason error) {
	ps, err := NewPulseSink(ctx)
	if err != nil {
		return NewSilentSink(nil), err
	}
	return ps, nil
}

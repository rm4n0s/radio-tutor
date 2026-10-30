package audio

import "context"

// OpenDefaultSink returns the sink the app plays through, and never nil. On
// Linux it is a PulseSink when a PulseAudio (or PipeWire) server answers at
// PULSE_SERVER or $XDG_RUNTIME_DIR/pulse/native, and otherwise a SilentSink.
// On Windows it is a WaveOutSink on the default output device of winmm, and
// otherwise a SilentSink. When reason is not nil the SilentSink is a
// fallback, and reason says why the device could not be used (on Linux an
// error tagged PulseNoServer, PulseConnectTimeout or PulseConnectCanceled, or
// the tagged error of the pulse package, such as PulseDialFailed; on Windows
// WaveOutNoDevice, WaveOutLoadFailed or WaveOutStartCanceled): the UI shows
// "No sound device found. The lesson plays without sound." once per run (SPEC
// §8.1) and logs reason. The caller closes the sink.
func OpenDefaultSink(ctx context.Context) (sink Sink, reason error) {
	s, err := openPlatformSink(ctx)
	if err != nil {
		return NewSilentSink(nil), err
	}
	return s, nil
}

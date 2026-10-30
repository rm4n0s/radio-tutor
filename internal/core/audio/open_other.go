//go:build !windows

package audio

import "context"

// openPlatformSink opens the PulseAudio sink (PipeWire serves the same
// protocol).
func openPlatformSink(ctx context.Context) (Sink, error) {
	ps, err := NewPulseSink(ctx)
	if err != nil {
		return nil, err
	}
	return ps, nil
}

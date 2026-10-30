package audio

import "context"

// openPlatformSink opens the waveOut sink of winmm.
func openPlatformSink(ctx context.Context) (Sink, error) {
	ws, err := NewWaveOutSink(ctx)
	if err != nil {
		return nil, err
	}
	return ws, nil
}

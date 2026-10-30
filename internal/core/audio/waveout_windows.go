package audio

import (
	"context"

	"github.com/rm4n0s/errors"
)

// NewWaveOutSink opens the sink over the winmm.dll of the machine. It fails,
// tagged WaveOutLoadFailed, when winmm.dll cannot be loaded, and WaveOutNoDevice
// when the machine has no sound output device (OpenDefaultSink falls back to
// a SilentSink). A device that is busy or refuses the format shows only at Start.
func NewWaveOutSink(ctx context.Context, opts ...WaveOutOptions) (*WaveOutSink, error) {
	var o WaveOutOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	procs, err := loadWinmm("winmm.dll")
	if err != nil {
		return nil, err
	}
	b := &winmmBackend{p: procs}
	if err := ctx.Err(); err != nil {
		return nil, errors.NewErr("WaveOutStartCanceled", err)
	}
	if b.deviceCount() <= 0 {
		return nil, errors.New("WaveOutNoDevice", "there is no sound output device")
	}
	return newWaveOutSink(b, o), nil
}

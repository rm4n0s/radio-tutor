package audio

import (
	"github.com/rm4n0s/radio-tutor/internal/core/problem"
)

// Limits of CASSETTE-FORMAT §6 (and §7.2 for prompt audio).
const (
	// MaxNarrationSeconds is the longest narration: 10 minutes.
	MaxNarrationSeconds = 10 * 60
	// MaxPromptSeconds is the longest prompt audio: 1 minute.
	MaxPromptSeconds = 60
	// MaxBytes is the largest audio file: 20 MiB.
	MaxBytes = 20 << 20
)

// CheckNarration checks the audio rules of CASSETTE-FORMAT §6 on a probed
// file: mono or stereo; 22050, 44100 or 48000 Hz; not empty; at most
// maxSeconds long (MaxNarrationSeconds for narration.ogg, MaxPromptSeconds for
// prompt.ogg); at most MaxBytes (checked only when info.Bytes is known). It
// returns every problem it finds. The paths are empty: the caller knows which
// file the info is from and applies problem.InFile.
func CheckNarration(info Info, maxSeconds float64) []problem.Problem {
	var ps problem.List
	switch info.SampleRate {
	case 22050, 44100, 48000:
	default:
		ps.Errorf("", "the sample rate is %d Hz; it must be 22050, 44100 or 48000 Hz", info.SampleRate)
	}
	switch info.Channels {
	case 1, 2:
	default:
		ps.Errorf("", "the audio has %d channels; it must be mono or stereo", info.Channels)
	}
	if info.Frames == 0 {
		ps.Errorf("", "the audio is empty")
	}
	if s := info.Seconds(); s > maxSeconds {
		ps.Errorf("", "the audio is %.1f seconds long; the limit is %.0f seconds", s, maxSeconds)
	}
	if info.Bytes > MaxBytes {
		ps.Errorf("", "the file is %d bytes; the limit is %d bytes (20 MiB)", info.Bytes, MaxBytes)
	}
	return ps
}

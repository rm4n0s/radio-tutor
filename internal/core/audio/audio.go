// Package audio decodes the Ogg Vorbis audio of a cassette (narration and
// prompt audio) and checks it against the rules of CASSETTE-FORMAT §6.
//
// Decode turns a whole file into a Clip: 32-bit float samples, interleaved by
// channel, which the lesson player later writes to a sink (SPEC §8.1). Probe
// reads the same file but keeps no samples: it reports the sample rate, the
// channel count and the exact length, for the validator. CheckNarration turns
// an Info into problems.
//
// Both functions stream through the decoder and check the context between
// blocks, so a long file can be abandoned. They take any io.Reader; the file
// need not be seekable. The length is counted while decoding instead of read
// from the Ogg container (which needs a seekable file and trusts the last
// page's header), so it is the number of samples the player will really play.
//
// The rest of the package plays a Clip (SPEC §8): a Player (Load, Play, Pause,
// Stop, Position, Ended) drives a Sink. The sinks are PulseSink (real sound
// over the PulseAudio protocol, which PipeWire serves too), SilentSink (a wall
// clock, no sound) and FakeSink (tests); OpenDefaultSink picks the first and
// falls back to the second. A Clock (SystemClock, or a FakeClock in tests)
// stands for time, so no test waits for real time and none makes a sound.
// See Player and PulseSink for the goroutine rules.
package audio

import (
	"context"
	"io"
	"math"
	"time"

	"github.com/jfreymuth/oggvorbis"
	"github.com/rm4n0s/errors"
)

// blockFrames is the number of frames requested from the decoder at a time.
const blockFrames = 4096

// Clip is decoded audio held in memory.
type Clip struct {
	// SampleRate is in Hz.
	SampleRate int
	// Channels is 1 (mono) or 2 (stereo) for audio that passes CheckNarration.
	Channels int
	// Samples are in the range -1 to 1, interleaved: frame i of channel c is
	// Samples[i*Channels+c].
	Samples []float32
}

// Frames returns the number of sample frames (samples per channel).
func (c *Clip) Frames() int {
	if c.Channels == 0 {
		return 0
	}
	return len(c.Samples) / c.Channels
}

// Seconds returns the length of the clip in seconds.
func (c *Clip) Seconds() float64 { return seconds(int64(c.Frames()), c.SampleRate) }

// Duration returns the length of the clip.
func (c *Clip) Duration() time.Duration { return duration(int64(c.Frames()), c.SampleRate) }

// Info describes an audio file without its samples.
type Info struct {
	// SampleRate is in Hz.
	SampleRate int
	// Channels is the channel count.
	Channels int
	// Frames is the number of sample frames in the file.
	Frames int64
	// Bytes is the size of the file as read from the reader (0 when unknown).
	Bytes int64
}

// Seconds returns the length of the audio in seconds.
func (i Info) Seconds() float64 { return seconds(i.Frames, i.SampleRate) }

// Duration returns the length of the audio.
func (i Info) Duration() time.Duration { return duration(i.Frames, i.SampleRate) }

func seconds(frames int64, rate int) float64 {
	if rate <= 0 {
		return 0
	}
	return float64(frames) / float64(rate)
}

func duration(frames int64, rate int) time.Duration {
	return time.Duration(math.Round(seconds(frames, rate) * float64(time.Second)))
}

// Decode reads a whole Ogg Vorbis file into memory. It stops with an error
// tagged AudioCanceled when ctx is cancelled. Use DecodeLimit when the file
// comes from an untrusted place and has not been probed yet.
func Decode(ctx context.Context, r io.Reader) (*Clip, error) {
	return DecodeLimit(ctx, r, 0)
}

// DecodeLimit is Decode, but fails with an error tagged AudioTooLong as soon
// as the audio is longer than maxSeconds, before it has been decoded to the
// end. maxSeconds <= 0 means no limit.
func DecodeLimit(ctx context.Context, r io.Reader, maxSeconds float64) (*Clip, error) {
	clip := &Clip{}
	_, err := stream(ctx, r, maxSeconds, func(rate, channels int, block []float32) {
		clip.SampleRate, clip.Channels = rate, channels
		clip.Samples = append(clip.Samples, block...)
	})
	if err != nil {
		return nil, err
	}
	return clip, nil
}

// Probe decodes a whole Ogg Vorbis file to find its sample rate, channels and
// exact length, and the size of the file, keeping no samples. The memory it
// uses does not depend on the length of the audio. A file that Decode would
// refuse is refused with the same errors.
func Probe(ctx context.Context, r io.Reader) (Info, error) {
	cr := &countingReader{r: r}
	info, err := stream(ctx, cr, 0, func(int, int, []float32) {})
	if err != nil {
		return Info{}, err
	}
	// Count the bytes after the last page too, so that Bytes is the size of
	// the whole file.
	buf := make([]byte, 32*1024)
	for {
		if cerr := ctx.Err(); cerr != nil {
			return Info{}, errors.NewErr("AudioCanceled", cerr)
		}
		_, rerr := cr.Read(buf)
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return Info{}, errors.NewErr("AudioReadFailed", rerr)
		}
	}
	info.Bytes = cr.n
	return info, nil
}

// stream decodes r block by block and hands every block (interleaved samples,
// valid only during the call) to sink. maxSeconds <= 0 means no limit.
func stream(ctx context.Context, r io.Reader, maxSeconds float64, sink func(rate, channels int, block []float32)) (info Info, err error) {
	// The decoder panics on some hostile inputs (for example an Ogg page with
	// no segments); a bad file must be an error, not a crash.
	defer func() {
		if p := recover(); p != nil {
			info = Info{}
			err = errors.New("AudioDecoderPanic", "the audio file is corrupt", "panic", p)
		}
	}()

	if cerr := ctx.Err(); cerr != nil {
		return Info{}, errors.NewErr("AudioCanceled", cerr)
	}
	// Hide io.Seeker: the decoder would seek to the end of the file to read
	// the length, which we count instead.
	rd, rerr := oggvorbis.NewReader(struct{ io.Reader }{r})
	if rerr != nil {
		return Info{}, errors.NewErr("AudioHeaderInvalid", rerr)
	}
	rate, channels := rd.SampleRate(), rd.Channels()
	if rate <= 0 || channels <= 0 {
		return Info{}, errors.New("AudioFormatInvalid", "the audio file has no sample rate or no channels",
			"sample_rate", rate, "channels", channels)
	}
	maxFrames := int64(math.MaxInt64)
	if maxSeconds > 0 {
		maxFrames = int64(maxSeconds * float64(rate))
	}

	// Always call sink once, so that a file without samples still reports its
	// format.
	sink(rate, channels, nil)
	buf := make([]float32, blockFrames*channels)
	var frames int64
	for {
		if cerr := ctx.Err(); cerr != nil {
			return Info{}, errors.NewErr("AudioCanceled", cerr)
		}
		n, derr := rd.Read(buf)
		if n > 0 {
			frames += int64(n / channels)
			if frames > maxFrames {
				return Info{}, errors.New("AudioTooLong", "the audio is longer than the limit",
					"max_seconds", maxSeconds)
			}
			sink(rate, channels, buf[:n])
		}
		if derr == io.EOF {
			break
		}
		if derr != nil {
			return Info{}, errors.NewErr("AudioDataCorrupt", derr).SetMetadata("frames_decoded", frames)
		}
	}
	return Info{SampleRate: rate, Channels: channels, Frames: frames}, nil
}

// countingReader counts the bytes read through it.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

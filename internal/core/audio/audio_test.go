package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
	"testing/iotest"
	"time"

	"github.com/rm4n0s/errors"
)

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// oggCRC is the checksum of an Ogg page (polynomial 0x04c11db7, no reflection).
func oggCRC(data []byte) uint32 {
	var crc uint32
	for _, b := range data {
		crc ^= uint32(b) << 24
		for i := 0; i < 8; i++ {
			if crc&0x80000000 != 0 {
				crc = crc<<1 ^ 0x04c11db7
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// patchIdentification returns a copy of an Ogg Vorbis file with the channel
// count and the sample rate of its identification header replaced, and the
// checksum of the first page fixed.
func patchIdentification(t *testing.T, data []byte, channels byte, rate uint32) []byte {
	t.Helper()
	out := append([]byte(nil), data...)
	// Page header (27 bytes) + one segment-table byte + packet type and
	// "vorbis" (7) + version (4).
	const chanOff = 27 + 1 + 7 + 4
	out[chanOff] = channels
	binary.LittleEndian.PutUint32(out[chanOff+1:], rate)
	segments := int(out[26])
	pageLen := 27 + segments
	for _, s := range out[27 : 27+segments] {
		pageLen += int(s)
	}
	page := out[:pageLen]
	binary.LittleEndian.PutUint32(page[22:], 0)
	binary.LittleEndian.PutUint32(page[22:], oggCRC(page))
	return out
}

// wantRoute asserts the route of err.
func wantRoute(t *testing.T, err error, route string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error with route %q, got nil", route)
	}
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T: %v", err, err)
	}
	if !appErr.HasRoute(route) {
		t.Errorf("unexpected failure path %q, want it to contain %q", appErr.Route(), route)
	}
}

var goodFiles = []struct {
	file     string
	rate     int
	channels int
}{
	{"tone-mono-44100.ogg", 44100, 1},
	{"tone-stereo-48000.ogg", 48000, 2},
	{"tone-mono-22050.ogg", 22050, 1},
	{"tone-mono-16000.ogg", 16000, 1},
}

func TestProbe(t *testing.T) {
	for _, tt := range goodFiles {
		t.Run(tt.file, func(t *testing.T) {
			data := readTestdata(t, tt.file)
			info, err := Probe(context.Background(), bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			want := Info{SampleRate: tt.rate, Channels: tt.channels, Frames: int64(tt.rate), Bytes: int64(len(data))}
			if info != want {
				t.Errorf("Probe = %+v, want %+v", info, want)
			}
			if got := info.Seconds(); got != 1 {
				t.Errorf("Seconds = %v, want 1", got)
			}
			if got := info.Duration(); got != time.Second {
				t.Errorf("Duration = %v, want 1s", got)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	for _, tt := range goodFiles {
		t.Run(tt.file, func(t *testing.T) {
			clip, err := Decode(context.Background(), bytes.NewReader(readTestdata(t, tt.file)))
			if err != nil {
				t.Fatal(err)
			}
			if clip.SampleRate != tt.rate || clip.Channels != tt.channels {
				t.Errorf("format = %d Hz, %d channels; want %d Hz, %d channels", clip.SampleRate, clip.Channels, tt.rate, tt.channels)
			}
			if clip.Frames() != tt.rate || len(clip.Samples) != tt.rate*tt.channels {
				t.Errorf("frames = %d, samples = %d; want %d frames", clip.Frames(), len(clip.Samples), tt.rate)
			}
			if clip.Duration() != time.Second || clip.Seconds() != 1 {
				t.Errorf("Duration = %v, Seconds = %v; want 1s", clip.Duration(), clip.Seconds())
			}
			// A 440 Hz sine of one second crosses zero about 880 times in
			// every channel, and no sample is outside -1..1.
			for ch := 0; ch < tt.channels; ch++ {
				crossings, peak := 0, 0.0
				for i := 1; i < clip.Frames(); i++ {
					a, b := clip.Samples[(i-1)*tt.channels+ch], clip.Samples[i*tt.channels+ch]
					if (a < 0) != (b < 0) {
						crossings++
					}
					peak = math.Max(peak, math.Abs(float64(b)))
				}
				if crossings < 870 || crossings > 890 {
					t.Errorf("channel %d: %d zero crossings, want about 880", ch, crossings)
				}
				if peak < 0.05 || peak > 1 {
					t.Errorf("channel %d: peak %v, want in [0.05, 1]", ch, peak)
				}
			}
		})
	}
}

func TestProbeMatchesDecode(t *testing.T) {
	for _, tt := range goodFiles {
		data := readTestdata(t, tt.file)
		info, err := Probe(context.Background(), bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		clip, err := Decode(context.Background(), bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if int64(clip.Frames()) != info.Frames {
			t.Errorf("%s: Decode has %d frames, Probe %d", tt.file, clip.Frames(), info.Frames)
		}
	}
}

// A reader that is not an io.Seeker and returns one byte at a time gives the
// same result.
func TestUnseekableReader(t *testing.T) {
	data := readTestdata(t, "tone-stereo-48000.ogg")
	info, err := Probe(context.Background(), iotest.OneByteReader(bytes.NewReader(data)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Frames != 48000 || info.Channels != 2 || info.Bytes != int64(len(data)) {
		t.Errorf("Probe = %+v", info)
	}
	clip, err := Decode(context.Background(), iotest.HalfReader(bytes.NewReader(data)))
	if err != nil {
		t.Fatal(err)
	}
	if clip.Frames() != 48000 {
		t.Errorf("frames = %d, want 48000", clip.Frames())
	}
}

func TestTrailingBytesAreCounted(t *testing.T) {
	data := append(readTestdata(t, "tone-mono-44100.ogg"), make([]byte, 100000)...)
	info, err := Probe(context.Background(), bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if info.Bytes != int64(len(data)) {
		t.Errorf("Bytes = %d, want %d", info.Bytes, len(data))
	}
}

func TestEmptyClip(t *testing.T) {
	if (&Clip{}).Frames() != 0 || (&Clip{}).Seconds() != 0 || (Info{}).Duration() != 0 {
		t.Error("a zero Clip or Info is not empty")
	}
}

func TestDecodeErrors(t *testing.T) {
	good := readTestdata(t, "tone-mono-44100.ogg")
	// A page with no segments makes the decoder panic.
	noSegments := append([]byte("OggS"), make([]byte, 23)...)
	tests := []struct {
		name  string
		data  []byte
		route string
	}{
		{"empty input", nil, "stream.AudioHeaderInvalid"},
		{"not an ogg file", []byte("this is not audio, it is a plain text file with no capture pattern"), "stream.AudioHeaderInvalid"},
		{"only the first bytes", good[:20], "stream.AudioHeaderInvalid"},
		{"truncated headers", good[:3000], "stream.AudioHeaderInvalid"},
		{"corrupted page", readTestdata(t, "tone-mono-44100-corrupt.ogg"), "stream.AudioDataCorrupt"},
		{"truncated file", readTestdata(t, "tone-mono-44100-truncated.ogg"), "stream.AudioDataCorrupt"},
		{"page with no segments", noSegments, "func1.AudioDecoderPanic"},
		{"zero sample rate", patchIdentification(t, good, 1, 0), "stream.AudioFormatInvalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Probe(context.Background(), bytes.NewReader(tt.data))
			wantRoute(t, err, tt.route)
			clip, err := Decode(context.Background(), bytes.NewReader(tt.data))
			wantRoute(t, err, tt.route)
			if clip != nil {
				t.Errorf("Decode returned a clip with an error")
			}
		})
	}
}

func TestZeroChannels(t *testing.T) {
	// Whatever the decoder does with zero channels, it is an error and not a
	// crash or an endless loop.
	data := patchIdentification(t, readTestdata(t, "tone-mono-44100.ogg"), 0, 44100)
	_, err := Probe(context.Background(), bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected an error")
	}
	if _, ok := errors.FromError(err); !ok {
		t.Errorf("expected *errors.Error, got %T", err)
	}
}

func TestCorruptDataKeepsOriginalError(t *testing.T) {
	_, err := Probe(context.Background(), bytes.NewReader(readTestdata(t, "tone-mono-44100-truncated.ogg")))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("errors.Is(err, io.ErrUnexpectedEOF) = false for %v", err)
	}
}

func TestDecodeLimit(t *testing.T) {
	data := readTestdata(t, "tone-mono-44100.ogg")
	tests := []struct {
		name  string
		max   float64
		route string // empty: no error
	}{
		{"no limit", 0, ""},
		{"exactly the length", 1, ""},
		{"longer than the limit", 0.5, "DecodeLimit->stream.AudioTooLong"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clip, err := DecodeLimit(context.Background(), bytes.NewReader(data), tt.max)
			if tt.route != "" {
				wantRoute(t, err, tt.route)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if clip.Frames() != 44100 {
				t.Errorf("frames = %d, want 44100", clip.Frames())
			}
		})
	}
}

// cancelAfter is a reader that cancels a context once n bytes were read.
type cancelAfter struct {
	r      io.Reader
	n      int
	cancel context.CancelFunc
}

func (c *cancelAfter) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n -= n
	if c.n <= 0 {
		c.cancel()
	}
	return n, err
}

func TestCancellation(t *testing.T) {
	data := readTestdata(t, "tone-stereo-48000.ogg")
	t.Run("already cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := Probe(ctx, bytes.NewReader(data))
		wantRoute(t, err, "Probe->stream.AudioCanceled")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("errors.Is(err, context.Canceled) = false for %v", err)
		}
		_, err = Decode(ctx, bytes.NewReader(data))
		wantRoute(t, err, "Decode->DecodeLimit->stream.AudioCanceled")
	})
	t.Run("while decoding", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// The headers are in the first page; cancel after the audio starts.
		_, err := Decode(ctx, &cancelAfter{r: bytes.NewReader(data), n: len(data) - 1000, cancel: cancel})
		wantRoute(t, err, "Decode->DecodeLimit->stream.AudioCanceled")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("errors.Is(err, context.Canceled) = false for %v", err)
		}
	})
	t.Run("deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel()
		<-ctx.Done()
		_, err := Probe(ctx, bytes.NewReader(data))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("errors.Is(err, context.DeadlineExceeded) = false for %v", err)
		}
	})
	t.Run("while reading the trailing bytes", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		trailing := &cancelAfter{r: bytes.NewReader(make([]byte, 1<<20)), n: 1, cancel: cancel}
		_, err := Probe(ctx, io.MultiReader(bytes.NewReader(data), trailing))
		wantRoute(t, err, "Probe.AudioCanceled")
	})
}

func TestProbeTrailingReadError(t *testing.T) {
	data := readTestdata(t, "tone-mono-44100.ogg")
	boom := errors.New("Boom", "disk failure")
	_, err := Probe(context.Background(), io.MultiReader(bytes.NewReader(data), iotest.ErrReader(boom)))
	wantRoute(t, err, "Probe.AudioReadFailed")
	if !errors.Is(err, boom) {
		t.Errorf("the original error is lost: %v", err)
	}
}

package audio

import "time"

// epoch is the start of every fake clock in the tests.
var epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// rampClip returns a mono clip at 1000 Hz of the given length in
// milliseconds whose sample i is i/100000, so that the samples a sink took
// say which frames they were.
func rampClip(ms int) *Clip {
	c := &Clip{SampleRate: 1000, Channels: 1, Samples: make([]float32, ms)}
	for i := range c.Samples {
		c.Samples[i] = float32(i) / 100000
	}
	return c
}

// stereoRampClip is rampClip with two channels: frame i has i/100000 on the
// left and -i/100000 on the right.
func stereoRampClip(ms int) *Clip {
	c := &Clip{SampleRate: 1000, Channels: 2, Samples: make([]float32, 2*ms)}
	for i := 0; i < ms; i++ {
		c.Samples[2*i] = float32(i) / 100000
		c.Samples[2*i+1] = -float32(i) / 100000
	}
	return c
}

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

func near(a, b float64) bool {
	d := a - b
	return d > -1e-6 && d < 1e-6
}

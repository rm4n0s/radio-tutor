// Copied from github.com/jfreymuth/pulse v0.1.3 and changed for Radio Tutor. MIT License, Copyright (c) 2019 Johann Freymuth: see internal/core/pulse/LICENSE.

package proto_test

import (
	"math"
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/pulse/proto"
)

func TestVolume(t *testing.T) {
	for n := 0; n <= 200; n++ {
		slider := float64(n) / 100
		volume := proto.LinearVolume(slider)
		slider2 := volume.Linear()
		if math.Abs(slider-slider2) > 0.0001 {
			t.Errorf("pulse.LinearVolume(%f).Linear() became %f", slider, slider2)
		}
	}
}

// Make sure all volume values survive a roundtrip through the linear
// representation.
func TestVolumeRoundTrip(t *testing.T) {
	for n := 0; n <= 0x10000*10; n++ {
		volume := proto.Volume(n)
		volume2 := proto.LinearVolume(volume.Linear())
		if volume != volume2 {
			t.Errorf("pulse.LinearVolume(pulse.Volume(n).Linear(%d)) became %d", n, volume2)
		}
	}
}

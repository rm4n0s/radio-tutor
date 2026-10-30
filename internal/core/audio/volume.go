package audio

import "math"

// MaxVolume is the loudest volume setting, in percent: the samples are played
// as they are.
const MaxVolume = 100

// Gain returns the factor the samples are multiplied by for a volume setting
// in percent (SPEC §8.1). The slider is perceptual: the gain is the cube of
// the fraction, the curve of the volume sliders of desktops, so 100 is 1, 50 is
// 0.125 and 0 is silence. A volume outside 0 to 100 is taken as the nearest
// end.
func Gain(volume int) float32 {
	f := float32(min(max(volume, 0), MaxVolume)) / MaxVolume
	return f * f * f
}

// SetVolume sets the volume, in percent, of the sound of the Player (SPEC
// §8.1); the default is MaxVolume. It takes effect at once: the clip that plays
// goes on from the next samples the sink pulls (a sink may have a short buffer
// of samples already, so the change is heard within a fraction of a second), and
// a clip started or resumed later plays at the new volume. Volume 0 plays
// silence: the clip still runs and ends. The volume is kept while the Player is
// muted. It is safe for concurrent use, also from the goroutine that is
// starting a clip, and it never blocks on the sound server.
func (p *Player) SetVolume(volume int) {
	p.volMu.Lock()
	defer p.volMu.Unlock()
	p.vol = Gain(volume)
	p.applyLevelLocked()
}

// SetMuted mutes the Player or lets it sound again, with the volume it was set
// to (SPEC §8.1). Like SetVolume it takes effect at once and is safe for
// concurrent use. A muted clip still runs and ends.
func (p *Player) SetMuted(muted bool) {
	p.volMu.Lock()
	defer p.volMu.Unlock()
	p.muted = muted
	p.applyLevelLocked()
}

// applyLevelLocked publishes the gain to the sink's goroutine. p.volMu must be
// held.
func (p *Player) applyLevelLocked() {
	gain := p.vol
	if p.muted {
		gain = 0
	}
	p.level.Store(math.Float32bits(gain))
}

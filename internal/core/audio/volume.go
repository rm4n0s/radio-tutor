package audio

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

// SetVolume sets the volume, in percent, of the sound the Player starts from
// now on (SPEC §8.1); the default is MaxVolume. A clip that already plays, or
// is paused, keeps the volume it started with. Volume 0 plays silence: the
// clip still runs and ends. It is safe for concurrent use.
func (p *Player) SetVolume(volume int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gain = Gain(volume)
	p.gainSet = true
}

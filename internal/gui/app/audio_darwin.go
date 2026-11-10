//go:build darwin

package app

// useGlutterAudio says that the default sink is the one of Glutter's audio: macOS has no PulseAudio, and its Core Audio is reached
// through Glutter (audio_glutter.go).
const useGlutterAudio = true

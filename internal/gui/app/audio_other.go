//go:build !darwin

package app

// useGlutterAudio is false where the core has a sink of its own: PulseAudio on Linux, waveOut on Windows.
const useGlutterAudio = false

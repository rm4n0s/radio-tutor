//go:build windows

package main

import (
	"syscall"
	"testing"
)

// TestWineHasNoAudioDevice guards the rule that a test run under Wine never
// plays on the speakers of the machine: scripts/wine-exec.sh turns the audio
// driver of the prefix off, and winmm then reports no output device. On a real
// Windows PC the test does not apply (the PC has devices), and it is skipped.
func TestWineHasNoAudioDevice(t *testing.T) {
	if syscall.NewLazyDLL("ntdll.dll").NewProc("wine_get_version").Find() != nil {
		t.Skip("not Wine: a real Windows PC has sound devices")
	}
	proc := syscall.NewLazyDLL("winmm.dll").NewProc("waveOutGetNumDevs")
	if err := proc.Find(); err != nil {
		t.Fatalf("waveOutGetNumDevs: %v", err)
	}
	n, _, _ := proc.Call()
	if n != 0 {
		t.Fatalf("waveOutGetNumDevs = %d under Wine, want 0: the audio driver of the prefix is not turned off (scripts/wine-exec.sh)", n)
	}
}

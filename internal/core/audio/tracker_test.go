package audio

import (
	"testing"
)

func TestTracker(t *testing.T) {
	type step struct {
		do      string // start, pause, resume, stop, advance, observe, stale
		ms      int    // advance by; observe: the position in ms
		wantPos float64
	}
	tests := []struct {
		name  string
		steps []step
	}{
		{"idle is 0", []step{{"advance", 500, 0}}},
		{"runs with the clock", []step{{"start", 0, 0}, {"advance", 250, 0.25}, {"advance", 250, 0.5}}},
		{"pause holds", []step{{"start", 0, 0}, {"advance", 300, 0.3}, {"pause", 0, 0.3}, {"advance", 1000, 0.3}}},
		{"resume continues from the hold", []step{
			{"start", 0, 0}, {"advance", 300, 0.3}, {"pause", 0, 0.3}, {"advance", 1000, 0.3},
			{"resume", 0, 0.3}, {"advance", 200, 0.5},
		}},
		{"stop goes to 0 and holds", []step{{"start", 0, 0}, {"advance", 300, 0.3}, {"stop", 0, 0}, {"advance", 300, 0}}},
		{"start after stop restarts at 0", []step{
			{"start", 0, 0}, {"advance", 300, 0.3}, {"stop", 0, 0}, {"start", 0, 0}, {"advance", 100, 0.1},
		}},
		{"an observation ahead moves the position forward", []step{
			{"start", 0, 0}, {"advance", 100, 0.1}, {"observe", 400, 0.4}, {"advance", 100, 0.5},
		}},
		{"an observation behind holds the position until time catches up", []step{
			{"start", 0, 0}, {"advance", 500, 0.5}, {"observe", 300, 0.5}, {"advance", 100, 0.5}, {"advance", 150, 0.55},
		}},
		{"an observation from before a pause is dropped", []step{
			{"start", 0, 0}, {"advance", 100, 0.1}, {"stale", 900, 0.1}, {"advance", 100, 0.2},
		}},
		{"an observation while paused is dropped", []step{
			{"start", 0, 0}, {"advance", 100, 0.1}, {"pause", 0, 0.1}, {"observe", 900, 0.1},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewFakeClock(epoch)
			tr := newTracker(c)
			for i, s := range tt.steps {
				switch s.do {
				case "start":
					tr.start()
				case "pause":
					tr.pause()
				case "resume":
					tr.resume()
				case "stop":
					tr.stop()
				case "advance":
					c.Advance(ms(s.ms))
				case "observe":
					e, _ := tr.token()
					if !tr.observe(e, float64(s.ms)/1000, c.Now()) && tt.name != "an observation while paused is dropped" {
						t.Fatalf("step %d: observation refused", i)
					}
				case "stale":
					// A measurement begun now, finished after a pause and a resume.
					e, _ := tr.token()
					tr.pause()
					tr.resume()
					if tr.observe(e, float64(s.ms)/1000, c.Now()) {
						t.Fatalf("step %d: stale observation accepted", i)
					}
				}
				if got := tr.position(); !near(got, s.wantPos) {
					t.Fatalf("step %d (%s): position = %v, want %v", i, s.do, got, s.wantPos)
				}
			}
		})
	}
}

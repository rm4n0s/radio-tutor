package audio

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
)

// newTestPlayer returns a Player on a FakeSink, both on one FakeClock.
func newTestPlayer(t *testing.T, clip *Clip) (*Player, *FakeSink, *FakeClock) {
	t.Helper()
	clock := NewFakeClock(epoch)
	sink := NewFakeSink(clock)
	p := NewPlayer(sink, clock)
	if clip != nil {
		if err := p.Load(clip); err != nil {
			t.Fatal(err)
		}
	}
	return p, sink, clock
}

func endedCount(p *Player) int {
	n := 0
	for {
		select {
		case <-p.Ended():
			n++
		default:
			return n
		}
	}
}

func TestPlayerPositions(t *testing.T) {
	type step struct {
		do        string // play, pause, stop, advance
		ms        int
		wantState State
		wantPos   float64
	}
	tests := []struct {
		name  string
		steps []step
	}{
		{"loaded and stopped", []step{{"advance", 500, StateStopped, 0}}},
		{"play advances", []step{
			{"play", 0, StatePlaying, 0}, {"advance", 500, StatePlaying, 0.5}, {"advance", 700, StatePlaying, 1.2},
		}},
		{"pause holds", []step{
			{"play", 0, StatePlaying, 0}, {"advance", 500, StatePlaying, 0.5},
			{"pause", 0, StatePaused, 0.5}, {"advance", 5000, StatePaused, 0.5},
		}},
		{"play after pause continues", []step{
			{"play", 0, StatePlaying, 0}, {"advance", 500, StatePlaying, 0.5}, {"pause", 0, StatePaused, 0.5},
			{"advance", 5000, StatePaused, 0.5}, {"play", 0, StatePlaying, 0.5}, {"advance", 300, StatePlaying, 0.8},
		}},
		{"stop goes back to 0", []step{
			{"play", 0, StatePlaying, 0}, {"advance", 500, StatePlaying, 0.5},
			{"stop", 0, StateStopped, 0}, {"advance", 500, StateStopped, 0},
		}},
		{"stop while paused goes back to 0", []step{
			{"play", 0, StatePlaying, 0}, {"advance", 500, StatePlaying, 0.5},
			{"pause", 0, StatePaused, 0.5}, {"stop", 0, StateStopped, 0},
		}},
		{"play after stop starts from 0", []step{
			{"play", 0, StatePlaying, 0}, {"advance", 500, StatePlaying, 0.5}, {"stop", 0, StateStopped, 0},
			{"play", 0, StatePlaying, 0}, {"advance", 200, StatePlaying, 0.2},
		}},
		{"pause and stop while stopped do nothing", []step{
			{"pause", 0, StateStopped, 0}, {"stop", 0, StateStopped, 0}, {"advance", 100, StateStopped, 0},
		}},
		{"play while playing does nothing", []step{
			{"play", 0, StatePlaying, 0}, {"advance", 400, StatePlaying, 0.4},
			{"play", 0, StatePlaying, 0.4}, {"advance", 100, StatePlaying, 0.5},
		}},
		{"pause while paused does nothing", []step{
			{"play", 0, StatePlaying, 0}, {"advance", 400, StatePlaying, 0.4}, {"pause", 0, StatePaused, 0.4},
			{"pause", 0, StatePaused, 0.4}, {"advance", 100, StatePaused, 0.4},
		}},
		{"the end", []step{
			{"play", 0, StatePlaying, 0}, {"advance", 1999, StatePlaying, 1.999}, {"advance", 1, StateEnded, 2},
			{"advance", 1000, StateEnded, 2},
		}},
		{"a long advance past the end stops at the duration", []step{
			{"play", 0, StatePlaying, 0}, {"advance", 60000, StateEnded, 2},
		}},
		{"play after the end restarts from 0", []step{
			{"play", 0, StatePlaying, 0}, {"advance", 2500, StateEnded, 2},
			{"play", 0, StatePlaying, 0}, {"advance", 300, StatePlaying, 0.3}, {"advance", 1700, StateEnded, 2},
		}},
		{"stop after the end goes back to 0", []step{
			{"play", 0, StatePlaying, 0}, {"advance", 2500, StateEnded, 2}, {"stop", 0, StateStopped, 0},
		}},
		{"a pause delays the end", []step{
			{"play", 0, StatePlaying, 0}, {"advance", 1500, StatePlaying, 1.5}, {"pause", 0, StatePaused, 1.5},
			{"advance", 10000, StatePaused, 1.5}, {"play", 0, StatePlaying, 1.5},
			{"advance", 400, StatePlaying, 1.9}, {"advance", 100, StateEnded, 2},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _, clock := newTestPlayer(t, rampClip(2000))
			for i, s := range tt.steps {
				switch s.do {
				case "play":
					if err := p.Play(context.Background()); err != nil {
						t.Fatalf("step %d: %v", i, err)
					}
				case "pause":
					p.Pause()
				case "stop":
					p.Stop()
				case "advance":
					clock.Advance(ms(s.ms))
				}
				if got := p.State(); got != s.wantState {
					t.Fatalf("step %d (%s): state = %v, want %v", i, s.do, got, s.wantState)
				}
				if got := p.Position(); !near(got, s.wantPos) {
					t.Fatalf("step %d (%s): position = %v, want %v", i, s.do, got, s.wantPos)
				}
			}
		})
	}
}

func TestPlayerEndedSignal(t *testing.T) {
	t.Run("one signal per end, after the state changed", func(t *testing.T) {
		p, sink, clock := newTestPlayer(t, rampClip(1000))
		if err := p.Play(context.Background()); err != nil {
			t.Fatal(err)
		}
		clock.Advance(ms(999))
		if n := endedCount(p); n != 0 {
			t.Fatalf("signalled %d times before the end", n)
		}
		clock.Advance(ms(1))
		if p.State() != StateEnded {
			t.Fatalf("state = %v", p.State())
		}
		if n := endedCount(p); n != 1 {
			t.Fatalf("signalled %d times at the end, want 1", n)
		}
		if got, want := sink.Calls(), []string{"start", "stop"}; !slices.Equal(got, want) {
			t.Fatalf("sink calls = %v, want %v (the stream is released at the end)", got, want)
		}
		if clock.Pending() != 0 {
			t.Fatalf("%d timers left", clock.Pending())
		}
	})

	t.Run("pause and stop do not signal", func(t *testing.T) {
		p, _, clock := newTestPlayer(t, rampClip(1000))
		if err := p.Play(context.Background()); err != nil {
			t.Fatal(err)
		}
		clock.Advance(ms(500))
		p.Pause()
		clock.Advance(ms(5000))
		if err := p.Play(context.Background()); err != nil {
			t.Fatal(err)
		}
		p.Stop()
		clock.Advance(ms(5000))
		if n := endedCount(p); n != 0 {
			t.Fatalf("signalled %d times without an end", n)
		}
		if clock.Pending() != 0 {
			t.Fatalf("%d timers left", clock.Pending())
		}
	})

	t.Run("ends nobody read are merged", func(t *testing.T) {
		p, _, clock := newTestPlayer(t, rampClip(100))
		for range 3 {
			if err := p.Play(context.Background()); err != nil {
				t.Fatal(err)
			}
			clock.Advance(ms(100))
		}
		if n := endedCount(p); n != 1 {
			t.Fatalf("signalled %d times, want 1", n)
		}
	})

	t.Run("an empty clip ends at once", func(t *testing.T) {
		p, sink, _ := newTestPlayer(t, &Clip{SampleRate: 1000, Channels: 1})
		if err := p.Play(context.Background()); err != nil {
			t.Fatal(err)
		}
		if p.State() != StateEnded || p.Position() != 0 || p.Duration() != 0 {
			t.Fatalf("state %v position %v duration %v", p.State(), p.Position(), p.Duration())
		}
		if n := endedCount(p); n != 1 {
			t.Fatalf("signalled %d times, want 1", n)
		}
		if len(sink.Calls()) != 0 {
			t.Fatalf("the sink was used: %v", sink.Calls())
		}
	})
}

// lagSink is a FakeSink whose reported position is behind the real one, like
// a device that started late.
type lagSink struct {
	*FakeSink
	lag float64
}

func (l lagSink) Position() float64 { return max(l.FakeSink.Position()-l.lag, 0) }

func TestPlayerEndWaitsForLateSink(t *testing.T) {
	clock := NewFakeClock(epoch)
	sink := lagSink{NewFakeSink(clock), 0.25}
	p := NewPlayer(sink, clock)
	if err := p.Load(rampClip(1000)); err != nil {
		t.Fatal(err)
	}
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(ms(1000))
	if p.State() != StatePlaying || !near(p.Position(), 0.75) {
		t.Fatalf("at the nominal end: state %v position %v", p.State(), p.Position())
	}
	clock.Advance(ms(250))
	if p.State() != StateEnded {
		t.Fatalf("state = %v after the lag", p.State())
	}
	if n := endedCount(p); n != 1 {
		t.Fatalf("signalled %d times", n)
	}
}

func TestPlayerPositionNeverRunsBackwards(t *testing.T) {
	// Every possible interleaving of a few calls, with the position read after
	// each: it only goes down on Stop (and on a Play that restarts after the
	// end).
	ops := []string{"play", "pause", "advance", "advance-long"}
	run := func(t *testing.T, seq []string) {
		p, _, clock := newTestPlayer(t, rampClip(1000))
		last := 0.0
		for i, op := range seq {
			switch op {
			case "play":
				wasEnded := p.State() == StateEnded
				if err := p.Play(context.Background()); err != nil {
					t.Fatal(err)
				}
				if wasEnded {
					last = 0
				}
			case "pause":
				p.Pause()
			case "advance":
				clock.Advance(ms(300))
			case "advance-long":
				clock.Advance(ms(900))
			}
			got := p.Position()
			if got < last-1e-9 {
				t.Fatalf("%v: step %d (%s): position went back from %v to %v", seq, i, op, last, got)
			}
			last = got
		}
	}
	var gen func(prefix []string, depth int)
	gen = func(prefix []string, depth int) {
		if depth == 0 {
			seq := slices.Clone(prefix)
			run(t, seq)
			return
		}
		for _, o := range ops {
			gen(append(prefix, o), depth-1)
		}
	}
	gen(nil, 6)
}

func TestPlayerDeliversTheSamples(t *testing.T) {
	tests := []struct {
		name string
		clip *Clip
	}{
		{"mono", rampClip(1000)},
		{"stereo", stereoRampClip(1000)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, sink, clock := newTestPlayer(t, tt.clip)
			if err := p.Play(context.Background()); err != nil {
				t.Fatal(err)
			}
			clock.Advance(ms(400))
			p.Pause()
			if got, want := len(sink.Played()), 400*tt.clip.Channels; got != want {
				t.Fatalf("after 400 ms the device took %d samples, want %d", got, want)
			}
			clock.Advance(ms(10000))
			if got := len(sink.Played()); got != 400*tt.clip.Channels {
				t.Fatalf("the device took samples while paused: %d", got)
			}
			if err := p.Play(context.Background()); err != nil {
				t.Fatal(err)
			}
			clock.Advance(ms(600))
			if p.State() != StateEnded {
				t.Fatalf("state = %v", p.State())
			}
			// The simulated device stops with the sink at the end, so look at
			// what it took before that: the whole clip, once, in order.
			if !slices.Equal(sink.Played(), tt.clip.Samples) {
				t.Fatalf("the device did not take exactly the clip's samples (%d of %d)", len(sink.Played()), len(tt.clip.Samples))
			}
			if got, want := sink.Calls(), []string{"start", "pause", "resume", "stop"}; !slices.Equal(got, want) {
				t.Fatalf("sink calls = %v, want %v", got, want)
			}
		})
	}
}

func TestPlayerRestartDeliversTheClipAgain(t *testing.T) {
	p, sink, clock := newTestPlayer(t, rampClip(500))
	for i := range 2 {
		if err := p.Play(context.Background()); err != nil {
			t.Fatal(err)
		}
		clock.Advance(ms(500))
		if p.State() != StateEnded {
			t.Fatalf("run %d: state = %v", i, p.State())
		}
	}
	if got, want := sink.Calls(), []string{"start", "stop", "start", "stop"}; !slices.Equal(got, want) {
		t.Fatalf("sink calls = %v, want %v", got, want)
	}
	if !slices.Equal(sink.Played(), rampClip(500).Samples) {
		t.Fatal("the second run did not deliver the whole clip from its start")
	}
}

func TestPlayerLoadStopsPlayback(t *testing.T) {
	p, sink, clock := newTestPlayer(t, rampClip(1000))
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(ms(300))
	if err := p.Load(rampClip(2000)); err != nil {
		t.Fatal(err)
	}
	if p.State() != StateStopped || p.Position() != 0 || p.Duration() != 2 {
		t.Fatalf("state %v position %v duration %v", p.State(), p.Position(), p.Duration())
	}
	if clock.Pending() != 0 {
		t.Fatalf("%d timers left", clock.Pending())
	}
	if got, want := sink.Calls(), []string{"start", "stop"}; !slices.Equal(got, want) {
		t.Fatalf("sink calls = %v, want %v", got, want)
	}
}

func TestPlayerErrors(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	sinkDown := errors.New("TestSinkDown", "the sink is down")

	tests := []struct {
		name  string
		do    func(t *testing.T) error
		route string
	}{
		{"no clip", func(t *testing.T) error {
			p, _, _ := newTestPlayer(t, nil)
			return p.Play(context.Background())
		}, "Player.Play.PlayerNoClip"},
		{"nil clip", func(t *testing.T) error {
			p, _, _ := newTestPlayer(t, nil)
			return p.Load(nil)
		}, "Player.Load.PlayerClipInvalid"},
		{"no sample rate", func(t *testing.T) error {
			p, _, _ := newTestPlayer(t, nil)
			return p.Load(&Clip{Channels: 1, Samples: []float32{0}})
		}, "Player.Load.PlayerClipInvalid"},
		{"three channels", func(t *testing.T) error {
			p, _, _ := newTestPlayer(t, nil)
			return p.Load(&Clip{SampleRate: 1000, Channels: 3, Samples: make([]float32, 3)})
		}, "Player.Load.PlayerClipInvalid"},
		{"half a frame", func(t *testing.T) error {
			p, _, _ := newTestPlayer(t, nil)
			return p.Load(&Clip{SampleRate: 1000, Channels: 2, Samples: make([]float32, 3)})
		}, "Player.Load.PlayerClipInvalid"},
		{"load when closed", func(t *testing.T) error {
			p, _, _ := newTestPlayer(t, nil)
			p.Close()
			return p.Load(rampClip(10))
		}, "Player.Load.PlayerClosed"},
		{"play when closed", func(t *testing.T) error {
			p, _, _ := newTestPlayer(t, rampClip(10))
			p.Close()
			return p.Play(context.Background())
		}, "Player.Play.PlayerClosed"},
		{"play with a cancelled context", func(t *testing.T) error {
			p, _, _ := newTestPlayer(t, rampClip(10))
			return p.Play(cancelled)
		}, "Player.Play.PlayerCanceled"},
		{"the sink cannot start", func(t *testing.T) error {
			p, sink, _ := newTestPlayer(t, rampClip(10))
			sink.FailStartWith(sinkDown)
			return p.Play(context.Background())
		}, "FakeSink.Start"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.do(t)
			if tt.name == "the sink cannot start" {
				// The Player returns the sink's error untouched.
				if err != sinkDown {
					t.Fatalf("err = %v, want the sink's own error", err)
				}
				return
			}
			wantRoute(t, err, tt.route)
		})
	}
}

func TestPlayerAfterFailedStart(t *testing.T) {
	p, sink, clock := newTestPlayer(t, rampClip(1000))
	sink.FailStartWith(errors.New("TestSinkDown", "down"))
	if err := p.Play(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
	if p.State() != StateStopped || p.Position() != 0 {
		t.Fatalf("state %v position %v", p.State(), p.Position())
	}
	sink.FailStartWith(nil)
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(ms(1000))
	if p.State() != StateEnded {
		t.Fatalf("state = %v", p.State())
	}
}

func TestPlayerResumeFailure(t *testing.T) {
	p, sink, clock := newTestPlayer(t, rampClip(1000))
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(ms(400))
	p.Pause()
	down := errors.New("TestSinkDown", "down")
	sink.FailStartWith(down)
	if err := p.Play(context.Background()); err != down {
		t.Fatalf("err = %v, want the sink's error", err)
	}
	if p.State() != StatePaused || !near(p.Position(), 0.4) {
		t.Fatalf("after the failure: state %v position %v", p.State(), p.Position())
	}
	sink.FailStartWith(nil)
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(ms(600))
	if p.State() != StateEnded {
		t.Fatalf("state = %v", p.State())
	}
}

func TestPlayerCloseKeepsSinkOpen(t *testing.T) {
	p, sink, _ := newTestPlayer(t, rampClip(1000))
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	p.Close()
	if got, want := sink.Calls(), []string{"start", "stop"}; !slices.Equal(got, want) {
		t.Fatalf("sink calls = %v, want %v (Close must not close the sink)", got, want)
	}
}

func TestPlayerConcurrentReaders(t *testing.T) {
	// The UI reads Position and State from its goroutine while the clock's
	// goroutine ends the clip and another calls Play/Pause: run under -race.
	p, _, clock := newTestPlayer(t, rampClip(1000))
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			last := 0.0
			for {
				select {
				case <-stop:
					return
				default:
				}
				pos := p.Position()
				_ = p.State()
				if pos < last-1e-9 {
					t.Errorf("position went back from %v to %v", last, pos)
					return
				}
				last = pos
			}
		}()
	}
	for range 100 {
		clock.Advance(ms(10))
	}
	close(stop)
	wg.Wait()
	if p.State() != StateEnded {
		t.Fatalf("state = %v", p.State())
	}
}

func TestPlayerWithSilentSinkOnSystemClock(t *testing.T) {
	// The real clock and the silent sink, as the app uses without a sound
	// device: a 60 ms clip ends by itself.
	p := NewPlayer(NewSilentSink(nil), nil)
	if err := p.Load(rampClip(60)); err != nil {
		t.Fatal(err)
	}
	if err := p.Play(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Ended():
	case <-time.After(5 * time.Second):
		t.Fatal("the clip did not end")
	}
	if p.State() != StateEnded || !near(p.Position(), 0.06) {
		t.Fatalf("state %v position %v", p.State(), p.Position())
	}
}

func TestStateString(t *testing.T) {
	for s, want := range map[State]string{StateStopped: "stopped", StatePlaying: "playing", StatePaused: "paused", StateEnded: "ended", State(9): "unknown"} {
		if got := s.String(); got != want {
			t.Errorf("State(%d) = %q, want %q", int(s), got, want)
		}
	}
}

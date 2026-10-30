package audio

import (
	"slices"
	"sync"
	"time"
)

// fakeWave is a waveBackend that simulates winmm: a device that consumes the
// buffers written to it only when a test says how much it plays.
type fakeWave struct {
	mu      sync.Mutex
	devices int
	openErr error
	opened  int
	format  Format
	dev     *fakeWaveDev
}

func newFakeWave() *fakeWave { return &fakeWave{devices: 1} }

func (b *fakeWave) deviceCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.devices
}

func (b *fakeWave) open(f Format, buffers, bufBytes int) (waveDevice, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.openErr != nil {
		return nil, b.openErr
	}
	b.opened++
	b.format = f
	d := &fakeWaveDev{
		bufs:   make([][]byte, buffers),
		isDone: make([]bool, buffers),
		paused: true,
		bpf:    2 * f.Channels,
		ev:     make(chan struct{}, 1),
		posOK:  true,
	}
	for i := range d.bufs {
		d.bufs[i] = make([]byte, bufBytes)
	}
	b.dev = d
	return d, nil
}

func (b *fakeWave) device() *fakeWaveDev {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.dev
}

type fakeQueued struct{ idx, n, taken int }

// fakeWaveDev is a waveDevice. Its methods are safe for concurrent use.
type fakeWaveDev struct {
	mu     sync.Mutex
	bufs   [][]byte
	isDone []bool
	queue  []fakeQueued
	paused bool
	bpf    int
	pos    int64 // frames played
	played []byte
	posOK  bool
	calls  []string
	closed bool
	ev     chan struct{}

	errWrite, errPause, errRestart, errReset, errClose error
}

func (d *fakeWaveDev) signal() {
	select {
	case d.ev <- struct{}{}:
	default:
	}
}

func (d *fakeWaveDev) buffer(i int) []byte { return d.bufs[i] }

func (d *fakeWaveDev) write(i, n int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, "write")
	if d.errWrite != nil {
		return d.errWrite
	}
	d.isDone[i] = false
	d.queue = append(d.queue, fakeQueued{idx: i, n: n})
	return nil
}

func (d *fakeWaveDev) done(i int) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.isDone[i]
}

func (d *fakeWaveDev) wait(t time.Duration) {
	select {
	case <-d.ev:
	case <-time.After(t):
	}
}

func (d *fakeWaveDev) wake() { d.signal() }

func (d *fakeWaveDev) pause() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, "pause")
	if d.errPause != nil {
		return d.errPause
	}
	d.paused = true
	return nil
}

func (d *fakeWaveDev) restart() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, "restart")
	if d.errRestart != nil {
		return d.errRestart
	}
	d.paused = false
	return nil
}

func (d *fakeWaveDev) position() (int64, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.pos, d.posOK
}

func (d *fakeWaveDev) reset() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, "reset")
	if d.errReset != nil {
		return d.errReset
	}
	for _, q := range d.queue {
		d.isDone[q.idx] = true
	}
	d.queue = nil
	d.pos = 0
	d.paused = true
	d.signal()
	return nil
}

func (d *fakeWaveDev) close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, "close")
	if d.errClose != nil {
		return d.errClose
	}
	d.closed = true
	return nil
}

// play lets the device play up to n bytes of what is queued; a paused device
// plays nothing. It returns the bytes played.
func (d *fakeWaveDev) play(n int) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	total := 0
	for !d.paused && n > 0 && len(d.queue) > 0 {
		q := &d.queue[0]
		take := min(n, q.n-q.taken)
		d.played = append(d.played, d.bufs[q.idx][q.taken:q.taken+take]...)
		q.taken += take
		n -= take
		total += take
		d.pos += int64(take / d.bpf)
		if q.taken == q.n {
			d.isDone[q.idx] = true
			d.queue = d.queue[1:]
		}
	}
	d.signal()
	return total
}

func (d *fakeWaveDev) queued() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.queue)
}

func (d *fakeWaveDev) playedBytes() []byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.played)
}

func (d *fakeWaveDev) callList() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.calls)
}

func (d *fakeWaveDev) isPaused() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.paused
}

func (d *fakeWaveDev) isClosed() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.closed
}

// errWave is the failure a fake winmm call returns.
func errWave(fn string) error { return &waveError{Func: fn, Code: 7} }

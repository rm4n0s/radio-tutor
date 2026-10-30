//go:build !windows

// These tests talk to the fake PulseAudio server of pulsetest over a Unix socket.
// Radio Tutor plays through winmm on Windows (SPEC D23), so the PulseAudio client is
// not used there, and Wine's AF_UNIX sockets reset the connection of this server.

package pulse

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/pulse/proto"
	"github.com/rm4n0s/radio-tutor/internal/core/pulse/pulsetest"
)

// collector is a Writer that keeps what it gets, and can be held up.
type collector struct {
	mu   sync.Mutex
	data []byte
	gate chan struct{} // when not nil, Write waits until it is closed
	fail error
}

func (c *collector) Format() byte { return proto.FormatUint8 }

func (c *collector) Write(p []byte) (int, error) {
	if c.gate != nil {
		<-c.gate
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fail != nil {
		return 0, c.fail
	}
	c.data = append(c.data, p...)
	return len(p), nil
}

func (c *collector) bytes() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.data)
}

func TestRecordDeliversPacketsInOrder(t *testing.T) {
	srv := pulsetest.NewServer(t)
	c := newTestClient(t, srv, 2*time.Second)
	w := &collector{}
	stream, err := c.NewRecord(w)
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}
	t.Cleanup(stream.Close)
	within(t, "Start", stream.Start)
	if !stream.Running() {
		t.Fatal("the stream does not run")
	}

	var want []byte
	for i := range 20 {
		chunk := slices.Repeat([]byte{byte(i)}, 100+i)
		want = append(want, chunk...)
		srv.SendData(stream.StreamIndex(), chunk)
	}
	eventually(t, "the data", func() bool { return len(w.bytes()) >= len(want) })
	if got := w.bytes(); !slices.Equal(got, want) {
		t.Errorf("recorded %d bytes that differ from the %d sent", len(got), len(want))
	}

	within(t, "Stop", stream.Stop)
	if stream.Running() {
		t.Error("the stream still runs after Stop")
	}
}

// TestRecordWriterDoesNotBlockTheConnection: a Writer that takes its time must
// not stop the connection's read loop.
func TestRecordWriterDoesNotBlockTheConnection(t *testing.T) {
	srv := pulsetest.NewServer(t)
	c := newTestClient(t, srv, 2*time.Second)
	w := &collector{gate: make(chan struct{})}
	var once sync.Once
	release := func() { once.Do(func() { close(w.gate) }) }
	t.Cleanup(release)
	stream, err := c.NewRecord(w)
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}
	t.Cleanup(stream.Close)
	within(t, "Start", stream.Start)

	for range 30 {
		srv.SendData(stream.StreamIndex(), make([]byte, 64))
	}
	within(t, "a request while the writer is held up", func() {
		if err := c.RawRequest(&proto.Subscribe{Mask: proto.SubscriptionMaskSinkInput}, nil); err != nil {
			t.Errorf("the connection does not answer: %v", err)
		}
	})
	release()
	eventually(t, "the held-up data", func() bool { return len(w.bytes()) == 30*64 })
}

func TestRecordWriterError(t *testing.T) {
	boom := errors.New("WriterBoom", "the writer failed")
	srv := pulsetest.NewServer(t)
	c := newTestClient(t, srv, 2*time.Second)
	w := &collector{fail: boom}
	stream, err := c.NewRecord(w)
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}
	t.Cleanup(stream.Close)
	within(t, "Start", stream.Start)
	srv.SendData(stream.StreamIndex(), []byte{1, 2, 3})
	eventually(t, "the stream to stop", func() bool { return !stream.Running() })
	if got := stream.Error(); got != boom {
		t.Errorf("Error() = %v, want %v", got, boom)
	}
	within(t, "Close", stream.Close)
	if !stream.Closed() {
		t.Error("the stream is not closed")
	}
}

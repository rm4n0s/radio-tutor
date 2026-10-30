package proto

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
)

// failingConn is a connection whose writes fail after okWrites writes and
// whose reads wait until it is closed or fail with readErr.
type failingConn struct {
	mu       sync.Mutex
	okWrites int
	writeErr error
	readErr  error
	closed   chan struct{}
}

func newFailingConn() *failingConn { return &failingConn{closed: make(chan struct{})} }

func (f *failingConn) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.okWrites > 0 {
		f.okWrites--
		return len(p), nil
	}
	return 0, f.writeErr
}

func (f *failingConn) Read(p []byte) (int, error) {
	if f.readErr != nil {
		return 0, f.readErr
	}
	<-f.closed
	return 0, io.ErrClosedPipe
}

func TestClientRequestFailures(t *testing.T) {
	cases := []struct {
		name      string
		conn      func() *failingConn
		wantRoute string
	}{
		{"the header cannot be written", func() *failingConn {
			return &failingConn{writeErr: io.ErrShortWrite, closed: make(chan struct{})}
		}, "Client.Send.PulseSendFailed"},
		{"the data cannot be written", func() *failingConn {
			return &failingConn{okWrites: 1, writeErr: io.ErrShortWrite, closed: make(chan struct{})}
		}, "Client.Send.PulseSendFailed"},
		{"the connection is lost", func() *failingConn {
			f := newFailingConn()
			f.okWrites = 10
			f.readErr = io.EOF
			return f
		}, "PulseConnectionLost"},
		{"no answer", func() *failingConn {
			f := newFailingConn()
			f.okWrites = 10
			return f
		}, "Client.Request.PulseRequestTimeout"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn := tc.conn()
			t.Cleanup(func() { close(conn.closed) })
			var c Client
			c.SetTimeout(100 * time.Millisecond)
			c.Open(conn)

			// Wait for the read loop to see a read error, if the case has one.
			time.Sleep(20 * time.Millisecond)
			err := c.Request(&Subscribe{Mask: SubscriptionMaskAll}, nil)
			appErr, ok := errors.FromError(err)
			if !ok {
				t.Fatalf("expected *errors.Error, got %T (%v)", err, err)
			}
			if !appErr.HasRoute(tc.wantRoute) {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
		})
	}
}

// TestClientRequestForgetsTheTimedOutReply: a reply that comes after the
// timeout is skipped and is not parsed into the caller's value.
func TestClientRequestForgetsTheTimedOutReply(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = serverConn.Close() }()
	defer func() { _ = clientConn.Close() }()

	var c Client
	c.SetTimeout(20 * time.Millisecond)
	c.Open(clientConn)

	go func() { _, _ = io.Copy(io.Discard, serverConn) }()
	var reply SetClientNameReply
	err := c.Request(&SetClientName{Props: PropList{}}, &reply)
	if appErr, ok := errors.FromError(err); !ok || !appErr.HasRoute("Client.Request.PulseRequestTimeout") {
		t.Fatalf("err = %v, want a timeout", err)
	}

	c.replyM.Lock()
	waiting := len(c.awaitReply)
	c.replyM.Unlock()
	if waiting != 0 {
		t.Errorf("%d requests still wait for a reply after the timeout", waiting)
	}
}

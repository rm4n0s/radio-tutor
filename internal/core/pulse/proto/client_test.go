// Copied from github.com/jfreymuth/pulse v0.1.3 and changed for Radio Tutor. MIT License, Copyright (c) 2019 Johann Freymuth: see internal/core/pulse/LICENSE.

package proto

import (
	"net"
	"testing"
	"time"
)

func TestClientReadLoopHandlesEOFBeforeCallbackInstalled(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()

	var c Client
	c.Open(clientConn)

	if err := serverConn.Close(); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(100 * time.Millisecond)
	for {
		c.replyM.Lock()
		err := c.err
		c.replyM.Unlock()
		if err != nil {
			return
		}

		select {
		case <-deadline:
			t.Fatal("timed out waiting for read loop to observe closed connection")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

// Copied from github.com/jfreymuth/pulse v0.1.3 and changed for Radio Tutor. MIT License, Copyright (c) 2019 Johann Freymuth: see internal/core/pulse/LICENSE.

package pulse

import (
	"net"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/pulse/proto"
)

// The Client is the connection to the pulseaudio server. An application typically only uses a single client.
type Client struct {
	conn net.Conn
	c    *proto.Client

	mu       sync.Mutex
	playback map[uint32]*PlaybackStream
	record   map[uint32]*RecordStream

	server  string
	props   proto.PropList
	timeout time.Duration
}

// NewClient connects to the server. It fails with the errors of proto.Connect
// and proto.Client.Request (tagged Pulse...: see there).
func NewClient(opts ...ClientOption) (*Client, error) {
	c := &Client{
		props: proto.PropList{
			"media.name":                 proto.PropListString("go audio"),
			"application.name":           proto.PropListString(path.Base(os.Args[0])),
			"application.icon_name":      proto.PropListString("audio-x-generic"),
			"application.process.id":     proto.PropListString(strconv.Itoa(os.Getpid())),
			"application.process.binary": proto.PropListString(os.Args[0]),
			"window.x11.display":         proto.PropListString(os.Getenv("DISPLAY")),
		},
	}
	for _, opt := range opts {
		opt(c)
	}

	var err error
	c.c, c.conn, err = proto.Connect(c.server)
	if err != nil {
		return nil, err
	}

	if c.timeout != 0 {
		c.c.SetTimeout(c.timeout)
	}

	err = c.c.Request(&proto.SetClientName{Props: c.props}, &proto.SetClientNameReply{})
	if err != nil {
		_ = c.conn.Close() // best effort: the error that matters is returned
		return nil, err
	}

	// The callback and the stream maps are installed before the first request
	// that makes the server send events (Subscribe), so that no event meets a
	// half-built client.
	c.playback = make(map[uint32]*PlaybackStream)
	c.record = make(map[uint32]*RecordStream)
	c.c.Callback = c.handle

	// Listen for changes to the sink input, which includes changes in volume.
	err = c.c.Request(&proto.Subscribe{Mask: proto.SubscriptionMaskSinkInput}, nil)
	if err != nil {
		_ = c.conn.Close() // best effort: the error that matters is returned
		return nil, err
	}

	return c, nil
}

// handle is the callback of the connection's read loop. It runs on the loop, so
// it never blocks: events are handed to the streams without waiting for them
// (a stream's own goroutine, or a buffered channel with a non-blocking send,
// does the slow part).
func (c *Client) handle(msg interface{}) {
	switch msg := msg.(type) {
	case *proto.Request:
		c.mu.Lock()
		stream, ok := c.playback[msg.StreamIndex]
		c.mu.Unlock()
		if ok {
			stream.addRequested(int(msg.Length))
		}
	case *proto.DataPacket:
		c.mu.Lock()
		stream, ok := c.record[msg.StreamIndex]
		c.mu.Unlock()
		if ok {
			stream.write(msg.Data)
		}
	case *proto.Started:
		c.mu.Lock()
		stream, ok := c.playback[msg.StreamIndex]
		c.mu.Unlock()
		if ok {
			// Whatever the stream's state is: a Start that waits must not miss the
			// event because its reader already ended the stream or underflowed.
			stream.signalStarted()
		}
	case *proto.Underflow:
		c.mu.Lock()
		stream, ok := c.playback[msg.StreamIndex]
		c.mu.Unlock()
		if ok {
			if stream.state.is(running) {
				stream.underflow.Store(true)
			}
		}
	case *proto.ConnectionClosed:
		c.mu.Lock()
		for _, p := range c.playback {
			p.setErr(ErrConnectionClosed)
			p.state.set(serverLost)
			p.finish()
		}
		for _, r := range c.record {
			r.setErr(ErrConnectionClosed)
			r.state.set(serverLost)
			r.finish()
		}
		c.playback = make(map[uint32]*PlaybackStream)
		c.record = make(map[uint32]*RecordStream)
		c.mu.Unlock()
		_ = c.conn.Close() // the server already closed it
	case *proto.SubscribeEvent:
		if msg.Event&proto.EventFacilityMask == proto.EventSinkSinkInput {
			// Something about the sink input changed, but we don't know
			// what exactly. Signal this to the playback stream.
			var stream *PlaybackStream
			c.mu.Lock()
			for _, v := range c.playback {
				if msg.Index == v.createReply.SinkInputIndex {
					stream = v
				}
			}
			c.mu.Unlock()
			if stream != nil {
				stream.eventsLock.Lock()
				if stream.events != nil {
					// Do a non-blocking send.
					// Because subscribeEvent is a buffered channel and
					// because the channel has no contents (just signals),
					// no message will be lost due to races.
					select {
					case stream.events <- struct{}{}:
					default:
					}
				}
				stream.eventsLock.Unlock()
			}
		}
	default:
	}
}

// Close closes the client. Calling methods on a closed client may panic.
func (c *Client) Close() {
	_ = c.conn.Close() // Close has no result; the connection is gone either way
}

// A ClientOption supplies configuration when creating the client.
type ClientOption func(*Client)

// ClientApplicationName sets the application name.
// This will e.g. be displayed by a volume control application to identity the application.
// It should be human-readable and localized.
func ClientApplicationName(name string) ClientOption {
	return func(c *Client) { c.props["application.name"] = proto.PropListString(name) }
}

// ClientApplicationIconName sets the application icon using an xdg icon name.
// This will e.g. be displayed by a volume control application to identity the application.
func ClientApplicationIconName(name string) ClientOption {
	return func(c *Client) { c.props["application.icon_name"] = proto.PropListString(name) }
}

// ClientServerString will override the default server strings.
// Server strings are used to connect to the server. For the server string format see
// https://www.freedesktop.org/wiki/Software/PulseAudio/Documentation/User/ServerStrings/
func ClientServerString(s string) ClientOption {
	return func(c *Client) { c.server = s }
}

// ClientTimeout sets the timeout of requests to the specified duration.
// If d is 0, the default value (1 s) will be used.
func ClientTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.timeout = d
	}
}

// RawRequest can be used to send arbitrary requests.
//
// req should be one of the request types defined by the proto package.
//
// rpl must be a pointer to the correct reply type or nil. This funcion will panic if rpl has the wrong type.
//
// The returned error can be compared against errors defined by the proto package to check for specific errors.
//
// The function will always block until the server has replied, even if rpl is nil.
func (c *Client) RawRequest(req proto.RequestArgs, rpl proto.Reply) error {
	return c.c.Request(req, rpl)
}

// ErrConnectionClosed is a special error value indicating that the server closed the connection.
// Compare with errors.Is.
var ErrConnectionClosed error = &errors.Error{Tag: "PulseConnectionClosed", Message: "pulseaudio: connection closed"}

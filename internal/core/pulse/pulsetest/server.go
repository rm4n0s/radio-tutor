// Package pulsetest is a fake PulseAudio server for tests: it speaks enough of
// the native protocol (protocol version 9) over a unix socket for the pulse
// package's client to connect, create playback and record streams, cork and
// uncork them, send audio and ask for the latency. Tests use it instead of a
// real audio server, so no sound is ever played.
//
// The server also lets a test do what a real server does at awkward moments:
// send events (a request for data, "started", record data), answer slowly, not
// answer at all, answer with an error code, or drop the connection.
package pulsetest

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rm4n0s/radio-tutor/internal/core/pulse/proto"
)

const (
	commandChannel = 0xFFFFFFFF
	descriptorSize = 20
	bufferMax      = 8192
	bufferTarget   = 4096
)

// Server is a fake PulseAudio server listening on a unix socket in the test's
// temporary folder. Create it with NewServer; it stops when the test ends.
type Server struct {
	t    testing.TB
	ln   net.Listener
	addr string

	mu           sync.Mutex
	conns        []*conn
	nextStream   uint32
	received     map[uint32]int // bytes of audio data received, by stream
	corked       map[uint32]bool
	commands     []uint32
	ignore       map[uint32]bool
	reject       map[uint32]uint32
	startedDelay time.Duration
	latency      uint64
	wg           sync.WaitGroup
	quit         chan struct{}
}

// NewServer starts a fake server for the test.
func NewServer(t testing.TB) *Server {
	t.Helper()
	ln, err := net.Listen("unix", filepath.Join(t.TempDir(), "native"))
	if err != nil {
		t.Fatalf("pulsetest: listen: %v", err)
	}
	s := &Server{
		t:        t,
		ln:       ln,
		addr:     "unix:" + ln.Addr().String(),
		received: map[uint32]int{},
		corked:   map[uint32]bool{},
		quit:     make(chan struct{}),
		ignore:   map[uint32]bool{},
		reject:   map[uint32]uint32{},
		latency:  10000,
	}
	s.wg.Add(1)
	go s.acceptLoop()
	t.Cleanup(s.Close)
	return s
}

// Addr returns the server string for pulse.ClientServerString.
func (s *Server) Addr() string { return s.addr }

// Close stops the server and drops every connection.
func (s *Server) Close() {
	select {
	case <-s.quit:
	default:
		close(s.quit)
	}
	_ = s.ln.Close() // the listener may be closed already
	s.mu.Lock()
	conns := append([]*conn(nil), s.conns...)
	s.mu.Unlock()
	for _, c := range conns {
		_ = c.c.Close() // the connection may be closed already
	}
	s.wg.Wait()
}

// DropConnections closes every connection but keeps listening.
func (s *Server) DropConnections() {
	s.mu.Lock()
	conns := append([]*conn(nil), s.conns...)
	s.mu.Unlock()
	for _, c := range conns {
		_ = c.c.Close() // the connection may be closed already
	}
}

// StartedDelay sets how long after an uncork the server waits before it sends
// the "started" event (default none), which is when a real server sends it.
func (s *Server) StartedDelay(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startedDelay = d
}

// Ignore makes the server not answer the command op (a proto.Op constant).
func (s *Server) Ignore(op uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ignore[op] = true
}

// Reject makes the server answer the command op with the error code.
func (s *Server) Reject(op uint32, code proto.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reject[op] = uint32(code)
}

// SetLatency sets the latency the server reports to the latency query.
func (s *Server) SetLatency(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latency = uint64(d / time.Microsecond)
}

// Commands returns the numbers of the commands received so far, in order.
func (s *Server) Commands() []uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]uint32(nil), s.commands...)
}

// Received returns how many bytes of audio data the server got for the stream.
func (s *Server) Received(stream uint32) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.received[stream]
}

// Corked reports whether the stream is corked.
func (s *Server) Corked(stream uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.corked[stream]
}

// SendRequest sends the event "the stream wants length more bytes".
func (s *Server) SendRequest(stream, length uint32) {
	var e enc
	e.u32(proto.OpRequest).u32(commandChannel).u32(stream).u32(length)
	s.broadcast(commandChannel, e)
}

// SendStarted sends the event "the stream started".
func (s *Server) SendStarted(stream uint32) {
	var e enc
	e.u32(proto.OpStarted).u32(commandChannel).u32(stream)
	s.broadcast(commandChannel, e)
}

// SendUnderflow sends the event "the stream ran out of data".
func (s *Server) SendUnderflow(stream uint32) {
	var e enc
	e.u32(proto.OpUnderflow).u32(commandChannel).u32(stream)
	s.broadcast(commandChannel, e)
}

// SendData sends audio data of a record stream.
func (s *Server) SendData(stream uint32, data []byte) {
	s.mu.Lock()
	var c *conn
	if len(s.conns) > 0 {
		c = s.conns[len(s.conns)-1]
	}
	s.mu.Unlock()
	if c != nil {
		c.send(stream, data)
	}
}

// broadcast sends a command-channel message to the newest connection.
func (s *Server) broadcast(channel uint32, e enc) {
	s.mu.Lock()
	var c *conn
	if len(s.conns) > 0 {
		c = s.conns[len(s.conns)-1]
	}
	s.mu.Unlock()
	if c != nil {
		c.send(channel, e)
	}
}

type conn struct {
	c   net.Conn
	wmu sync.Mutex
}

func (c *conn) send(channel uint32, payload []byte) {
	hdr := make([]byte, descriptorSize)
	binary.BigEndian.PutUint32(hdr[0:], uint32(len(payload)))
	binary.BigEndian.PutUint32(hdr[4:], channel)
	c.wmu.Lock()
	defer c.wmu.Unlock()
	// A write fails when the test dropped the connection: nothing to report.
	if _, err := c.c.Write(hdr); err != nil {
		return
	}
	_, _ = c.c.Write(payload)
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		nc, err := s.ln.Accept()
		if err != nil {
			return
		}
		c := &conn{c: nc}
		s.mu.Lock()
		s.conns = append(s.conns, c)
		s.mu.Unlock()
		s.wg.Add(1)
		go s.serve(c)
	}
}

func (s *Server) serve(c *conn) {
	defer s.wg.Done()
	defer func() { _ = c.c.Close() }() // the connection may be closed already
	r := bufio.NewReader(c.c)
	for {
		hdr := make([]byte, descriptorSize)
		if _, err := io.ReadFull(r, hdr); err != nil {
			return
		}
		length := binary.BigEndian.Uint32(hdr[0:])
		channel := binary.BigEndian.Uint32(hdr[4:])
		payload := make([]byte, length)
		if _, err := io.ReadFull(r, payload); err != nil {
			return
		}
		if channel != commandChannel {
			s.mu.Lock()
			s.received[channel] += len(payload)
			s.mu.Unlock()
			continue
		}
		if len(payload) < 10 {
			s.t.Errorf("pulsetest: a command of %d bytes", len(payload))
			return
		}
		op := binary.BigEndian.Uint32(payload[1:])
		tag := binary.BigEndian.Uint32(payload[6:])
		s.command(c, op, tag, payload[10:])
	}
}

// command answers one command.
func (s *Server) command(c *conn, op, tag uint32, args []byte) {
	s.mu.Lock()
	s.commands = append(s.commands, op)
	ignored := s.ignore[op]
	code, rejected := s.reject[op]
	delay := s.startedDelay
	latency := s.latency
	s.mu.Unlock()
	if ignored {
		return
	}
	if rejected {
		var e enc
		e.u32(proto.OpError).u32(tag).u32(code)
		c.send(commandChannel, e)
		return
	}

	reply := func(f func(*enc)) {
		var e enc
		e.u32(proto.OpReply).u32(tag)
		if f != nil {
			f(&e)
		}
		c.send(commandChannel, e)
	}

	switch op {
	case proto.OpAuth:
		reply(func(e *enc) { e.u32(9) })
	case proto.OpSetClientName:
		reply(func(e *enc) { e.u32(1) })
	case proto.OpCreatePlaybackStream:
		s.mu.Lock()
		stream := s.nextStream
		s.nextStream++
		s.corked[stream] = true
		s.mu.Unlock()
		reply(func(e *enc) {
			e.u32(stream).u32(100 + stream).u32(bufferTarget)
			e.u32(bufferMax).u32(bufferTarget).u32(bufferTarget / 2).u32(1024)
		})
	case proto.OpCreateRecordStream:
		s.mu.Lock()
		stream := s.nextStream
		s.nextStream++
		s.corked[stream] = true
		s.mu.Unlock()
		reply(func(e *enc) {
			e.u32(stream).u32(200 + stream)
			e.u32(bufferMax).u32(1024)
		})
	case proto.OpCorkPlaybackStream, proto.OpCorkRecordStream:
		if len(args) < 6 {
			s.t.Errorf("pulsetest: a cork command of %d bytes", len(args))
			return
		}
		stream := binary.BigEndian.Uint32(args[1:])
		corked := args[5] == '1'
		s.mu.Lock()
		s.corked[stream] = corked
		s.mu.Unlock()
		reply(nil)
		if !corked && op == proto.OpCorkPlaybackStream {
			// A real server tells that the stream started after it answered.
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				select {
				case <-time.After(delay):
				case <-s.quit:
					return
				}
				var e enc
				e.u32(proto.OpStarted).u32(commandChannel).u32(stream)
				c.send(commandChannel, e)
			}()
		}
	case proto.OpGetPlaybackLatency:
		stream := binary.BigEndian.Uint32(args[1:])
		s.mu.Lock()
		got := int64(s.received[stream])
		running := !s.corked[stream]
		s.mu.Unlock()
		reply(func(e *enc) {
			e.u64(latency).u64(0).bool(running).tm().tm().i64(got).i64(got)
		})
	case proto.OpSubscribe, proto.OpFlushPlaybackStream, proto.OpFlushRecordStream,
		proto.OpDeletePlaybackStream, proto.OpDeleteRecordStream, proto.OpDrainPlaybackStream,
		proto.OpSetSinkInputVolume:
		reply(nil)
	default:
		var e enc
		e.u32(proto.OpError).u32(tag).u32(uint32(proto.ErrNotSupported))
		c.send(commandChannel, e)
	}
}

// enc builds a tag-structured payload.
type enc []byte

func (e *enc) u32(v uint32) *enc {
	*e = append(*e, 'L', byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	return e
}

func (e *enc) u64(v uint64) *enc {
	*e = append(*e, 'R')
	*e = binary.BigEndian.AppendUint64(*e, v)
	return e
}

func (e *enc) i64(v int64) *enc {
	*e = append(*e, 'r')
	*e = binary.BigEndian.AppendUint64(*e, uint64(v))
	return e
}

func (e *enc) bool(v bool) *enc {
	if v {
		*e = append(*e, '1')
	} else {
		*e = append(*e, '0')
	}
	return e
}

func (e *enc) tm() *enc {
	*e = append(*e, 'T', 0, 0, 0, 0, 0, 0, 0, 0)
	return e
}

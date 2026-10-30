// Copied from github.com/jfreymuth/pulse v0.1.3 and changed for Radio Tutor. MIT License, Copyright (c) 2019 Johann Freymuth: see internal/core/pulse/LICENSE.

package proto

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"reflect"
	"sync"
	"time"

	"github.com/rm4n0s/errors"
)

// Client speaks the PulseAudio native protocol over one connection: it sends
// requests, matches the server's replies to them, and hands every other message
// of the server to Callback.
type Client struct {
	r ProtocolReader
	w ProtocolWriter
	v Version

	replyM     sync.Mutex
	writeM     sync.Mutex
	nextID     uint32                // protected by replyM
	awaitReply map[uint32]AwaitReply // protected by replyM
	timeout    time.Duration
	err        error // protected by replyM and writeM (hold one to read, hold both to write)

	// Callback receives the messages of the server that are not replies, and the
	// data packets of record streams. It runs on the connection's read loop, so it
	// must never block: while it runs, no reply is read.
	Callback func(interface{})
}

// Version returns the protocol version in use.
func (c *Client) Version() Version {
	return c.v
}

// SetVersion lowers the protocol version in use to v, if v is lower.
func (c *Client) SetVersion(v Version) {
	c.v = c.v.Min(v)
}

// SetTimeout sets how long Request waits for a reply.
func (c *Client) SetTimeout(t time.Duration) {
	c.timeout = t
}

// Open starts the read loop on rw. It must be called once, before any request.
func (c *Client) Open(rw io.ReadWriter) {
	c.r.r = bufio.NewReader(rw)
	c.w.w = rw
	c.v = Version(32)

	c.awaitReply = make(map[uint32]AwaitReply)
	go c.readLoop()
}

// AwaitReply is a request that waits for the server's reply: the value the
// reply is parsed into, and the channel that gets the outcome.
type AwaitReply struct {
	value interface{}
	reply chan<- error
}

// Request sends req and waits for the server's reply, which is parsed into rpl
// (nil when the command has no reply data). It fails, tagged
// PulseRequestRejected (the server answered with an error code, which is the
// underlying cause: errors.Is(err, proto.ErrNoSuchEntity) works),
// PulseRequestTimeout, PulseSendFailed or PulseConnectionLost.
func (c *Client) Request(req RequestArgs, rpl Reply) error {
	if rpl != nil && req.command() != rpl.IsReplyTo() {
		panic("pulse: wrong reply type")
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	reply := make(chan error, 1)
	c.replyM.Lock()
	if c.err != nil {
		c.replyM.Unlock()
		return c.err
	}
	tag := c.nextID
	c.nextID++
	c.awaitReply[tag] = AwaitReply{rpl, reply}
	c.replyM.Unlock()

	var buf bytes.Buffer
	w := ProtocolWriter{w: &buf}
	w.byte('L')
	w.uint32(req.command())
	w.byte('L')
	w.uint32(tag)
	w.value(req, c.v)
	w.flush()

	err := c.Send(0xFFFFFFFF, buf.Bytes())
	if err != nil {
		return err
	}

	select {
	case err := <-reply:
		if code, ok := err.(Error); ok {
			return errors.NewErr("PulseRequestRejected", code).SetMetadata("command", req.command())
		}
		return err
	case <-ctx.Done():
		// A reply that comes later is skipped, not parsed into rpl.
		c.replyM.Lock()
		delete(c.awaitReply, tag)
		c.replyM.Unlock()
		return errors.NewErr("PulseRequestTimeout", ctx.Err()).SetMetadata("command", req.command(), "timeout", c.timeout.String())
	}
}

// Send writes one packet for the stream index (0xFFFFFFFF is the command
// channel). It fails, tagged PulseSendFailed or PulseConnectionLost.
func (c *Client) Send(index uint32, data []byte) error {
	c.writeM.Lock()
	defer c.writeM.Unlock()
	if c.err != nil {
		return c.err
	}
	c.w.uint32(uint32(len(data)))
	c.w.uint32(index)
	c.w.uint64(0)
	c.w.uint32(0)
	c.w.flush()
	if c.w.err != nil {
		return errors.NewErr("PulseSendFailed", c.w.err).SetMetadata("stage", "header")
	}
	if _, err := c.w.w.Write(data); err != nil {
		return errors.NewErr("PulseSendFailed", err).SetMetadata("stage", "data", "bytes", len(data))
	}
	return nil
}

func (c *Client) readLoop() {
	for {
		length := c.r.uint32()
		index := c.r.uint32()
		offset := c.r.uint64()
		flags := c.r.uint32()
		_, _ = offset, flags
		if c.r.err != nil {
			c.error(c.r.err)
			return
		}
		if index == 0xFFFFFFFF {
			c.r.byte() // L
			op := c.r.uint32()
			c.r.byte() // L
			tag := c.r.uint32()
			var message interface{}
			switch op {
			case OpError:
				c.r.byte()
				err := Error(c.r.uint32())
				c.replyM.Lock()
				a, ok := c.awaitReply[tag]
				delete(c.awaitReply, tag)
				c.replyM.Unlock()
				if ok {
					a.reply <- err
				}
			case OpReply:
				c.replyM.Lock()
				a, ok := c.awaitReply[tag]
				delete(c.awaitReply, tag)
				c.replyM.Unlock()
				if ok {
					if a.value != nil {
						if reflect.TypeOf(a.value).Elem().Kind() == reflect.Slice {
							c.parseInfoList(a.value, int(length)-10)
						} else {
							c.r.value(a.value, c.v)
						}
					} else {
						c.r.advance(int(length) - 10)
					}
					a.reply <- nil
				} else {
					c.r.advance(int(length) - 10)
				}
			case OpRequest:
				message = &Request{}
			case OpOverflow:
				message = &Overflow{}
			case OpUnderflow:
				message = &Underflow{}
			case OpPlaybackStreamKilled:
				message = &PlaybackStreamKilled{}
			case OpRecordStreamKilled:
				message = &RecordStreamKilled{}
			case OpSubscribeEvent:
				message = &SubscribeEvent{}
			case OpPlaybackStreamSuspended:
				message = &PlaybackStreamSuspended{}
			case OpRecordStreamSuspended:
				message = &RecordStreamSuspended{}
			case OpPlaybackStreamMoved:
				message = &PlaybackStreamMoved{}
			case OpRecordStreamMoved:
				message = &RecordStreamMoved{}
			case OpClientEvent:
				message = &ClientEvent{}
			case OpPlaybackStreamEvent:
				message = &PlaybackStreamEvent{}
			case OpRecordStreamEvent:
				message = &RecordStreamEvent{}
			case OpStarted:
				message = &Started{}
			case OpPlaybackBufferAttrChanged:
				message = &PlaybackBufferAttrChanged{}
			default:
				c.r.advance(int(length) - 10)
			}
			if message != nil {
				c.r.value(message, c.v)
				if c.Callback != nil {
					c.Callback(message)
				}
			}
		} else {
			buf := c.r.tmpbytes(int(length))
			if c.Callback != nil {
				c.Callback(&DataPacket{index, buf})
			}
			c.r.pos += int(length)
		}
	}
}

// error ends the client after the read loop failed with err: every request that
// waits, and every later one, fails with the tagged error.
func (c *Client) error(readErr error) {
	err := errors.NewErr("PulseConnectionLost", readErr)
	c.replyM.Lock()
	c.writeM.Lock()
	c.err = err
	c.writeM.Unlock()
	r := c.awaitReply
	c.awaitReply = make(map[uint32]AwaitReply)
	c.replyM.Unlock()
	for _, r := range r {
		r.reply <- err
	}
	if errors.Is(readErr, io.EOF) && c.Callback != nil {
		c.Callback(&ConnectionClosed{})
	}
}

func (c *Client) parseInfoList(value interface{}, length int) {
	start := c.r.pos
	for c.r.err == nil && c.r.pos-start < length {
		switch value := value.(type) {
		case *GetSinkInfoListReply:
			var v GetSinkInfoReply
			c.r.value(&v, c.v)
			*value = append(*value, &v)
		case *GetSourceInfoListReply:
			var v GetSourceInfoReply
			c.r.value(&v, c.v)
			*value = append(*value, &v)
		case *GetModuleInfoListReply:
			var v GetModuleInfoReply
			c.r.value(&v, c.v)
			*value = append(*value, &v)
		case *GetClientInfoListReply:
			var v GetClientInfoReply
			c.r.value(&v, c.v)
			*value = append(*value, &v)
		case *GetCardInfoListReply:
			var v GetCardInfoReply
			c.r.value(&v, c.v)
			*value = append(*value, &v)
		case *GetSinkInputInfoListReply:
			var v GetSinkInputInfoReply
			c.r.value(&v, c.v)
			*value = append(*value, &v)
		case *GetSourceOutputInfoListReply:
			var v GetSourceOutputInfoReply
			c.r.value(&v, c.v)
			*value = append(*value, &v)
		case *GetSampleInfoListReply:
			var v GetSampleInfoReply
			c.r.value(&v, c.v)
			*value = append(*value, &v)
		default:
			panic("wrong type")
		}
	}
}

// DataPacket is the audio data the server sent for a record stream. Data is
// only valid until the callback returns.
type DataPacket struct {
	StreamIndex uint32
	Data        []byte
}

// ConnectionClosed is the message the client hands to its Callback when the
// server closed the connection.
type ConnectionClosed struct{}

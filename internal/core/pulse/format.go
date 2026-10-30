// Copied from github.com/jfreymuth/pulse v0.1.3 and changed for Radio Tutor. MIT License, Copyright (c) 2019 Johann Freymuth: see internal/core/pulse/LICENSE.

package pulse

import (
	"io"
	"unsafe"

	"github.com/rm4n0s/radio-tutor/internal/core/pulse/proto"
)

// A Reader provides audio data in a specific format.
type Reader interface {
	io.Reader
	Format() byte // Format should return one of the format constants defined by the proto package
}

// A Writer accepts audio data in a specific format.
type Writer interface {
	io.Writer
	Format() byte // Format should return one of the format constants defined by the proto package
}

// Uint8Reader implements the Reader interface.
// The semantics are the same as io.Reader's Read.
type Uint8Reader func([]byte) (int, error)

// Int16Reader implements the Reader interface.
// The semantics are the same as io.Reader's Read, but it returns
// the number of int16 values read, not the number of bytes.
type Int16Reader func([]int16) (int, error)

// Int32Reader implements the Reader interface.
// The semantics are the same as io.Reader's Read, but it returns
// the number of int32 values read, not the number of bytes.
type Int32Reader func([]int32) (int, error)

// Float32Reader implements the Reader interface.
// The semantics are the same as io.Reader's Read, but it returns
// the number of float32 values read, not the number of bytes.
type Float32Reader func([]float32) (int, error)

// NewReader creates a reader from an io.Reader and a format.
// The format must be one of the constants defined in the proto package.
func NewReader(r io.Reader, format byte) Reader {
	check(format)
	return &reader{r, format}
}

// Uint8Writer implements the Writer interface.
// The semantics are the same as io.Writer's Write.
type Uint8Writer func([]byte) (int, error)

// Int16Writer implements the Writer interface.
// The semantics are the same as io.Writer's Write, but it returns
// the number of int16 values written, not the number of bytes.
type Int16Writer func([]int16) (int, error)

// Int32Writer implements the Writer interface.
// The semantics are the same as io.Writer's Write, but it returns
// the number of int32 values written, not the number of bytes.
type Int32Writer func([]int32) (int, error)

// Float32Writer implements the Writer interface.
// The semantics are the same as io.Writer's Write, but it returns
// the number of float32 values written, not the number of bytes.
type Float32Writer func([]float32) (int, error)

// NewWriter creates a writer from an io.Writer and a format.
// The format must be one of the constants defined in the proto package.
func NewWriter(w io.Writer, format byte) Writer {
	check(format)
	return &writer{w, format}
}

// Read calls the function; it returns the number of bytes read.
func (c Uint8Reader) Read(buf []byte) (int, error) { return c(buf) }

// Format returns the sample format of the Uint8Reader.
func (c Uint8Reader) Format() byte { return proto.FormatUint8 }

// Read calls the function; it returns the number of bytes read.
func (c Int16Reader) Read(buf []byte) (int, error) {
	n, err := c(int16Slice(buf))
	return n * 2, err
}

// Format returns the sample format of the Int16Reader.
func (c Int16Reader) Format() byte { return formatI16 }

// Read calls the function; it returns the number of bytes read.
func (c Int32Reader) Read(buf []byte) (int, error) {
	n, err := c(int32Slice(buf))
	return n * 4, err
}

// Format returns the sample format of the Int32Reader.
func (c Int32Reader) Format() byte { return formatI32 }

// Read calls the function; it returns the number of bytes read.
func (c Float32Reader) Read(buf []byte) (int, error) {
	n, err := c(float32Slice(buf))
	return n * 4, err
}

// Format returns the sample format of the Float32Reader.
func (c Float32Reader) Format() byte { return formatF32 }

// Write calls the function; it returns the number of bytes written.
func (c Uint8Writer) Write(buf []byte) (int, error) { return c(buf) }

// Format returns the sample format of the Uint8Writer.
func (c Uint8Writer) Format() byte { return proto.FormatUint8 }

// Write calls the function; it returns the number of bytes written.
func (c Int16Writer) Write(buf []byte) (int, error) {
	n, err := c(int16Slice(buf))
	return n * 2, err
}

// Format returns the sample format of the Int16Writer.
func (c Int16Writer) Format() byte { return formatI16 }

// Write calls the function; it returns the number of bytes written.
func (c Int32Writer) Write(buf []byte) (int, error) {
	n, err := c(int32Slice(buf))
	return n * 4, err
}

// Format returns the sample format of the Int32Writer.
func (c Int32Writer) Format() byte { return formatI32 }

// Write calls the function; it returns the number of bytes written.
func (c Float32Writer) Write(buf []byte) (int, error) {
	n, err := c(float32Slice(buf))
	return n * 4, err
}

// Format returns the sample format of the Float32Writer.
func (c Float32Writer) Format() byte { return formatF32 }

type reader struct {
	r io.Reader
	f byte
}

func (r *reader) Read(buf []byte) (int, error) { return r.r.Read(buf) }
func (r *reader) Format() byte                 { return r.f }

type writer struct {
	w io.Writer
	f byte
}

func (w *writer) Write(buf []byte) (int, error) { return w.w.Write(buf) }
func (w *writer) Format() byte                  { return w.f }

func bytes(f byte) int {
	switch f {
	case proto.FormatUint8:
		return 1
	case proto.FormatInt16LE, proto.FormatInt16BE:
		return 2
	case proto.FormatInt32LE, proto.FormatInt32BE, proto.FormatFloat32LE, proto.FormatFloat32BE:
		return 4
	}
	panic("pulse: invalid format")
}

func check(f byte) {
	switch f {
	case proto.FormatUint8, proto.FormatInt16LE, proto.FormatInt16BE,
		proto.FormatInt32LE, proto.FormatInt32BE, proto.FormatFloat32LE, proto.FormatFloat32BE:
		return
	}
	panic("pulse: invalid format")
}

// The formats of the machine's byte order, used for int16, int32 and float32.
var formatI16, formatI32, formatF32 = nativeFormats()

// nativeFormats returns the format constants for int16, int32 and float32 in
// the byte order of the machine.
func nativeFormats() (i16, i32, f32 byte) {
	i := uint16(1)
	littleEndian := *(*byte)(unsafe.Pointer(&i)) == 1
	if littleEndian {
		return proto.FormatInt16LE, proto.FormatInt32LE, proto.FormatFloat32LE
	}
	return proto.FormatInt16BE, proto.FormatInt32BE, proto.FormatFloat32BE
}

// int16Slice views the bytes as int16 values (a trailing odd byte is left out).
func int16Slice(s []byte) []int16 {
	if len(s) < 2 {
		return nil
	}
	return unsafe.Slice((*int16)(unsafe.Pointer(&s[0])), len(s)/2)
}

// int32Slice views the bytes as int32 values (trailing bytes that do not fill one are left out).
func int32Slice(s []byte) []int32 {
	if len(s) < 4 {
		return nil
	}
	return unsafe.Slice((*int32)(unsafe.Pointer(&s[0])), len(s)/4)
}

// float32Slice views the bytes as float32 values (trailing bytes that do not fill one are left out).
func float32Slice(s []byte) []float32 {
	if len(s) < 4 {
		return nil
	}
	return unsafe.Slice((*float32)(unsafe.Pointer(&s[0])), len(s)/4)
}

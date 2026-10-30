package audio

import (
	"encoding/binary"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/rm4n0s/errors"
	"golang.org/x/sys/windows"
)

// winmm constants (mmeapi.h, mmsystem.h).
const (
	mmWaveMapper    = 0xFFFFFFFF // WAVE_MAPPER: the user's default device
	mmCallbackEvent = 0x00050000 // CALLBACK_EVENT
	mmFormatPCM     = 1          // WAVE_FORMAT_PCM
	mmTimeSamples   = 0x0002     // TIME_SAMPLES
	mmTimeBytes     = 0x0004     // TIME_BYTES
	mmHdrDone       = 0x00000001 // WHDR_DONE
)

// waveFormatEx is WAVEFORMATEX. The C struct is packed to 1 byte and is 18
// bytes; the offsets are the same here, and winmm reads only the 16 bytes of
// PCMWAVEFORMAT for WAVE_FORMAT_PCM, so the size passed to it does not matter.
type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	Size           uint16
}

// waveHdr is WAVEHDR (48 bytes on a 64-bit Windows).
type waveHdr struct {
	Data          uintptr
	BufferLength  uint32
	BytesRecorded uint32
	User          uintptr
	Flags         uint32
	Loops         uint32
	Next          uintptr
	Reserved      uintptr
}

// mmTime is MMTIME (packed: 4 bytes of type and a union of 8).
type mmTime struct {
	Type uint32
	U    [8]byte
}

// hdrStride is the distance between two WAVEHDRs in the buffer memory: a
// multiple of 16, so that every header and the data after them are aligned.
const hdrStride = (int(unsafe.Sizeof(waveHdr{})) + 15) &^ 15

// winmmProcs is the table of the functions of winmm.dll (and of kernel32) the
// sink calls. Nothing else of the system is used. The DLL is loaded from the
// system folder only.
type winmmProcs struct {
	dll             *windows.LazyDLL
	getNumDevs      *windows.LazyProc
	open            *windows.LazyProc
	close           *windows.LazyProc
	prepareHeader   *windows.LazyProc
	unprepareHeader *windows.LazyProc
	write           *windows.LazyProc
	pause           *windows.LazyProc
	restart         *windows.LazyProc
	reset           *windows.LazyProc
	getPosition     *windows.LazyProc
}

// loadWinmm loads the DLL called name (winmm.dll) and finds the functions in
// it. It fails, tagged WaveOutLoadFailed, when the DLL or one of them is
// missing.
func loadWinmm(name string) (*winmmProcs, error) {
	dll := windows.NewLazySystemDLL(name)
	p := &winmmProcs{
		dll:             dll,
		getNumDevs:      dll.NewProc("waveOutGetNumDevs"),
		open:            dll.NewProc("waveOutOpen"),
		close:           dll.NewProc("waveOutClose"),
		prepareHeader:   dll.NewProc("waveOutPrepareHeader"),
		unprepareHeader: dll.NewProc("waveOutUnprepareHeader"),
		write:           dll.NewProc("waveOutWrite"),
		pause:           dll.NewProc("waveOutPause"),
		restart:         dll.NewProc("waveOutRestart"),
		reset:           dll.NewProc("waveOutReset"),
		getPosition:     dll.NewProc("waveOutGetPosition"),
	}
	for _, proc := range []*windows.LazyProc{p.getNumDevs, p.open, p.close, p.prepareHeader, p.unprepareHeader,
		p.write, p.pause, p.restart, p.reset, p.getPosition} {
		if err := proc.Find(); err != nil {
			return nil, errors.NewErr("WaveOutLoadFailed", err).SetMetadata("dll", name, "function", proc.Name)
		}
	}
	return p, nil
}

// call runs a winmm function that returns an MMRESULT and turns a failure into
// a *waveError.
func (p *winmmProcs) call(name string, proc *windows.LazyProc, args ...uintptr) error {
	// The third result of Call is GetLastError, which winmm does not set;
	// the MMRESULT in r1 is the result.
	r1, _, _ := proc.Call(args...)
	if r1 != 0 {
		return &waveError{Func: name, Code: uint32(r1)}
	}
	return nil
}

// winmmBackend is the waveBackend over the real winmm.
type winmmBackend struct {
	p *winmmProcs
}

func (b *winmmBackend) deviceCount() int {
	r1, _, _ := b.p.getNumDevs.Call()
	return int(r1)
}

// open makes the device. Memory that winmm keeps a pointer to (the WAVEHDRs
// and the sample buffers) must not move and must not be collected while the
// device holds it. Go's heap does not move, but the collector frees what
// nothing in Go points to, and a pointer kept only in winmm (as a uintptr in a
// WAVEHDR) does not count; pinning (runtime.Pinner) would need a pin per
// buffer for the life of the device and would still leave the headers in Go
// memory that winmm writes to behind the race detector's back. So everything
// is in one block from VirtualAlloc, which is outside the Go heap: the
// collector never looks at it or frees it, it never moves, and it is released
// only after the device is closed and winmm has given every buffer back.
func (b *winmmBackend) open(f Format, buffers, bufBytes int) (waveDevice, error) {
	d := &winmmDevice{p: b.p, n: buffers, bufBytes: bufBytes, bytesPerFrame: 2 * f.Channels, rate: f.SampleRate}
	ev, err := windows.CreateEvent(nil, 0, 0, nil) // auto-reset, not signalled
	if err != nil {
		return nil, &waveError{Func: "CreateEvent", Code: uint32(errnoOf(err))}
	}
	d.event = ev
	d.memSize = uintptr(buffers * (hdrStride + bufBytes))
	d.mem, err = windows.VirtualAlloc(0, d.memSize, windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	if err != nil {
		_ = windows.CloseHandle(ev)
		return nil, &waveError{Func: "VirtualAlloc", Code: uint32(errnoOf(err))}
	}
	base := pointerOf(d.mem)
	for i := 0; i < buffers; i++ {
		hdr := (*waveHdr)(unsafe.Add(base, i*hdrStride))
		data := unsafe.Add(base, buffers*hdrStride+i*bufBytes)
		hdr.Data = uintptr(data)
		hdr.BufferLength = uint32(bufBytes)
		d.hdrs = append(d.hdrs, hdr)
		d.bufs = append(d.bufs, unsafe.Slice((*byte)(data), bufBytes))
	}

	wfx := waveFormatEx{
		FormatTag:      mmFormatPCM,
		Channels:       uint16(f.Channels),
		SamplesPerSec:  uint32(f.SampleRate),
		AvgBytesPerSec: uint32(f.SampleRate * d.bytesPerFrame),
		BlockAlign:     uint16(d.bytesPerFrame),
		BitsPerSample:  16,
	}
	var h uintptr
	if err := d.p.call("waveOutOpen", d.p.open, uintptr(unsafe.Pointer(&h)), mmWaveMapper,
		uintptr(unsafe.Pointer(&wfx)), uintptr(ev), 0, mmCallbackEvent); err != nil {
		d.free()
		return nil, err
	}
	d.h = h
	for i := range d.hdrs {
		if err := d.p.call("waveOutPrepareHeader", d.p.prepareHeader, d.h, uintptr(unsafe.Pointer(d.hdrs[i])), hdrStride0); err != nil {
			d.prepared = i
			_ = d.close() // the failure that is returned is the first one
			return nil, err
		}
	}
	d.prepared = len(d.hdrs)
	// waveOutOpen starts the device; hold it until the ring is filled.
	if err := d.pause(); err != nil {
		_ = d.close()
		return nil, err
	}
	return d, nil
}

// hdrStride0 is sizeof(WAVEHDR), the size winmm is told.
var hdrStride0 = uintptr(unsafe.Sizeof(waveHdr{}))

// pointerOf turns an address from VirtualAlloc into a pointer. The address is
// not a Go pointer, and the conversion through the address of the variable
// keeps `go vet` from reading it as a misuse of unsafe.Pointer.
func pointerOf(addr uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&addr))
}

// errnoOf returns the Windows error number of err, or 0.
func errnoOf(err error) uintptr {
	if no, ok := err.(windows.Errno); ok { //nolint:errorlint // a plain Errno from a Windows call
		return uintptr(no)
	}
	return 0
}

// winmmDevice is one open waveOut device.
type winmmDevice struct {
	p             *winmmProcs
	h             uintptr // HWAVEOUT
	event         windows.Handle
	mem           uintptr // the block from VirtualAlloc
	memSize       uintptr
	n             int
	bufBytes      int
	bytesPerFrame int
	rate          int
	hdrs          []*waveHdr
	bufs          [][]byte
	prepared      int // how many headers are prepared
}

func (d *winmmDevice) buffer(i int) []byte { return d.bufs[i] }

func (d *winmmDevice) write(i, n int) error {
	d.hdrs[i].BufferLength = uint32(n)
	return d.p.call("waveOutWrite", d.p.write, d.h, uintptr(unsafe.Pointer(d.hdrs[i])), hdrStride0)
}

// done reads the flags that winmm sets from its own thread.
func (d *winmmDevice) done(i int) bool {
	return atomic.LoadUint32(&d.hdrs[i].Flags)&mmHdrDone != 0
}

func (d *winmmDevice) wait(t time.Duration) {
	ms := uint32(max(t.Milliseconds(), 1))
	// The result is not needed: the caller looks at the buffers after any
	// return, signalled or timed out.
	_, _ = windows.WaitForSingleObject(d.event, ms)
}

func (d *winmmDevice) wake() { _ = windows.SetEvent(d.event) }

func (d *winmmDevice) pause() error {
	return d.p.call("waveOutPause", d.p.pause, d.h)
}

func (d *winmmDevice) restart() error {
	return d.p.call("waveOutRestart", d.p.restart, d.h)
}

func (d *winmmDevice) reset() error {
	return d.p.call("waveOutReset", d.p.reset, d.h)
}

// position is waveOutGetPosition in samples (or bytes when the driver only
// counts those). The counter is a DWORD: it wraps after 2^32 frames, 24 hours
// of sound at 48 kHz, far longer than a lesson.
func (d *winmmDevice) position() (int64, bool) {
	t := mmTime{Type: mmTimeSamples}
	if d.p.call("waveOutGetPosition", d.p.getPosition, d.h, uintptr(unsafe.Pointer(&t)), unsafe.Sizeof(t)) != nil {
		return 0, false
	}
	v := int64(binary.LittleEndian.Uint32(t.U[:4]))
	switch t.Type {
	case mmTimeSamples:
		return v, true
	case mmTimeBytes:
		return v / int64(d.bytesPerFrame), true
	}
	return 0, false
}

// unprepareAll unprepares the prepared headers; it returns the first failure.
func (d *winmmDevice) unprepareAll() error {
	var first error
	for i := 0; i < d.prepared; i++ {
		if err := d.p.call("waveOutUnprepareHeader", d.p.unprepareHeader, d.h, uintptr(unsafe.Pointer(d.hdrs[i])), hdrStride0); err != nil && first == nil {
			first = err
		}
	}
	d.prepared = 0
	return first
}

// free releases the memory and the event.
func (d *winmmDevice) free() {
	_ = windows.VirtualFree(d.mem, 0, windows.MEM_RELEASE)
	_ = windows.CloseHandle(d.event)
}

func (d *winmmDevice) close() error {
	if err := d.unprepareAll(); err != nil {
		return err // winmm may still own the buffers: leak them
	}
	if err := d.p.call("waveOutClose", d.p.close, d.h); err != nil {
		return err
	}
	d.free()
	return nil
}

package phone

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"math/rand/v2"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
)

// fakeClock is a clock the tests move by hand.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// testJPEG returns a small JPEG photo.
func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x * 255 / w), uint8(y * 255 / h), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// noisyJPEG returns a JPEG of random pixels, which compresses badly and so is
// large for its size.
func noisyJPEG(t *testing.T, side int) []byte {
	t.Helper()
	rng := rand.New(rand.NewPCG(1, 2))
	img := image.NewRGBA(image.Rect(0, 0, side, side))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = uint8(rng.UintN(256)), uint8(rng.UintN(256)), uint8(rng.UintN(256)), 255
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// testPNG returns a small PNG photo.
func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = uint8(i)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// file is one part of a test upload.
type file struct {
	field string
	name  string
	data  []byte
}

// photos makes n parts named "photos".
func photoParts(data []byte, n int) []file {
	files := make([]file, n)
	for i := range files {
		files[i] = file{"photos", "photo.jpg", data}
	}
	return files
}

// multipartBody builds a multipart form body.
func multipartBody(t *testing.T, files []file) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, f := range files {
		h := textproto.MIMEHeader{}
		disp := `form-data; name="` + f.field + `"`
		if f.name != "" {
			disp += `; filename="` + f.name + `"`
		}
		h.Set("Content-Disposition", disp)
		h.Set("Content-Type", "application/octet-stream")
		pw, err := mw.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pw.Write(f.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

// events records the callbacks of a token.
type events struct {
	mu       sync.Mutex
	opened   int
	started  int
	received [][][]byte
	// rejectWith, when not nil, is returned by Received.
	rejectWith error
}

func (e *events) handlers() Handlers {
	return Handlers{
		Opened: func() {
			e.mu.Lock()
			e.opened++
			e.mu.Unlock()
		},
		Started: func() {
			e.mu.Lock()
			e.started++
			e.mu.Unlock()
		},
		Received: func(ctx context.Context, photos [][]byte) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.rejectWith != nil {
				return e.rejectWith
			}
			e.received = append(e.received, photos)
			return nil
		},
	}
}

func (e *events) counts() (opened, started, received int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.opened, e.started, len(e.received)
}

// testServer is a running server on 127.0.0.1 with a fake clock.
type testServer struct {
	*Server
	clock *fakeClock
	base  string

	mu     sync.Mutex
	offers map[string]*Offer // the offer made by open, by link token
	links  map[string]*Link
}

func startServer(t *testing.T, opts Options) *testServer {
	t.Helper()
	clock := newFakeClock()
	if opts.Clock == nil {
		opts.Clock = clock.Now
	}
	opts.ListenHost = "127.0.0.1"
	s := New(opts)
	ctx, cancel := context.WithCancel(context.Background())
	if err := s.Start(ctx, 0); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		stopCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := s.Stop(stopCtx); err != nil {
			t.Errorf("Stop: %v", err)
		}
	})
	return &testServer{
		Server: s, clock: clock, base: "http://127.0.0.1:" + strconv.Itoa(s.Port()),
		offers: map[string]*Offer{}, links: map[string]*Link{},
	}
}

// testPage is the page the tests offer.
var testPage = PageData{Title: "Cassette", Exercises: []PageExercise{{Label: "1", Title: "Add", Prompt: "2+2"}}}

// newLink makes a link.
func (ts *testServer) newLink(t *testing.T) *Link {
	t.Helper()
	k, err := ts.OpenLink("127.0.0.1", i18n.EN)
	if err != nil {
		t.Fatal(err)
	}
	ts.mu.Lock()
	ts.links[k.Token] = k
	ts.mu.Unlock()
	return k
}

// offer offers testPage on a link and remembers the offer for upload.
func (ts *testServer) offer(t *testing.T, k *Link, h Handlers) *Offer {
	t.Helper()
	o, _, err := k.Offer(testPage, h)
	if err != nil {
		t.Fatal(err)
	}
	ts.mu.Lock()
	ts.offers[k.Token] = o
	ts.mu.Unlock()
	return o
}

// open makes a link with an offer of a simple page and returns its token.
func (ts *testServer) open(t *testing.T, h Handlers) (token string) {
	t.Helper()
	k := ts.newLink(t)
	ts.offer(t, k, h)
	return k.Token
}

// get fetches a path and returns the status and the body.
func (ts *testServer) get(t *testing.T, path string) (int, string, http.Header) {
	t.Helper()
	resp, err := http.Get(ts.base + path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(body), resp.Header
}

// upload posts a multipart form to the photos path of the link, for the offer
// that open or offer made last (offer 1 when there is none).
func (ts *testServer) upload(t *testing.T, token string, files []file) (int, string) {
	t.Helper()
	ts.mu.Lock()
	id := uint64(1)
	if o := ts.offers[token]; o != nil {
		id = o.ID()
	}
	ts.mu.Unlock()
	return ts.uploadOffer(t, token, id, files)
}

// uploadOffer posts a multipart form for the named offer.
func (ts *testServer) uploadOffer(t *testing.T, token string, offer uint64, files []file) (int, string) {
	t.Helper()
	body, ct := multipartBody(t, files)
	return ts.post(t, "/p/"+token+"/photos?offer="+strconv.FormatUint(offer, 10), ct, body)
}

func (ts *testServer) post(t *testing.T, path, contentType string, body io.Reader) (int, string) {
	t.Helper()
	resp, err := http.Post(ts.base+path, contentType, body)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(out)
}

// wantRoute fails unless err is a package error whose route has the part.
func wantRoute(t *testing.T, err error, route string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error with route %q, got nil", route)
	}
	appErr, ok := errors.FromError(err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T: %v", err, err)
	}
	if !appErr.HasRoute(route) {
		t.Errorf("unexpected failure path: %s (want %s)", appErr.Route(), route)
	}
}

// bufCopy copies r into a strings.Builder.
func bufCopy(w *strings.Builder, r io.Reader) (int64, error) { return io.Copy(w, r) }

// waitFor polls cond until it holds, or fails the test after five seconds.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("the condition did not come true")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// contextWithCancel returns a context that is cancelled when the test ends.
func contextWithCancel(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx, cancel
}

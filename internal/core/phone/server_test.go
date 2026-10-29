package phone

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/jpeg"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
)

func TestPageRendersAndEscapes(t *testing.T) {
	ts := startServer(t, Options{})
	ev := &events{}
	page := PageData{
		Lang:  i18n.EN,
		Title: `Maths <script>alert("t")</script>`,
		Exercises: []PageExercise{
			{Label: `A&B`, Title: `Sum <b>bold</b>`, Prompt: "Line one\nLine <i>two</i>\r\nLine three"},
			{Label: "2", Title: "No prompt", Prompt: ""},
		},
	}
	token, url, err := ts.Open("192.168.1.20", page, ev.handlers())
	if err != nil {
		t.Fatal(err)
	}
	wantURL := "http://192.168.1.20:" + strconv.Itoa(ts.Port()) + "/s/" + token
	if url != wantURL {
		t.Errorf("url = %q, want %q", url, wantURL)
	}

	status, body, hdr := ts.get(t, "/s/"+token)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body %s", status, body)
	}
	if ct := hdr.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q", ct)
	}
	for _, want := range []string{
		`<html lang="en">`,
		`capture="environment"`, `accept="image/*"`, ` multiple>`,
		`Maths &lt;script&gt;alert(&#34;t&#34;)&lt;/script&gt;`,
		`A&amp;B`,
		`Sum &lt;b&gt;bold&lt;/b&gt;`,
		`Line one<br>Line &lt;i&gt;two&lt;/i&gt;<br>Line three`,
		`Take photos of the solution`,
		`Write the number of each exercise next to its solution`,
		`No prompt`,
		`/static/page.css`, `/static/page.js`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	for _, bad := range []string{`<script>alert`, `<b>bold</b>`, `<i>two</i>`, "http://", "https://"} {
		if strings.Contains(body, bad) {
			t.Errorf("page contains %q", bad)
		}
	}
	// The texts for the script are valid JSON with the phone messages.
	start := strings.Index(body, `id="text">`)
	if start < 0 {
		t.Fatal("no text block")
	}
	rest := body[start+len(`id="text">`):]
	end := strings.Index(rest, "</script>")
	if end < 0 {
		t.Fatal("text block is not closed")
	}
	var text map[string]string
	if err := json.Unmarshal([]byte(rest[:end]), &text); err != nil {
		t.Fatalf("text block is not JSON: %v", err)
	}
	if text["phone.take"] != "Take photos of the solution" || text["phone.sent"] != "Sent! Look at the computer." {
		t.Errorf("text block = %v", text)
	}
	if opened, _, _ := ev.counts(); opened != 1 {
		t.Errorf("Opened called %d times, want 1", opened)
	}
	// A page may be opened again (a reload) and the event follows.
	ts.get(t, "/s/"+token)
	if opened, _, _ := ev.counts(); opened != 2 {
		t.Errorf("Opened called %d times, want 2", opened)
	}
}

func TestPageInGreek(t *testing.T) {
	ts := startServer(t, Options{})
	token, _, err := ts.Open("10.0.0.2", PageData{Lang: i18n.EL, Title: "Μαθηματικά", Exercises: []PageExercise{{Label: "1", Title: "Πρόσθεση"}}}, Handlers{})
	if err != nil {
		t.Fatal(err)
	}
	_, body, _ := ts.get(t, "/s/"+token)
	for _, want := range []string{`<html lang="el">`, "Τράβηξε φωτογραφίες της λύσης", "Μαθηματικά", "Πρόσθεση"} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
}

func TestStaticFiles(t *testing.T) {
	ts := startServer(t, Options{})
	for _, tc := range []struct {
		path        string
		status      int
		contentType string
		contains    string
	}{
		{"/static/page.css", 200, "text/css", ".big-button"},
		{"/static/page.js", 200, "text/javascript", "XMLHttpRequest"},
		{"/static/other.js", 404, "text/html", "Page not found"},
		{"/static/", 404, "text/html", "Page not found"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			status, body, hdr := ts.get(t, tc.path)
			if status != tc.status {
				t.Errorf("status = %d, want %d", status, tc.status)
			}
			if !strings.HasPrefix(hdr.Get("Content-Type"), tc.contentType) {
				t.Errorf("Content-Type = %q", hdr.Get("Content-Type"))
			}
			if !strings.Contains(body, tc.contains) {
				t.Errorf("body does not contain %q", tc.contains)
			}
			if hdr.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(hdr.Get("Content-Security-Policy"), "default-src 'none'") {
				t.Errorf("security headers missing: %v", hdr)
			}
		})
	}
}

func TestTokenFormat(t *testing.T) {
	ts := startServer(t, Options{})
	seen := map[string]bool{}
	for range 20 {
		token := ts.open(t, Handlers{})
		if len(token) != 26 || strings.Trim(token, "abcdefghijklmnopqrstuvwxyz234567") != "" {
			t.Errorf("token %q is not 16 bytes of lowercase base32 without padding", token)
		}
		if seen[token] {
			t.Errorf("token %q repeated", token)
		}
		seen[token] = true
	}
}

func TestUnknownExpiredAndRevokedTokens(t *testing.T) {
	ts := startServer(t, Options{TokenTTL: time.Minute})
	expiring := ts.open(t, Handlers{})
	revoked := ts.open(t, Handlers{})
	ts.Close(revoked)
	ts.Close("does-not-exist") // does nothing
	ts.clock.Advance(2 * time.Minute)
	fresh := ts.open(t, Handlers{})

	tests := []struct {
		name  string
		token string
		gone  bool
	}{
		{"unknown", "nosuchtoken", true},
		{"expired", expiring, true},
		{"revoked", revoked, true},
		{"fresh", fresh, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, body, _ := ts.get(t, "/s/"+tc.token)
			if tc.gone {
				if status != http.StatusGone || !strings.Contains(body, "This link has expired") {
					t.Errorf("GET = %d %q, want 410 with the expired message", status, body)
				}
				status, body = ts.upload(t, tc.token, photoParts(testJPEG(t, 20, 20), 1))
				if status != http.StatusGone || !strings.Contains(body, `"expired":true`) {
					t.Errorf("POST = %d %q, want 410 expired", status, body)
				}
			} else if status != http.StatusOK {
				t.Errorf("GET = %d, want 200", status)
			}
		})
	}
}

func TestExpiryIsChecked(t *testing.T) {
	// The page is fine just before the TTL and gone at the TTL.
	ts := startServer(t, Options{TokenTTL: 30 * time.Minute})
	token := ts.open(t, Handlers{})
	ts.clock.Advance(30*time.Minute - time.Second)
	if status, _, _ := ts.get(t, "/s/"+token); status != http.StatusOK {
		t.Errorf("before the TTL: %d", status)
	}
	ts.clock.Advance(time.Second)
	if status, _, _ := ts.get(t, "/s/"+token); status != http.StatusGone {
		t.Errorf("at the TTL: %d", status)
	}
}

func TestOtherPathsAre404(t *testing.T) {
	ts := startServer(t, Options{})
	token := ts.open(t, Handlers{})
	for _, path := range []string{"/", "/index.html", "/s/", "/s", "/s/" + token + "/", "/s/" + token + "/other", "/admin", "/static"} {
		t.Run(path, func(t *testing.T) {
			status, body, _ := ts.get(t, path)
			if status != http.StatusNotFound || !strings.Contains(body, "Page not found") {
				t.Errorf("GET %s = %d %q", path, status, body)
			}
		})
	}
	// A wrong method is 404 too.
	if status, _ := ts.post(t, "/s/"+token, "text/plain", strings.NewReader("x")); status != http.StatusNotFound {
		t.Errorf("POST on the page = %d", status)
	}
	// The generic pages follow the server's language.
	ts.SetLang(i18n.EL)
	ts.SetLang("xx") // ignored
	if _, body, _ := ts.get(t, "/nothing"); !strings.Contains(body, "Η σελίδα δεν βρέθηκε") {
		t.Errorf("404 page is not in Greek: %s", body)
	}
}

func TestUploadOneAndSix(t *testing.T) {
	for _, n := range []int{1, 6} {
		t.Run(strconv.Itoa(n)+" photos", func(t *testing.T) {
			ts := startServer(t, Options{})
			ev := &events{}
			token := ts.open(t, ev.handlers())
			var files []file
			for i := range n {
				if i%2 == 0 {
					files = append(files, file{"photos", "p.jpg", testJPEG(t, 40+i, 30)})
				} else {
					files = append(files, file{"photos", "p.png", testPNG(t, 30, 40+i)})
				}
			}
			files = append(files, file{"note", "", []byte("ignored field")})
			status, body := ts.upload(t, token, files)
			if status != http.StatusOK || !strings.Contains(body, `"ok":true`) {
				t.Fatalf("upload = %d %s", status, body)
			}
			_, started, received := ev.counts()
			if started != 1 || received != 1 {
				t.Fatalf("started %d, received %d", started, received)
			}
			got := ev.received[0]
			if len(got) != n {
				t.Fatalf("got %d photos, want %d", len(got), n)
			}
			for i, photo := range got {
				cfg, err := jpeg.DecodeConfig(bytes.NewReader(photo))
				if err != nil {
					t.Fatalf("photo %d is not a JPEG: %v", i, err)
				}
				// The order is kept: the sizes tell the photos apart.
				wantW, wantH := 40+i, 30
				if i%2 == 1 {
					wantW, wantH = 30, 40+i
				}
				if cfg.Width != wantW || cfg.Height != wantH {
					t.Errorf("photo %d is %dx%d, want %dx%d", i, cfg.Width, cfg.Height, wantW, wantH)
				}
			}
			// The token is single use.
			if status, _, _ := ts.get(t, "/s/"+token); status != http.StatusGone {
				t.Errorf("GET after the upload = %d, want 410", status)
			}
			status, _ = ts.upload(t, token, photoParts(testJPEG(t, 20, 20), 1))
			if status != http.StatusGone {
				t.Errorf("second upload = %d, want 410", status)
			}
			if _, _, received := ev.counts(); received != 1 {
				t.Errorf("Received called %d times", received)
			}
		})
	}
}

func TestLargePhotoIsScaled(t *testing.T) {
	ts := startServer(t, Options{})
	ev := &events{}
	token := ts.open(t, ev.handlers())
	status, body := ts.upload(t, token, photoParts(testJPEG(t, 3000, 1000), 1))
	if status != http.StatusOK {
		t.Fatalf("upload = %d %s", status, body)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(ev.received[0][0]))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 2048 {
		t.Errorf("width = %d, want 2048 (NormalizePhoto ran)", cfg.Width)
	}
}

func TestRefusedUploads(t *testing.T) {
	jpg := testJPEG(t, 40, 30)
	noisy := noisyJPEG(t, 64)
	if len(noisy) < 3000 {
		t.Fatalf("noisy photo is only %d bytes", len(noisy))
	}
	gif := []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;")
	tests := []struct {
		name    string
		opts    Options
		files   []file
		status  int
		message string
	}{
		{"seven photos", Options{}, photoParts(jpg, 7), 413, "at most 6 photos"},
		{"a photo too big", Options{MaxPhotoBytes: 2000}, []file{{"photos", "a.jpg", jpg}, {"photos", "b.jpg", noisy}}, 413, "This photo is too big"},
		{"too big together", Options{MaxPhotoBytes: 100_000, MaxTotalBytes: 2 * int64(len(jpg))}, photoParts(jpg, 3), 413, "too big all together"},
		{"a text file", Options{}, []file{{"photos", "notes.jpg", []byte("hello, I am not a photo")}}, 415, "Only photos can be sent"},
		{"a GIF", Options{}, []file{{"photos", "x.gif", gif}}, 415, "Only photos can be sent"},
		{"an empty file", Options{}, []file{{"photos", "x.jpg", nil}}, 415, "Only photos can be sent"},
		{"a truncated JPEG", Options{}, []file{{"photos", "x.jpg", jpg[:len(jpg)/3]}}, 422, "could not be read"},
		{"a PNG signature and garbage", Options{}, []file{{"photos", "x.png", append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)}}, 422, "could not be read"},
		{"no photo", Options{}, []file{{"note", "", []byte("hi")}}, 400, "Take at least one photo"},
		{"a file in another field", Options{}, []file{{"other", "x.jpg", jpg}}, 400, "Take at least one photo"},
		{"nothing at all", Options{}, nil, 400, "Take at least one photo"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := startServer(t, tc.opts)
			ev := &events{}
			token := ts.open(t, ev.handlers())
			status, body := ts.upload(t, token, tc.files)
			if status != tc.status {
				t.Fatalf("status = %d (%s), want %d", status, body, tc.status)
			}
			if !strings.Contains(body, tc.message) {
				t.Errorf("body %q does not contain %q", body, tc.message)
			}
			if _, _, received := ev.counts(); received != 0 {
				t.Errorf("Received called after a refused upload")
			}
			// A refused upload does not use the token up: the child tries again.
			status, body = ts.upload(t, token, photoParts(jpg, 1))
			if status != http.StatusOK {
				t.Errorf("retry = %d %s, want 200", status, body)
			}
		})
	}
}

func TestUploadNotMultipart(t *testing.T) {
	ts := startServer(t, Options{})
	token := ts.open(t, Handlers{})
	status, _ := ts.post(t, "/s/"+token+"/photos", "text/plain", strings.NewReader("just text"))
	if status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", status)
	}
}

func TestUploadBodyAboveTheLimit(t *testing.T) {
	// The body itself is above the limit (the files may be fine one by one).
	ts := startServer(t, Options{MaxTotalBytes: 1000})
	token := ts.open(t, Handlers{})
	big := append(testJPEG(t, 40, 30), bytes.Repeat([]byte{0}, 2<<20)...)
	body, ct := multipartBody(t, []file{{"photos", "a.jpg", big}})
	req, err := http.NewRequest(http.MethodPost, ts.base+"/s/"+token+"/photos", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", ct)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// The server may hang up while the client is still sending.
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", resp.StatusCode)
	}
}

func TestReceivedCanRefuse(t *testing.T) {
	ts := startServer(t, Options{})
	ev := &events{rejectWith: errors.New("SubmissionCancelled", "cancelled")}
	token := ts.open(t, ev.handlers())
	status, body := ts.upload(t, token, photoParts(testJPEG(t, 20, 20), 1))
	if status != http.StatusGone || !strings.Contains(body, `"expired":true`) || !strings.Contains(body, "This link has expired") {
		t.Errorf("upload = %d %s, want 410 expired", status, body)
	}
	if status, _, _ := ts.get(t, "/s/"+token); status != http.StatusGone {
		t.Errorf("GET after a refusal = %d, want 410", status)
	}
}

func TestCloseDuringUpload(t *testing.T) {
	// The token is revoked while the upload is read: the page gets "expired"
	// and Received is not called.
	ts := startServer(t, Options{})
	started := make(chan struct{})
	release := make(chan struct{})
	received := false
	token := ts.open(t, Handlers{
		Started: func() { close(started); <-release },
		Received: func(context.Context, [][]byte) error {
			received = true
			return nil
		},
	})
	done := make(chan int)
	go func() {
		status, _ := ts.upload(t, token, photoParts(testJPEG(t, 20, 20), 1))
		done <- status
	}()
	<-started
	ts.Close(token)
	close(release)
	if status := <-done; status != http.StatusGone {
		t.Errorf("status = %d, want 410", status)
	}
	if received {
		t.Error("Received was called for a revoked token")
	}
}

func TestSecondUploadWhileReceiving(t *testing.T) {
	ts := startServer(t, Options{})
	started := make(chan struct{})
	release := make(chan struct{})
	var once bool
	token := ts.open(t, Handlers{Started: func() {
		if !once {
			once = true
			close(started)
			<-release
		}
	}})
	done := make(chan int)
	go func() {
		status, _ := ts.upload(t, token, photoParts(testJPEG(t, 20, 20), 1))
		done <- status
	}()
	<-started
	status, body := ts.upload(t, token, photoParts(testJPEG(t, 20, 20), 1))
	if status != http.StatusConflict || !strings.Contains(body, "already") {
		t.Errorf("second upload = %d %s, want 409", status, body)
	}
	close(release)
	if status := <-done; status != http.StatusOK {
		t.Errorf("first upload = %d, want 200", status)
	}
}

func TestRateLimit(t *testing.T) {
	ts := startServer(t, Options{RequestsPerMinute: 3})
	token := ts.open(t, Handlers{})
	for i := range 3 {
		if status, _, _ := ts.get(t, "/s/"+token); status != http.StatusOK {
			t.Fatalf("request %d = %d", i+1, status)
		}
	}
	status, body, hdr := ts.get(t, "/s/"+token)
	if status != http.StatusTooManyRequests || !strings.Contains(body, "Too many tries") {
		t.Errorf("fourth request = %d %q, want 429", status, body)
	}
	if secs, err := strconv.Atoi(hdr.Get("Retry-After")); err != nil || secs < 1 || secs > 21 {
		t.Errorf("Retry-After = %q", hdr.Get("Retry-After"))
	}
	// The photos path answers with JSON.
	status, body = ts.upload(t, token, photoParts(testJPEG(t, 20, 20), 1))
	if status != http.StatusTooManyRequests || !strings.Contains(body, `"error"`) {
		t.Errorf("upload when limited = %d %s", status, body)
	}
	// A request is allowed again after a refill (3 per minute is 1 per 20 s).
	ts.clock.Advance(21 * time.Second)
	if status, _, _ := ts.get(t, "/s/"+token); status != http.StatusOK {
		t.Errorf("after the wait = %d, want 200", status)
	}
}

func TestRateLimitIsPerAddress(t *testing.T) {
	clock := newFakeClock()
	s := New(Options{Clock: clock.Now, RequestsPerMinute: 2})
	h := s.guard(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	do := func(remote string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	for range 2 {
		do("192.168.1.5:5000")
	}
	if got := do("192.168.1.5:6000"); got != http.StatusTooManyRequests {
		t.Errorf("same address, other port = %d, want 429", got)
	}
	if got := do("192.168.1.6:5000"); got != http.StatusOK {
		t.Errorf("other address = %d, want 200", got)
	}
	if got := do("no-port"); got != http.StatusOK {
		t.Errorf("address without a port = %d, want 200", got)
	}
}

func TestOpenErrors(t *testing.T) {
	s := New(Options{})
	_, _, err := s.Open("10.0.0.1", PageData{}, Handlers{})
	wantRoute(t, err, "Server.Open.PhoneNotRunning")

	ts := startServer(t, Options{Random: strings.NewReader("short")})
	_, _, err = ts.Open("10.0.0.1", PageData{}, Handlers{})
	wantRoute(t, err, "Server.Open.PhoneTokenFailed")
}

func TestOpenDropsExpiredTokens(t *testing.T) {
	ts := startServer(t, Options{TokenTTL: time.Minute})
	for range 5 {
		ts.open(t, Handlers{})
	}
	ts.clock.Advance(2 * time.Minute)
	ts.open(t, Handlers{})
	ts.mu.Lock()
	n := len(ts.tokens)
	ts.mu.Unlock()
	if n != 1 {
		t.Errorf("%d tokens are kept, want 1", n)
	}
}

func TestStartErrors(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = occupied.Close() }()
	port := occupied.Addr().(*net.TCPAddr).Port

	t.Run("port in use", func(t *testing.T) {
		s := New(Options{ListenHost: "127.0.0.1"})
		err := s.Start(context.Background(), port)
		wantRoute(t, err, "Server.Start.PhonePortInUse")
		appErr, _ := errors.FromError(err)
		if appErr.Metadata["port"] != port {
			t.Errorf("port metadata = %v, want %d", appErr.Metadata["port"], port)
		}
		if s.Running() {
			t.Error("server runs after a failed start")
		}
	})
	t.Run("cannot listen", func(t *testing.T) {
		s := New(Options{ListenHost: "127.0.0.1"})
		wantRoute(t, s.Start(context.Background(), -5), "Server.Start.PhoneListenFailed")
	})
	t.Run("started twice", func(t *testing.T) {
		ts := startServer(t, Options{})
		wantRoute(t, ts.Start(context.Background(), 0), "Server.Start.PhoneAlreadyStarted")
	})
}

func TestStopAndRestart(t *testing.T) {
	s := New(Options{ListenHost: "127.0.0.1"})
	if err := s.Stop(context.Background()); err != nil {
		t.Errorf("Stop on a stopped server: %v", err)
	}
	if err := s.Start(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	port := s.Port()
	if port == 0 || !s.Running() {
		t.Fatal("not running")
	}
	token, _, err := s.Open("127.0.0.1", PageData{Title: "x"}, Handlers{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.Running() || s.Port() != 0 {
		t.Error("still running after Stop")
	}
	if conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), time.Second); err == nil {
		_ = conn.Close()
		t.Error("the port still accepts connections")
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Errorf("second Stop: %v", err)
	}
	// Tokens do not survive a stop.
	if err := s.Start(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Stop(context.Background()) }()
	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(s.Port()) + "/s/" + token)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusGone {
		t.Errorf("old token after a restart = %d, want 410", resp.StatusCode)
	}
}

func TestContextCancelStopsTheServer(t *testing.T) {
	s := New(Options{ListenHost: "127.0.0.1"})
	ctx, cancel := context.WithCancel(context.Background())
	if err := s.Start(ctx, 0); err != nil {
		t.Fatal(err)
	}
	cancel()
	deadline := time.Now().Add(5 * time.Second)
	for s.Running() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if s.Running() {
		t.Error("the server still runs after its context was cancelled")
	}
}

func TestStopFailsWhenARequestOutlivesTheGrace(t *testing.T) {
	s := New(Options{ListenHost: "127.0.0.1"})
	if err := s.Start(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	inHandler := make(chan struct{})
	release := make(chan struct{})
	token, _, err := s.Open("127.0.0.1", PageData{}, Handlers{Opened: func() {
		close(inHandler)
		<-release
	}})
	if err != nil {
		t.Fatal(err)
	}
	url := "http://127.0.0.1:" + strconv.Itoa(s.Port()) + "/s/" + token
	go func() {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-inHandler
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	wantRoute(t, s.Stop(ctx), "Server.stop.PhoneStopFailed")
	close(release)
}

func TestUploadStatus(t *testing.T) {
	tests := []struct {
		tag    string
		status int
	}{
		{"PhoneUploadMalformed", 400},
		{"PhoneUploadEmpty", 400},
		{"PhoneUploadTooMany", 413},
		{"PhoneUploadPhotoTooBig", 413},
		{"PhoneUploadTotalTooBig", 413},
		{"PhotoTooLarge", 413},
		{"PhoneUploadNotPhoto", 415},
		{"PhotoUnsupported", 415},
		{"PhotoUndecodable", 422},
		{"PhotoDecodeFailed", 422},
		{"PhotoEncodeFailed", 500},
		{"PhotoCanceled", 500},
		{"SomethingElse", 500},
	}
	for _, tc := range tests {
		t.Run(tc.tag, func(t *testing.T) {
			if got, _ := uploadStatus(errors.New(tc.tag, "x")); got != tc.status {
				t.Errorf("status = %d, want %d", got, tc.status)
			}
		})
	}
	if got, _ := uploadStatus(context.Canceled); got != 500 {
		t.Errorf("a plain error = %d, want 500", got)
	}
}

// pngChunk builds a PNG chunk with a valid checksum.
func pngChunk(kind string, data []byte) []byte {
	out := binary.BigEndian.AppendUint32(nil, uint32(len(data)))
	out = append(out, kind...)
	out = append(out, data...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(append([]byte(kind), data...)))
}

func TestTooManyPixelsIs413(t *testing.T) {
	// A PNG whose header claims 20000x20000 pixels is refused before decoding.
	ihdr := binary.BigEndian.AppendUint32(nil, 20000)
	ihdr = binary.BigEndian.AppendUint32(ihdr, 20000)
	ihdr = append(ihdr, 8, 0, 0, 0, 0) // 8 bits, gray
	data := append([]byte("\x89PNG\r\n\x1a\n"), pngChunk("IHDR", ihdr)...)
	data = append(data, pngChunk("IDAT", []byte{0})...)
	ts := startServer(t, Options{})
	token := ts.open(t, Handlers{})
	status, body := ts.upload(t, token, []file{{"photos", "x.png", data}})
	if status != http.StatusRequestEntityTooLarge || !strings.Contains(body, "too big") {
		t.Errorf("status = %d %s, want 413", status, body)
	}
}

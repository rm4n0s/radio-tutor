package phone

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// request builds an upload request for receive.
func request(t *testing.T, files []file) *http.Request {
	t.Helper()
	body, ct := multipartBody(t, files)
	req := httptest.NewRequest(http.MethodPost, "/s/x/photos", body)
	req.Header.Set("Content-Type", ct)
	return req
}

func TestReceiveRoutes(t *testing.T) {
	jpg := testJPEG(t, 30, 30)
	tests := []struct {
		name  string
		opts  Options
		req   func(t *testing.T) *http.Request
		route string
	}{
		{"not multipart", Options{}, func(t *testing.T) *http.Request {
			req := httptest.NewRequest(http.MethodPost, "/s/x/photos", strings.NewReader("x"))
			req.Header.Set("Content-Type", "text/plain")
			return req
		}, "Server.receive.PhoneUploadMalformed"},
		{"truncated multipart", Options{}, func(t *testing.T) *http.Request {
			body, ct := multipartBody(t, photoParts(jpg, 1))
			buf := new(strings.Builder)
			if _, err := bufCopy(buf, body); err != nil {
				t.Fatal(err)
			}
			cut := buf.String()[:len(buf.String())/2]
			req := httptest.NewRequest(http.MethodPost, "/s/x/photos", strings.NewReader(cut))
			req.Header.Set("Content-Type", ct)
			return req
		}, "PhoneUploadMalformed"},
		{"no files", Options{}, func(t *testing.T) *http.Request {
			return request(t, []file{{"note", "", []byte("x")}})
		}, "Server.receive.PhoneUploadEmpty"},
		{"too many", Options{}, func(t *testing.T) *http.Request {
			return request(t, photoParts(jpg, MaxPhotos+1))
		}, "Server.receive.PhoneUploadTooMany"},
		{"photo too big", Options{MaxPhotoBytes: 10}, func(t *testing.T) *http.Request {
			return request(t, photoParts(jpg, 1))
		}, "readPart.PhoneUploadPhotoTooBig"},
		{"total too big", Options{MaxPhotoBytes: 1 << 20, MaxTotalBytes: int64(len(jpg)) + 1}, func(t *testing.T) *http.Request {
			return request(t, photoParts(jpg, 2))
		}, "Server.receive.PhoneUploadTotalTooBig"},
		{"not a photo", Options{}, func(t *testing.T) *http.Request {
			return request(t, []file{{"photos", "a.jpg", []byte("<html></html>")}})
		}, "readPart.PhoneUploadNotPhoto"},
		{"undecodable", Options{}, func(t *testing.T) *http.Request {
			return request(t, []file{{"photos", "a.jpg", jpg[:20]}})
		}, "Server.receive->NormalizePhoto->normalizePhoto.PhotoUndecodable"},
		{"body above the limit, in a part", Options{}, func(t *testing.T) *http.Request {
			req := request(t, photoParts(noisyJPEG(t, 64), 1))
			req.Body = http.MaxBytesReader(nil, req.Body, 5000)
			return req
		}, "readPart.PhoneUploadTotalTooBig"},
		{"body above the limit, between parts", Options{}, func(t *testing.T) *http.Request {
			req := request(t, photoParts(jpg, 1))
			req.Body = http.MaxBytesReader(nil, req.Body, 10)
			return req
		}, "Server.receive.PhoneUploadTotalTooBig"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(tc.opts)
			photos, err := s.receive(tc.req(t))
			if photos != nil {
				t.Errorf("got %d photos with an error", len(photos))
			}
			wantRoute(t, err, tc.route)
		})
	}
}

func TestReceiveAccepts(t *testing.T) {
	s := New(Options{})
	photos, err := s.receive(request(t, []file{{"photos", "a.jpg", testJPEG(t, 30, 30)}, {"photos", "b.png", testPNG(t, 30, 30)}}))
	if err != nil || len(photos) != 2 {
		t.Fatalf("photos = %d, err = %v", len(photos), err)
	}
}

package phone

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
)

// claim marks the offer as receiving an upload. It returns the link and the
// offer, or why it cannot: gone is true for an unknown or ended link, stale for
// an offer that is not the waiting one (or no offer waiting), busy for an offer
// that is receiving another upload.
func (s *Server) claim(token string, offerID uint64) (l *link, o *offer, gone, stale, busy bool) {
	l = s.findLink(token)
	if l == nil {
		return nil, nil, true, false, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	switch {
	case l.ended:
		return nil, nil, true, false, false
	case l.cur == nil || l.cur.id != offerID:
		return nil, nil, false, true, false
	case l.cur.busy:
		return nil, nil, false, false, true
	}
	l.cur.busy = true
	return l, l.cur, false, false, false
}

// release lets the offer receive another upload, after a refused one.
func (s *Server) release(l *link, o *offer) {
	l.mu.Lock()
	o.busy = false
	l.mu.Unlock()
}

// finish ends the offer when it is still the waiting one: the page goes back to
// waiting. It returns false when the link ended or the offer was withdrawn or
// replaced meanwhile.
func (s *Server) finish(l *link, o *offer) bool {
	now := s.clock()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.ended || l.cur != o {
		return false
	}
	l.cur = nil
	l.lastUse = now
	l.bump()
	return true
}

// handleUpload receives the photos of an offer (POST /p/<token>/photos?offer=<n>).
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	offerID, _ := strconv.ParseUint(r.URL.Query().Get("offer"), 10, 64) // a bad number is 0: no offer has it
	l, o, gone, stale, busy := s.claim(r.PathValue("token"), offerID)
	switch {
	case gone:
		s.writeJSON(w, http.StatusGone, s.currentLang(), reply{Error: "phone.expired.title", Expired: true})
		return
	case stale:
		s.opts.Log.Info("phone upload refused: the offer is not waiting", "offer", offerID)
		s.writeJSON(w, http.StatusConflict, s.currentLang(), reply{Error: "phone.error.stale", Stale: true})
		return
	case busy:
		s.writeJSON(w, http.StatusConflict, s.currentLang(), reply{Error: "phone.error.busy"})
		return
	}
	completed := false
	defer func() {
		if !completed {
			s.release(l, o)
		}
	}()
	if o.handlers.Started != nil {
		o.handlers.Started()
	}

	rc := http.NewResponseController(w)
	if err := rc.SetReadDeadline(time.Now().Add(uploadReadTimeout)); err != nil {
		s.opts.Log.Debug("phone upload: read deadline not set", "err", err)
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.opts.MaxTotalBytes+multipartOverhead)

	photos, err := s.receive(r)
	if err != nil {
		s.uploadFailed(w, l.lang, err)
		return
	}

	// The callback must not be cancelled because the phone hung up after
	// sending the last byte: the photos are already here.
	cbCtx := context.WithoutCancel(r.Context())
	if !s.finish(l, o) {
		s.refusedAfterReceive(w, l)
		return
	}
	completed = true // the offer is used up: there is nothing to release
	if o.handlers.Received != nil {
		if err := o.handlers.Received(cbCtx, photos); err != nil {
			s.opts.Log.Info("phone: photos refused", "err", err)
			s.writeJSON(w, http.StatusConflict, l.lang, reply{Error: "phone.error.stale", Stale: true})
			return
		}
	}
	s.writeJSON(w, http.StatusOK, l.lang, reply{OK: true})
}

// refusedAfterReceive answers an upload whose offer was withdrawn, replaced or
// whose link ended while the photos were read.
func (s *Server) refusedAfterReceive(w http.ResponseWriter, l *link) {
	l.mu.Lock()
	ended := l.ended
	l.mu.Unlock()
	if ended {
		s.writeJSON(w, http.StatusGone, l.lang, reply{Error: "phone.expired.title", Expired: true})
		return
	}
	s.writeJSON(w, http.StatusConflict, l.lang, reply{Error: "phone.error.stale", Stale: true})
}

// uploadFailed answers a refused upload.
func (s *Server) uploadFailed(w http.ResponseWriter, lang i18n.Lang, err error) {
	status, key := uploadStatus(err)
	if status == http.StatusInternalServerError {
		s.opts.Log.Error("phone upload failed", "err", err)
	} else {
		s.opts.Log.Info("phone upload refused", "err", err)
	}
	s.writeJSON(w, status, lang, reply{Error: key})
}

// uploadStatus maps the error of an upload to a status and a message key.
func uploadStatus(err error) (int, string) {
	appErr, ok := errors.FromError(err)
	if !ok {
		return http.StatusInternalServerError, "phone.error.server"
	}
	switch appErr.Tag {
	case "PhoneUploadMalformed":
		return http.StatusBadRequest, "phone.error.server"
	case "PhoneUploadEmpty":
		return http.StatusBadRequest, "phone.error.none"
	case "PhoneUploadTooMany":
		return http.StatusRequestEntityTooLarge, "phone.error.too_many"
	case "PhoneUploadPhotoTooBig", "PhotoTooLarge":
		return http.StatusRequestEntityTooLarge, "phone.error.too_big"
	case "PhoneUploadTotalTooBig":
		return http.StatusRequestEntityTooLarge, "phone.error.total_big"
	case "PhoneUploadNotPhoto", "PhotoUnsupported":
		return http.StatusUnsupportedMediaType, "phone.error.not_photo"
	case "PhotoUndecodable", "PhotoDecodeFailed":
		return http.StatusUnprocessableEntity, "phone.error.unreadable"
	}
	return http.StatusInternalServerError, "phone.error.server"
}

// receive reads the multipart body of an upload, checks the limits and the kind
// of each file, and normalizes the photos (SPEC §10.3). It reads the parts as a
// stream, so a request that is too big is refused as soon as a limit is
// crossed.
func (s *Server) receive(r *http.Request) ([][]byte, error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, errors.NewErr("PhoneUploadMalformed", err)
	}
	var raw [][]byte
	var total int64
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if isTooBig(err) {
			return nil, errors.NewErr("PhoneUploadTotalTooBig", err)
		}
		if err != nil {
			return nil, errors.NewErr("PhoneUploadMalformed", err)
		}
		if part.FormName() != "photos" || part.FileName() == "" {
			continue // not a file of the form: ignored
		}
		if len(raw) >= MaxPhotos {
			return nil, errors.New("PhoneUploadTooMany", "more photos than the limit", "max", MaxPhotos)
		}
		data, err := readPart(part, s.opts.MaxPhotoBytes)
		if err != nil {
			return nil, err
		}
		total += int64(len(data))
		if total > s.opts.MaxTotalBytes {
			return nil, errors.New("PhoneUploadTotalTooBig", "the photos are too big together", "max_bytes", s.opts.MaxTotalBytes)
		}
		raw = append(raw, data)
	}
	if len(raw) == 0 {
		return nil, errors.New("PhoneUploadEmpty", "the upload has no photo")
	}
	photos := make([][]byte, 0, len(raw))
	for _, data := range raw {
		photo, err := grading.NormalizePhoto(r.Context(), data)
		if err != nil {
			return nil, err
		}
		photos = append(photos, photo)
	}
	return photos, nil
}

// readPart reads one file of the upload, at most limit bytes, and checks by its
// first bytes that it is a JPEG or a PNG (not by its name or its declared type).
func readPart(part *multipart.Part, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(part, limit+1))
	if isTooBig(err) {
		return nil, errors.NewErr("PhoneUploadTotalTooBig", err)
	}
	if err != nil {
		return nil, errors.NewErr("PhoneUploadMalformed", err)
	}
	if int64(len(data)) > limit {
		return nil, errors.New("PhoneUploadPhotoTooBig", "a photo is too big", "max_bytes", limit)
	}
	if !isPhoto(data) {
		return nil, errors.New("PhoneUploadNotPhoto", "the file is not a JPEG or a PNG photo")
	}
	return data, nil
}

// isTooBig reports whether err says the body is longer than the limit of
// http.MaxBytesReader.
func isTooBig(err error) bool {
	var tooBig *http.MaxBytesError
	return errors.As(err, &tooBig)
}

// isPhoto reports whether data starts like a JPEG or a PNG file, using the
// content sniffing of net/http.
func isPhoto(data []byte) bool {
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	switch http.DetectContentType(head) {
	case "image/jpeg", "image/png":
		return true
	}
	return false
}

// Package phone is the phone server of Radio Tutor (SPEC §10): a small HTTP
// server on the local network that serves the page where the child takes
// photos of the solutions, receives the photos and hands them over.
//
// The server does not know where photos are stored. A caller opens a token
// with Open and gives it Handlers; when an upload completes, the photos are
// normalized (grading.NormalizePhoto) and passed to Handlers.Received, which
// stores them and may refuse them.
//
// # Threads
//
// Every Handlers callback runs on a goroutine of the HTTP server, never on the
// UI thread, and callbacks of different tokens (or of one token, for a page
// that is opened twice) may run at the same time. A GUI that wants to show an
// event Posts it to the UI thread (CLAUDE.md "Go conventions").
//
// # Status codes
//
// A token that is unknown, expired, revoked or already used answers 410 with a
// friendly page (SPEC §10.2); other paths answer 404. An upload that is
// refused answers: 400 (no photo, or not a multipart form), 409 (the token is
// receiving another upload), 413 (more than MaxPhotos files, a file or the
// total above its limit, or a photo with more than grading.MaxPhotoPixels
// pixels), 415 (not a JPEG or PNG photo), 422 (the photo cannot be decoded),
// 429 (rate limit) or 500.
package phone

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"html/template"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
)

// Limits and defaults of the server (SPEC §10.2).
const (
	// DefaultPort is the default port of the phone page (setting phone.port).
	DefaultPort = 8737
	// MaxPhotos is the most photos in one upload.
	MaxPhotos = 6
	// MaxPhotoBytes is the largest size of one uploaded file.
	MaxPhotoBytes = 20 << 20
	// MaxTotalBytes is the largest total size of the files of one upload.
	MaxTotalBytes = 80 << 20
	// TokenTTL is how long a token is valid.
	TokenTTL = 30 * time.Minute
	// RequestsPerMinute is the most requests one remote address may make in a
	// minute.
	RequestsPerMinute = 30
)

// tokenBytes is the number of random bytes of a token (SPEC §10.1).
const tokenBytes = 16

// maxAddresses bounds the memory of the rate limiter.
const maxAddresses = 4096

// Options configure a Server. The zero value is valid and gives the defaults of
// SPEC §10.2; the other fields exist for tests.
type Options struct {
	// Lang is the language of the pages that belong to no token (404, expired
	// and unknown tokens). Default English; see Server.SetLang.
	Lang i18n.Lang
	// ListenHost is the address the server binds to. Default "0.0.0.0".
	ListenHost string
	// Clock returns the current time. Default time.Now.
	Clock func() time.Time
	// TokenTTL is how long a token is valid. Default the constant TokenTTL.
	TokenTTL time.Duration
	// Random is the source of the token bytes. Default crypto/rand.
	Random io.Reader
	// Log receives the server's log lines. Default: discarded.
	Log *slog.Logger
	// MaxPhotoBytes and MaxTotalBytes override the constants of the same
	// names. Default 0 means the constant.
	MaxPhotoBytes int64
	// MaxTotalBytes overrides the constant of the same name.
	MaxTotalBytes int64
	// RequestsPerMinute overrides the constant of the same name.
	RequestsPerMinute int
}

// PageData is what the phone page shows for one token.
type PageData struct {
	// Lang is the language of the page (the app's language).
	Lang i18n.Lang
	// Title is the cassette's title.
	Title string
	// Exercises are the exercises of the submission, in the order shown.
	Exercises []PageExercise
}

// PageExercise is one exercise on the phone page. All text is escaped when the
// page is rendered.
type PageExercise struct {
	// Label is the exercise's label, shown in a badge.
	Label string
	// Title is the exercise's title.
	Title string
	// Prompt is the exercise's prompt text; a newline becomes a line break.
	Prompt string
}

// Handlers receive the events of one token. Any of them may be nil. They run on
// server goroutines (see the package documentation).
type Handlers struct {
	// Opened is called each time the page of the token is opened (a GET).
	Opened func()
	// Started is called when an upload of the token is accepted and its body
	// starts to be read.
	Started func()
	// Received is called when a whole upload has been read and normalized,
	// with the photos as JPEG data in the order they were sent. The context
	// is not cancelled when the phone disconnects. Returning nil completes the
	// submission: the token is used up and the page shows "Sent!". Returning an
	// error (the submission was cancelled, the photos could not be stored)
	// revokes the token and the page shows the expired message.
	Received func(ctx context.Context, photos [][]byte) error
}

// entry is an open token.
type entry struct {
	view     pageView
	lang     i18n.Lang
	handlers Handlers
	expires  time.Time
	busy     bool // an upload of the token is being received
}

// Server is the phone server. Use New. All methods are safe for concurrent use.
type Server struct {
	opts    Options
	clock   func() time.Time
	limiter *limiter
	tmpl    *template.Template

	mu      sync.Mutex
	lang    i18n.Lang
	tokens  map[string]*entry
	srv     *http.Server
	ln      net.Listener
	runCtx  context.Context
	serving chan struct{} // closed when the serve goroutine has ended
}

// New returns a Server that is not listening yet.
func New(opts Options) *Server {
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	if opts.ListenHost == "" {
		opts.ListenHost = "0.0.0.0"
	}
	if opts.TokenTTL <= 0 {
		opts.TokenTTL = TokenTTL
	}
	if opts.Random == nil {
		opts.Random = rand.Reader
	}
	if opts.Log == nil {
		opts.Log = slog.New(slog.DiscardHandler)
	}
	if opts.MaxPhotoBytes <= 0 {
		opts.MaxPhotoBytes = MaxPhotoBytes
	}
	if opts.MaxTotalBytes <= 0 {
		opts.MaxTotalBytes = MaxTotalBytes
	}
	if opts.RequestsPerMinute <= 0 {
		opts.RequestsPerMinute = RequestsPerMinute
	}
	lang := opts.Lang
	if !lang.Valid() {
		lang = i18n.EN
	}
	return &Server{
		opts:    opts,
		clock:   opts.Clock,
		tmpl:    pageTemplates,
		limiter: newLimiter(opts.Clock, opts.RequestsPerMinute, maxAddresses),
		lang:    lang,
		tokens:  map[string]*entry{},
	}
}

// SetLang sets the language of the pages that belong to no token.
func (s *Server) SetLang(l i18n.Lang) {
	if !l.Valid() {
		return
	}
	s.mu.Lock()
	s.lang = l
	s.mu.Unlock()
}

// Open makes a token for a submission and returns it with the URL the phone
// opens: http://<host>:<port>/s/<token>. host is the LAN address (DetectLAN).
// The token is valid for the token TTL, until its photos are received, or until
// Close. The server must be running.
func (s *Server) Open(host string, page PageData, h Handlers) (token, url string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln == nil {
		return "", "", errors.New("PhoneNotRunning", "the phone server is not running")
	}
	raw := make([]byte, tokenBytes)
	if _, err := io.ReadFull(s.opts.Random, raw); err != nil {
		return "", "", errors.NewErr("PhoneTokenFailed", err)
	}
	token = tokenEncoding.EncodeToString(raw)
	now := s.clock()
	for t, e := range s.tokens {
		if !now.Before(e.expires) && !e.busy {
			delete(s.tokens, t)
		}
	}
	if !page.Lang.Valid() {
		page.Lang = i18n.EN
	}
	s.tokens[token] = &entry{
		view:     newPageView(page),
		lang:     page.Lang,
		handlers: h,
		expires:  now.Add(s.opts.TokenTTL),
	}
	port := s.ln.Addr().(*net.TCPAddr).Port
	return token, "http://" + net.JoinHostPort(host, strconv.Itoa(port)) + "/s/" + token, nil
}

// Close revokes a token. A page that is open on a phone shows the expired
// message when it sends. Closing an unknown token does nothing.
func (s *Server) Close(token string) {
	s.mu.Lock()
	delete(s.tokens, token)
	s.mu.Unlock()
}

// tokenEncoding is how tokens are written (SPEC §10.1): base32, no padding,
// lowercase letters being easier to type.
var tokenEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

// find returns the token's entry when the token is open and not expired; an
// expired entry is dropped. The caller holds s.mu.
func (s *Server) find(token string) *entry {
	e, ok := s.tokens[token]
	if !ok {
		return nil
	}
	if !s.clock().Before(e.expires) {
		delete(s.tokens, token)
		return nil
	}
	return e
}

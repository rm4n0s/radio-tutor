// Package phone is the phone server of Radio Tutor (SPEC §10): a small HTTP
// server on the local network that serves the page where the child takes
// photos of the solutions, receives the photos and hands them over.
//
// # Links and offers
//
// A caller makes a Link for a student with OpenLink; the link is the phone's
// address (the QR code) and stays open between submissions. When a submission
// is ready for photos, the caller Offers it on the link with Handlers: the
// phone page, which is waiting, switches to the exercises by itself. When the
// photos arrive they are normalized (grading.NormalizePhoto) and passed to
// Handlers.Received, which stores them and may refuse them; the page goes back
// to waiting and the link stays. An Offer that the caller Withdraws (the
// submission was cancelled) sends the page back to waiting too.
//
// The server does not know where photos are stored, nor who the student is: a
// caller that ties a link to a student ends it (Link.End) when the student logs
// out.
//
// A link ends by Link.End, by Server.Stop, and when no offer has ended on it
// for the link idle time (LinkIdleTTL); a link with an offer waiting for photos
// does not end by idling.
//
// # Threads
//
// Every Handlers callback runs on a goroutine of the HTTP server, never on the
// UI thread, and callbacks of different offers (or of one offer, for a page
// that is opened on two phones) may run at the same time. A GUI that wants to
// show an event Posts it to the UI thread (CLAUDE.md "Go conventions").
//
// # Status codes
//
// A link that is unknown, ended or idle answers 410 with a friendly page
// (SPEC §10.2); other paths answer 404. An upload that is refused answers: 400
// (no photo, or not a multipart form), 409 (the offer is not the current one,
// or the offer is receiving another upload), 410 (the link ended), 413 (more
// than MaxPhotos files, a file or the total above its limit, or a photo with
// more than grading.MaxPhotoPixels pixels), 415 (not a JPEG or PNG photo), 422
// (the photo cannot be decoded), 429 (rate limit) or 500.
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
	"sync"
	"time"

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
	// LinkIdleTTL is how long a link lives when no offer is waiting on it: it
	// counts from the creation of the link or from the last time an offer
	// ended on it (photos sent, or withdrawn).
	LinkIdleTTL = 60 * time.Minute
	// PollTimeout is how long the server holds a state request that has
	// nothing new to answer.
	PollTimeout = 25 * time.Second
	// SeenWithin is how long after its last request a phone counts as having
	// the link open (Link.Seen).
	SeenWithin = 30 * time.Second
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
	// Lang is the language of the pages that belong to no link (404, expired
	// and unknown links). Default English; see Server.SetLang.
	Lang i18n.Lang
	// ListenHost is the address the server binds to. Default "0.0.0.0".
	ListenHost string
	// Clock returns the current time. Default time.Now. It decides when a link
	// is idle and whether a phone was seen; the hold of a state request uses
	// the real time (PollTimeout).
	Clock func() time.Time
	// LinkIdleTTL overrides the constant of the same name.
	LinkIdleTTL time.Duration
	// PollTimeout overrides the constant of the same name.
	PollTimeout time.Duration
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

// PageData is what the phone page shows for one offer.
type PageData struct {
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

// Handlers receive the events of one offer. Any of them may be nil. They run on
// server goroutines (see the package documentation).
type Handlers struct {
	// Opened is called once for the offer, the first time a phone asks for
	// the page or for news while the offer is waiting (not when a phone was
	// already there when the offer was made: Link.Offer says so).
	Opened func()
	// Started is called when an upload of the offer is accepted and its body
	// starts to be read.
	Started func()
	// Received is called when a whole upload has been read and normalized,
	// with the photos as JPEG data in the order they were sent. The context
	// is not cancelled when the phone disconnects. Returning nil completes the
	// offer: it is no longer waiting and the page shows "Sent!". Returning an
	// error (the submission was cancelled, the photos could not be stored)
	// sends the page back to waiting with a friendly message.
	Received func(ctx context.Context, photos [][]byte) error
}

// Server is the phone server. Use New. All methods are safe for concurrent use.
type Server struct {
	opts    Options
	clock   func() time.Time
	limiter *limiter
	tmpl    *template.Template

	mu      sync.Mutex
	lang    i18n.Lang
	links   map[string]*link
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
	if opts.LinkIdleTTL <= 0 {
		opts.LinkIdleTTL = LinkIdleTTL
	}
	if opts.PollTimeout <= 0 {
		opts.PollTimeout = PollTimeout
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
		links:   map[string]*link{},
	}
}

// SetLang sets the language of the pages that belong to no link.
func (s *Server) SetLang(l i18n.Lang) {
	if !l.Valid() {
		return
	}
	s.mu.Lock()
	s.lang = l
	s.mu.Unlock()
}

// tokenEncoding is how tokens are written (SPEC §10.1): base32, no padding,
// lowercase letters being easier to type.
var tokenEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

package phone

import (
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
)

// State names of the phone page (SPEC §10.1). The page has more states of its
// own (sent, expired); these are the two the server knows.
const (
	stateWaiting = "waiting"
	stateReady   = "ready"
)

// stateView is the state of a link as the page asks for it
// (GET /p/<token>/state) and as the first page carries it. V grows with every
// change, so that a page can tell a stale answer from a new one.
type stateView struct {
	V         uint64         `json:"v"`
	State     string         `json:"state"`
	Title     string         `json:"title,omitempty"`
	Offer     uint64         `json:"offer,omitempty"`
	Exercises []exerciseView `json:"exercises,omitempty"`
	Expired   bool           `json:"expired,omitempty"`
}

// exerciseView is one exercise of the state.
type exerciseView struct {
	Label string `json:"label"`
	Title string `json:"title"`
	// Lines are the lines of the prompt.
	Lines []string `json:"lines,omitempty"`
}

// link is the server's side of a Link. The lock order is Server.mu, then
// link.mu.
type link struct {
	token string
	lang  i18n.Lang
	text  map[string]string
	clock func() time.Time

	mu        sync.Mutex
	ended     bool
	lastUse   time.Time // creation, or when an offer last ended
	seenAt    time.Time // the last request of a phone; zero when none came
	polling   int       // state requests being held
	version   uint64    // grows on every change; starts at 1
	changed   chan struct{}
	title     string // the title of the last offer
	cur       *offer // the submission waiting for photos
	lastOffer uint64
}

// offer is a submission waiting for photos on a link.
type offer struct {
	id        uint64
	title     string
	exercises []exerciseView
	handlers  Handlers
	opened    bool // Opened was called, or a phone was there already
	busy      bool // an upload of the offer is being received
}

// bump records a change and wakes the held state requests. The caller holds
// l.mu.
func (l *link) bump() {
	l.version++
	close(l.changed)
	l.changed = make(chan struct{})
}

// state is the current state. The caller holds l.mu.
func (l *link) state() stateView {
	v := stateView{V: l.version, State: stateWaiting, Title: l.title, Expired: l.ended}
	if o := l.cur; o != nil && !l.ended {
		v.State, v.Title, v.Offer, v.Exercises = stateReady, o.title, o.id, o.exercises
	}
	return v
}

// endLocked ends the link: no offer is waiting any more and the held state
// requests wake up and answer "expired". The caller holds l.mu.
func (l *link) endLocked() {
	if l.ended {
		return
	}
	l.ended = true
	l.cur = nil
	l.bump()
}

// idleLocked reports whether the link has been idle for ttl: no offer is
// waiting on it and the last one ended (or the link was made) ttl ago. The
// caller holds l.mu.
func (l *link) idleLocked(now time.Time, ttl time.Duration) bool {
	return l.cur == nil && !now.Before(l.lastUse.Add(ttl))
}

// seenLocked reports whether a phone has the link open: it is waiting for news
// or asked within SeenWithin. The caller holds l.mu.
func (l *link) seenLocked(now time.Time) bool {
	return l.polling > 0 || (!l.seenAt.IsZero() && now.Sub(l.seenAt) <= SeenWithin)
}

// touchLocked records a request of a phone. It returns the Opened handler of
// the waiting offer when this is the first request since the offer was made,
// for the caller to call without holding l.mu. The caller holds l.mu.
func (l *link) touchLocked(now time.Time) func() {
	l.seenAt = now
	if o := l.cur; o != nil && !o.opened {
		o.opened = true
		return o.handlers.Opened
	}
	return nil
}

// Link is the address of a phone: a secret URL on which the page shows the
// submissions that are offered one after the other. See the package
// documentation.
type Link struct {
	// Token is the secret part of the URL (16 random bytes, base32).
	Token string
	// URL is the address the phone opens: http://<host>:<port>/p/<token>.
	URL string

	s *Server
	l *link
}

// Offer is a submission waiting for photos on a link.
type Offer struct {
	l *link
	o *offer
}

// OpenLink makes a link. host is the LAN address (DetectLAN) the URL names.
// lang is the language of the page. The link lives until End, until the server
// stops, or until it has been idle for the link idle time. The server must be
// running; otherwise the tag is PhoneNotRunning.
func (s *Server) OpenLink(host string, lang i18n.Lang) (*Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln == nil {
		return nil, errors.New("PhoneNotRunning", "the phone server is not running")
	}
	raw := make([]byte, tokenBytes)
	if _, err := io.ReadFull(s.opts.Random, raw); err != nil {
		return nil, errors.NewErr("PhoneTokenFailed", err)
	}
	token := tokenEncoding.EncodeToString(raw)
	now := s.clock()
	for t, l := range s.links {
		l.mu.Lock()
		gone := l.ended || l.idleLocked(now, s.opts.LinkIdleTTL)
		if gone {
			l.endLocked()
			delete(s.links, t)
		}
		l.mu.Unlock()
	}
	if !lang.Valid() {
		lang = i18n.EN
	}
	l := &link{
		token:   token,
		lang:    lang,
		text:    phoneText(lang),
		clock:   s.clock,
		lastUse: now,
		version: 1,
		changed: make(chan struct{}),
	}
	s.links[token] = l
	port := s.ln.Addr().(*net.TCPAddr).Port
	url := "http://" + net.JoinHostPort(host, strconv.Itoa(port)) + "/p/" + token
	return &Link{Token: token, URL: url, s: s, l: l}, nil
}

// findLink returns the link of a token when it is open: a link that is idle is
// ended and dropped here, so that it answers 410 from then on.
func (s *Server) findLink(token string) *link {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.links[token]
	if !ok {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.ended || l.idleLocked(s.clock(), s.opts.LinkIdleTTL) {
		l.endLocked()
		delete(s.links, token)
		return nil
	}
	return l
}

// endLinks ends every link. The caller holds s.mu.
func (s *Server) endLinks() {
	for t, l := range s.links {
		l.mu.Lock()
		l.endLocked()
		l.mu.Unlock()
		delete(s.links, t)
	}
}

// Alive reports whether the link is still open (it was not ended and is not
// idle).
func (k *Link) Alive() bool { return k.s.findLink(k.Token) == k.l }

// Seen reports whether a phone has the link open: it asked for the page or
// for news within the last SeenWithin, or is waiting for news now.
func (k *Link) Seen() bool {
	k.l.mu.Lock()
	defer k.l.mu.Unlock()
	return !k.l.ended && k.l.seenLocked(k.s.clock())
}

// End ends the link: its page shows the expired message at its next request
// (a page that waits for news is woken at once), and a waiting offer is
// dropped. Ending a link that ended does nothing.
func (k *Link) End() {
	k.s.mu.Lock()
	defer k.s.mu.Unlock()
	if k.s.links[k.Token] == k.l {
		delete(k.s.links, k.Token)
	}
	k.l.mu.Lock()
	k.l.endLocked()
	k.l.mu.Unlock()
}

// Offer makes a submission waiting for photos on the link: the page switches to
// the exercises of page. An offer that is waiting is replaced (its upload, if
// one is being received, is refused as stale). seen tells that a phone had the
// link open when the offer was made (Link.Seen); Handlers.Opened is then not
// called for the offer. It fails with PhoneLinkEnded when the link ended.
func (k *Link) Offer(page PageData, h Handlers) (o *Offer, seen bool, err error) {
	now := k.s.clock()
	k.s.mu.Lock()
	defer k.s.mu.Unlock()
	k.l.mu.Lock()
	defer k.l.mu.Unlock()
	if k.s.links[k.Token] != k.l || k.l.ended || k.l.idleLocked(now, k.s.opts.LinkIdleTTL) {
		return nil, false, errors.New("PhoneLinkEnded", "the phone link has ended")
	}
	k.l.lastOffer++
	seen = k.l.seenLocked(now)
	in := &offer{
		id:        k.l.lastOffer,
		title:     page.Title,
		exercises: exerciseViews(page.Exercises),
		handlers:  h,
		opened:    seen,
	}
	k.l.cur = in
	k.l.title = page.Title
	k.l.bump()
	return &Offer{l: k.l, o: in}, seen, nil
}

// ID is the number the page names when it uploads photos for the offer.
func (o *Offer) ID() uint64 { return o.o.id }

// Withdraw takes the offer back: the page goes back to waiting and an upload
// for it is refused. Withdrawing an offer that is not the waiting one any more
// (it was sent, replaced or withdrawn) does nothing.
func (o *Offer) Withdraw() {
	now := o.l.clock()
	o.l.mu.Lock()
	defer o.l.mu.Unlock()
	if o.l.cur != o.o {
		return
	}
	o.l.cur = nil
	o.l.lastUse = now
	o.l.bump()
}

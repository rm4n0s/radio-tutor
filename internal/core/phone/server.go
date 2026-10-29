package phone

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
)

// Timeouts of the HTTP server. The body of an upload gets its own read
// deadline (uploadReadTimeout), because photos come slowly over Wi-Fi.
const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 60 * time.Second
	uploadReadTimeout = 5 * time.Minute
	stopGrace         = 5 * time.Second
	// multipartOverhead is what the body may hold above the files: the
	// boundaries and the headers of the parts.
	multipartOverhead = 1 << 20
)

// Start binds the server to ListenHost:port and serves in the background. A
// port of 0 lets the system pick one (Port gives it). The server stops when
// ctx is cancelled or when Stop is called. A port that is taken gives the tag
// PhonePortInUse with the port in the metadata.
func (s *Server) Start(ctx context.Context, port int) error {
	static, err := readStatic(assets)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv != nil {
		return errors.New("PhoneAlreadyStarted", "the phone server is already running", "port", s.ln.Addr().(*net.TCPAddr).Port)
	}
	addr := net.JoinHostPort(s.opts.ListenHost, strconv.Itoa(port))
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			return errors.NewErr("PhonePortInUse", err).SetMetadata("port", port)
		}
		return errors.NewErr("PhoneListenFailed", err).SetMetadata("port", port)
	}
	srv := &http.Server{
		Handler:           s.routes(static),
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		BaseContext:       func(net.Listener) context.Context { return ctx },
		ErrorLog:          slog.NewLogLogger(s.opts.Log.Handler(), slog.LevelDebug),
	}
	serving := make(chan struct{})
	s.srv, s.ln, s.runCtx, s.serving = srv, ln, ctx, serving
	go func() {
		defer close(serving)
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.opts.Log.Error("phone server stopped", "err", err)
		}
	}()
	go func() {
		select {
		case <-ctx.Done():
			stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stopGrace)
			defer cancel()
			if err := s.stop(stopCtx, srv); err != nil {
				s.opts.Log.Error("phone server stop failed", "err", err)
			}
		case <-serving:
		}
	}()
	return nil
}

// Port returns the port the server listens on, or 0 when it is not running.
func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln == nil {
		return 0
	}
	return s.ln.Addr().(*net.TCPAddr).Port
}

// Running reports whether the server is listening.
func (s *Server) Running() bool { return s.Port() != 0 }

// Stop shuts the server down gracefully: it stops accepting, lets the requests
// in flight finish until ctx ends (then closes their connections), and revokes
// every token. Stopping a server that is not running does nothing.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	srv := s.srv
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	return s.stop(ctx, srv)
}

// stop shuts down srv if it is still the running server.
func (s *Server) stop(ctx context.Context, srv *http.Server) error {
	s.mu.Lock()
	if s.srv != srv {
		s.mu.Unlock()
		return nil
	}
	serving := s.serving
	s.srv, s.ln, s.runCtx, s.serving = nil, nil, nil, nil
	clear(s.tokens)
	s.mu.Unlock()
	err := srv.Shutdown(ctx)
	if err != nil {
		_ = srv.Close() // the grace period is over: drop the connections
		<-serving
		return errors.NewErr("PhoneStopFailed", err)
	}
	<-serving
	return nil
}

// routes builds the handler (SPEC §10.2): the page, the upload and the static
// files; anything else is 404.
func (s *Server) routes(static map[string]staticFile) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /s/{token}", s.handlePage)
	mux.HandleFunc("POST /s/{token}/photos", s.handleUpload)
	mux.HandleFunc("GET /static/{name}", func(w http.ResponseWriter, r *http.Request) {
		f, ok := static[r.PathValue("name")]
		if !ok {
			s.notFound(w)
			return
		}
		w.Header().Set("Content-Type", f.contentType)
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(f.data)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { s.notFound(w) })
	return s.guard(mux)
}

// guard applies the rate limit and the security headers to every request.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		h.Set("Content-Security-Policy", "default-src 'none'; img-src 'self' blob: data:; style-src 'self'; script-src 'self'; connect-src 'self'; form-action 'none'; base-uri 'none'; frame-ancestors 'none'")
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if ok, wait := s.limiter.allow(host); !ok {
			secs := int((wait + time.Second - 1) / time.Second)
			h.Set("Retry-After", strconv.Itoa(secs))
			if strings.HasSuffix(r.URL.Path, "/photos") {
				s.writeJSON(w, http.StatusTooManyRequests, s.currentLang(), reply{Error: "phone.error.rate"})
			} else {
				http.Error(w, i18n.For(s.currentLang()).T("phone.error.rate"), http.StatusTooManyRequests)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) currentLang() i18n.Lang {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lang
}

// notFound writes the friendly 404 page.
func (s *Server) notFound(w http.ResponseWriter) {
	if err := renderMessage(s.tmpl, w, http.StatusNotFound, s.currentLang(), "phone.not_found.title", "phone.not_found.body"); err != nil {
		s.opts.Log.Error("phone message page failed", "err", err)
	}
}

// gone writes the friendly "this link has expired" page (410).
func (s *Server) gone(w http.ResponseWriter) {
	if err := renderMessage(s.tmpl, w, http.StatusGone, s.currentLang(), "phone.expired.title", "phone.expired.body"); err != nil {
		s.opts.Log.Error("phone message page failed", "err", err)
	}
}

// handlePage serves the page of a token.
func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	e := s.find(r.PathValue("token"))
	s.mu.Unlock()
	if e == nil {
		s.gone(w)
		return
	}
	if err := renderPage(s.tmpl, w, e.view); err != nil {
		s.opts.Log.Error("phone page failed", "err", err)
		http.Error(w, i18n.For(e.lang).T("phone.error.server"), http.StatusInternalServerError)
		return
	}
	if e.handlers.Opened != nil && r.Method == http.MethodGet {
		e.handlers.Opened()
	}
}

// reply is the JSON the upload answers with. Error is a message key until
// writeJSON translates it.
type reply struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Expired bool   `json:"expired,omitempty"`
}

// writeJSON writes a reply, translating its error key into lang.
func (s *Server) writeJSON(w http.ResponseWriter, status int, lang i18n.Lang, rep reply) {
	if rep.Error != "" {
		rep.Error = i18n.For(lang).T(rep.Error, "max", strconv.Itoa(MaxPhotos))
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(rep); err != nil {
		s.opts.Log.Debug("phone reply failed", "err", err)
	}
}

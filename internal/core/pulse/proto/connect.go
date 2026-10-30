// Copied from github.com/jfreymuth/pulse v0.1.3 and changed for Radio Tutor. MIT License, Copyright (c) 2019 Johann Freymuth: see internal/core/pulse/LICENSE.

package proto

import (
	"fmt"
	"net"
	"os"
	"os/user"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/rm4n0s/errors"
)

// defaultPulseAudioTCPPortString is the TCP port of a server string that names
// only a host.
const defaultPulseAudioTCPPortString = "4713"

// Connect connects to the pulse server.
//
// For the server string format see
// https://www.freedesktop.org/wiki/Software/PulseAudio/Documentation/User/ServerStrings/
// If the server string is empty, the environment variable PULSE_SERVER will be used.
//
// It fails, tagged PulseNoValidServer, PulseHostnameFailed, PulseNoReachableServer
// or, for the last server it tried, PulseDialFailed, PulseCookieReadFailed or
// the tag of the failed authentication request (see Client.Request).
func Connect(server string) (*Client, net.Conn, error) {
	return connect(server, os.Hostname)
}

// connect is Connect with the lookup of the host name as a parameter.
func connect(server string, hostname func() (string, error)) (*Client, net.Conn, error) {
	var sstr []serverString
	if server != "" {
		sstr = parseServerString(server)
	} else if serverRaw, ok := os.LookupEnv("PULSE_SERVER"); ok {
		sstr = parseServerString(serverRaw)
	} else {
		sstr = defaultServerStrings()
	}
	if len(sstr) == 0 {
		return nil, nil, errors.New("PulseNoValidServer", "pulseaudio: no valid server", "server", server)
	}

	localname, err := hostname()
	if err != nil {
		return nil, nil, errors.NewErr("PulseHostnameFailed", err)
	}

	var lastErr error
	for _, s := range sstr {
		if s.localname != "" && localname != s.localname {
			continue
		}
		conn, err := net.Dial(s.protocol, s.addr)
		if err != nil {
			lastErr = errors.NewErr("PulseDialFailed", err).SetMetadata("protocol", s.protocol, "addr", s.addr)
			continue
		}
		// Every attempt gets its own client: a failed one keeps its error.
		c := &Client{
			timeout: 1 * time.Second,
		}
		c.Open(conn)

		cookiePath := os.Getenv("HOME") + "/.config/pulse/cookie"
		if path, ok := os.LookupEnv("PULSE_COOKIE"); ok {
			cookiePath = path
		}

		cookie, err := os.ReadFile(cookiePath)
		if err != nil {
			if !os.IsNotExist(err) {
				_ = conn.Close() // best effort: the error that matters is kept
				lastErr = errors.NewErr("PulseCookieReadFailed", err).SetMetadata("path", cookiePath)
				continue
			}
			// If the server is launched with auth-anonymous=1,
			// any 256 bytes cookie will be accepted.
			cookie = make([]byte, 256)
		}
		var authReply AuthReply
		err = c.Request(
			&Auth{
				Version: c.Version(),
				Cookie:  cookie,
			}, &authReply)
		if err != nil {
			_ = conn.Close() // best effort: the error that matters is kept
			lastErr = err
			continue
		}
		c.SetVersion(authReply.Version)

		return c, conn, nil
	}

	if lastErr == nil {
		// Every server string was for another host.
		return nil, nil, errors.New("PulseNoReachableServer", "pulseaudio: no server string is for this host", "server", server, "host", localname)
	}
	return nil, nil, lastErr
}

type serverString struct {
	localname string
	protocol  string
	addr      string
}

// See: https://www.freedesktop.org/wiki/Software/PulseAudio/Documentation/User/ServerStrings/
func parseServerString(str string) []serverString {
	s := strings.Fields(str)
	var result []serverString
	for _, s := range s {
		server, ok := parseOneServerString(s)
		if !ok {
			continue
		}
		result = append(result, server)
	}
	return result
}

func parseOneServerString(s string) (serverString, bool) {
	var server serverString
	if s[0] == '{' {
		end := strings.IndexByte(s, '}')
		if end < 0 {
			// no closing brace
			return serverString{}, false
		}
		server.localname = s[1:end]
		s = s[end+1:]
	}
	switch {
	case len(s) == 0:
		// no server string
		return serverString{}, false
	case s[0] == '/':
		// rule #2
		server.protocol = "unix"
		server.addr = s
	case strings.HasPrefix(s, "unix:"):
		// rule #2
		server.protocol = "unix"
		server.addr = s[5:]
	case strings.HasPrefix(s, "tcp6:"):
		// rule #4
		server.protocol = "tcp6"
		server.addr = s[5:]
	case strings.HasPrefix(s, "tcp4:"):
		// rule #3
		server.protocol = "tcp4"
		server.addr = s[5:]
	case strings.HasPrefix(s, "tcp:"):
		// rule #3
		server.protocol = "tcp"
		server.addr = s[4:]
	default:
		// rule #5
		if _, _, err := net.SplitHostPort(s); err == nil {
			server.protocol = "tcp"
			server.addr = s
		} else {
			// Adding a default port, because in the doc
			// it is stated that `gurki` is a valid example...
			server.protocol = "tcp"
			server.addr = net.JoinHostPort(s, defaultPulseAudioTCPPortString)
		}
	}
	return server, true
}

func defaultServerStrings() []serverString {
	switch runtime.GOOS {
	case "dragonfly", "freebsd", "linux", "netbsd", "openbsd":
		return []serverString{{protocol: "unix",
			addr: path.Join(os.Getenv("XDG_RUNTIME_DIR"), "pulse/native"),
		}}
	case "darwin":
		u, err := user.Current()
		if err != nil {
			return nil
		}

		h, err := os.Hostname()
		if err != nil {
			return nil
		}

		return []serverString{{protocol: "unix",
			addr: fmt.Sprintf("%s/.config/pulse/%s-runtime/native", u.HomeDir, h),
		}}
	}
	return nil
}

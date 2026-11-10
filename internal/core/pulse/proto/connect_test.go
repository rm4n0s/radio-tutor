package proto

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/rm4n0s/errors"
)

func TestConnectErrors(t *testing.T) {
	missing := "unix:" + filepath.Join(t.TempDir(), "missing")
	cookieDir := t.TempDir() // reading a folder as the cookie fails, and not with "not exist"
	hostname := func() (string, error) { return "testhost", nil }
	failingHostname := func() (string, error) { return "", io.ErrNoProgress }

	cases := []struct {
		name      string
		server    string
		cookie    string
		hostname  func() (string, error)
		wantRoute string
	}{
		{"no valid server string", "{unclosed", "", hostname, "connect.PulseNoValidServer"},
		{"no host name", missing, "", failingHostname, "connect.PulseHostnameFailed"},
		{"nobody listens", missing, filepath.Join(t.TempDir(), "no-cookie"), hostname, "connect.PulseDialFailed"},
		{"every server is for another host", "{another-host}" + missing, "", hostname, "connect.PulseNoReachableServer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cookie != "" {
				t.Setenv("PULSE_COOKIE", tc.cookie)
			}
			_, _, err := connect(tc.server, tc.hostname)
			appErr, ok := errors.FromError(err)
			if !ok {
				t.Fatalf("expected *errors.Error, got %T (%v)", err, err)
			}
			if !appErr.HasRoute(tc.wantRoute) {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
		})
	}

	t.Run("the cookie cannot be read", func(t *testing.T) {
		// a short path: a Unix socket's address holds 104 bytes on macOS, which t.TempDir's folder is longer than
		short, err := os.MkdirTemp("/tmp", "pulse")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(short) })
		sock := filepath.Join(short, "native")
		ln, err := net.Listen("unix", sock)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ln.Close() }()
		go func() {
			c, err := ln.Accept()
			if err == nil {
				_ = c.Close() // the client gives up before it sends anything
			}
		}()
		t.Setenv("PULSE_COOKIE", cookieDir)
		_, _, err = connect("unix:"+sock, hostname)
		appErr, ok := errors.FromError(err)
		if !ok {
			t.Fatalf("expected *errors.Error, got %T (%v)", err, err)
		}
		if !appErr.HasRoute("connect.PulseCookieReadFailed") {
			t.Errorf("unexpected failure path: %s", appErr.Route())
		}
	})
}

func TestParseServerStringMalformed(t *testing.T) {
	for _, in := range []string{"{", "{abc", "{abc unix:/x", ""} {
		t.Run(in, func(t *testing.T) {
			for _, s := range parseServerString(in) {
				if s.localname == "abc" {
					t.Errorf("parsed %+v from a string without closing brace", s)
				}
			}
		})
	}
}

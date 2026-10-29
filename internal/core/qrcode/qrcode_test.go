package qrcode

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rm4n0s/errors"
)

var updateGoldens = flag.Bool("update-goldens", false, "rewrite the golden files")

// phoneURL has the shape of SPEC §10.1: http://<lan-address>:<port>/s/<token>
// with a 16-byte token in base32 without padding (26 characters).
const phoneURL = "http://192.168.100.200:8737/s/MFRGGZDFMZTWQ2LKNNWG23TPOM"

func mustEncode(t *testing.T, text string, level Level) Matrix {
	t.Helper()
	m, err := Encode(text, level)
	if err != nil {
		t.Fatalf("Encode(%q, %v): %v", text, level, err)
	}
	return m
}

func TestEncodeSize(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		level Level
		size  int
	}{
		{"phone URL M", phoneURL, LevelM, 33},
		{"phone URL L", phoneURL, LevelL, 33},
		{"phone URL H", phoneURL, LevelH, 41},
		{"digits are denser", "12345", LevelM, 21},
		{"short text", "a", LevelL, 21},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := mustEncode(t, tt.text, tt.level)
			if m.Size != tt.size {
				t.Errorf("Size = %d, want %d", m.Size, tt.size)
			}
			if (m.Size-17)%4 != 0 {
				t.Errorf("Size %d is not 17+4v", m.Size)
			}
			if m.Total() != m.Size+2*QuietZone {
				t.Errorf("Total = %d", m.Total())
			}
		})
	}
}

func TestEncodeFinderPatterns(t *testing.T) {
	for _, level := range []Level{LevelL, LevelM, LevelQ, LevelH} {
		t.Run(level.String(), func(t *testing.T) {
			m := mustEncode(t, phoneURL, level)
			// The 7x7 finder pattern: dark ring, light ring, 3x3 dark center.
			finder := func(ox, oy int) {
				for dy := 0; dy < 7; dy++ {
					for dx := 0; dx < 7; dx++ {
						edge := dx == 0 || dx == 6 || dy == 0 || dy == 6
						center := dx >= 2 && dx <= 4 && dy >= 2 && dy <= 4
						want := edge || center
						if got := m.Dark(ox+dx, oy+dy); got != want {
							t.Errorf("finder at (%d,%d): module (%d,%d) = %v, want %v", ox, oy, dx, dy, got, want)
						}
					}
				}
			}
			finder(0, 0)
			finder(m.Size-7, 0)
			finder(0, m.Size-7)
			// The separators next to the finders are light.
			for i := 0; i < 8; i++ {
				if m.Dark(7, i) || m.Dark(i, 7) {
					t.Errorf("top-left separator module %d is dark", i)
				}
			}
			// The timing pattern alternates along row 6 between the finders.
			for x := 8; x < m.Size-8; x++ {
				if got, want := m.Dark(x, 6), x%2 == 0; got != want {
					t.Errorf("timing module (%d,6) = %v, want %v", x, got, want)
				}
			}
		})
	}
}

func TestEncodeDeterministic(t *testing.T) {
	a := mustEncode(t, phoneURL, DefaultLevel)
	b := mustEncode(t, phoneURL, DefaultLevel)
	if a.String() != b.String() {
		t.Error("two encodings of the same text differ")
	}
	if c := mustEncode(t, phoneURL+"X", DefaultLevel); c.String() == a.String() {
		t.Error("different texts gave the same matrix")
	}
}

func TestDarkAndQuietZone(t *testing.T) {
	m := mustEncode(t, phoneURL, LevelM)
	tests := []struct {
		name  string
		x, y  int
		dark  bool
		quiet bool // expected DarkQuiet at the same coordinates shifted by QuietZone
	}{
		{"top-left module", 0, 0, true, true},
		{"negative x", -1, 0, false, false},
		{"negative y", 0, -1, false, false},
		{"x past the end", m.Size, 0, false, false},
		{"y past the end", 0, m.Size, false, false},
		{"far outside", 1000, -1000, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.Dark(tt.x, tt.y); got != tt.dark {
				t.Errorf("Dark(%d,%d) = %v, want %v", tt.x, tt.y, got, tt.dark)
			}
			if got := m.DarkQuiet(tt.x+QuietZone, tt.y+QuietZone); got != tt.quiet {
				t.Errorf("DarkQuiet(%d,%d) = %v, want %v", tt.x+QuietZone, tt.y+QuietZone, got, tt.quiet)
			}
		})
	}
	// The whole quiet zone is light.
	n := m.Total()
	for i := 0; i < n; i++ {
		for q := 0; q < QuietZone; q++ {
			if m.DarkQuiet(i, q) || m.DarkQuiet(i, n-1-q) || m.DarkQuiet(q, i) || m.DarkQuiet(n-1-q, i) {
				t.Fatalf("quiet zone module near (%d,%d) is dark", i, q)
			}
		}
	}
	if m.Total() != m.Size+8 {
		t.Errorf("Total = %d, want %d", m.Total(), m.Size+8)
	}
	var zero Matrix
	if zero.Dark(0, 0) || zero.Size != 0 || zero.String() != "" {
		t.Error("zero Matrix is not empty")
	}
}

func TestMatrixString(t *testing.T) {
	m := mustEncode(t, "a", LevelL)
	lines := strings.Split(strings.TrimSuffix(m.String(), "\n"), "\n")
	if len(lines) != m.Size {
		t.Fatalf("%d lines, want %d", len(lines), m.Size)
	}
	for y, line := range lines {
		if len(line) != m.Size {
			t.Fatalf("line %d has %d columns, want %d", y, len(line), m.Size)
		}
		for x, c := range line {
			if want := m.Dark(x, y); (c == '#') != want {
				t.Fatalf("String module (%d,%d) = %q, Dark = %v", x, y, c, want)
			}
		}
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelL, "L"}, {LevelM, "M"}, {LevelQ, "Q"}, {LevelH, "H"}, {Level(7), "Level(7)"}, {Level(-1), "Level(-1)"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", int(tt.level), got, tt.want)
		}
	}
	if DefaultLevel != LevelM {
		t.Errorf("DefaultLevel = %v, want M", DefaultLevel)
	}
}

func TestEncodeErrors(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		level Level
		route string
	}{
		{"empty text", "", LevelM, "Encode.QRTextEmpty"},
		{"level too high", "x", Level(4), "Encode.QRLevelInvalid"},
		{"negative level", "x", Level(-1), "Encode.QRLevelInvalid"},
		{"text too long", strings.Repeat("x", 4000), LevelM, "Encode.QRTextTooLong"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := Encode(tt.text, tt.level)
			if err == nil {
				t.Fatal("expected an error")
			}
			if m.Size != 0 {
				t.Errorf("Size = %d on error", m.Size)
			}
			appErr, ok := errors.FromError(err)
			if !ok {
				t.Fatalf("expected *errors.Error, got %T", err)
			}
			if !appErr.HasRoute(tt.route) {
				t.Errorf("unexpected failure path: %s", appErr.Route())
			}
		})
	}
}

func TestGoldenMatrix(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		level Level
		file  string
	}{
		{"phone URL at M", phoneURL, LevelM, "phone_url_m.txt"},
		{"phone URL at H", phoneURL, LevelH, "phone_url_h.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustEncode(t, tt.text, tt.level).String()
			path := filepath.Join("testdata", "goldens", tt.file)
			if *updateGoldens {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading golden (run with -update-goldens to create it): %v", err)
			}
			if got != string(want) {
				t.Errorf("matrix differs from %s:\n got:\n%s\nwant:\n%s", path, got, want)
			}
		})
	}
}

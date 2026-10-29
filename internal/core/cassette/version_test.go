package cassette

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/rm4n0s/errors"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		in    string
		want  Version
		route string // empty when valid
	}{
		{"0.0.0", Version{0, 0, 0}, ""},
		{"1.0.0", Version{1, 0, 0}, ""},
		{"1.20.300", Version{1, 20, 300}, ""},
		{"123456789.0.0", Version{123456789, 0, 0}, ""},
		{"", Version{}, "ParseVersion.VersionNotThreeNumbers"},
		{"1", Version{}, "ParseVersion.VersionNotThreeNumbers"},
		{"1.0", Version{}, "ParseVersion.VersionNotThreeNumbers"},
		{"1.0.0.0", Version{}, "ParseVersion.VersionNotThreeNumbers"},
		{"1..0", Version{}, "ParseVersion.VersionNumberInvalid"},
		{".1.0", Version{}, "ParseVersion.VersionNumberInvalid"},
		{"1.0.", Version{}, "ParseVersion.VersionNumberInvalid"},
		{"v1.0.0", Version{}, "ParseVersion.VersionNumberInvalid"},
		{"1.0.0-beta", Version{}, "ParseVersion.VersionNumberInvalid"},
		{"1.0.0+build", Version{}, "ParseVersion.VersionNumberInvalid"},
		{" 1.0.0", Version{}, "ParseVersion.VersionNumberInvalid"},
		{"-1.0.0", Version{}, "ParseVersion.VersionNumberInvalid"},
		{"+1.0.0", Version{}, "ParseVersion.VersionNumberInvalid"},
		{"1.0.0\n", Version{}, "ParseVersion.VersionNumberInvalid"},
		{"1.0.٣", Version{}, "ParseVersion.VersionNumberInvalid"},
		{"1234567890.0.0", Version{}, "ParseVersion.VersionNumberInvalid"},
		{"01.0.0", Version{}, "ParseVersion.VersionLeadingZero"},
		{"1.00.0", Version{}, "ParseVersion.VersionLeadingZero"},
		{"1.0.007", Version{}, "ParseVersion.VersionLeadingZero"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseVersion(tt.in)
			if tt.route == "" {
				if err != nil || got != tt.want {
					t.Fatalf("ParseVersion = %v, %v; want %v", got, err, tt.want)
				}
				if got.String() != tt.in {
					t.Errorf("String() = %q, want %q", got.String(), tt.in)
				}
				return
			}
			appErr, ok := errors.FromError(err)
			if !ok {
				t.Fatalf("err = %v", err)
			}
			if !appErr.HasRoute(tt.route) {
				t.Errorf("route = %s, want %s", appErr.Route(), tt.route)
			}
			if appErr.Metadata["version"] != tt.in {
				t.Errorf("metadata = %v", appErr.Metadata)
			}
		})
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.9.9", "2.0.0", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.2.0", "1.10.0", -1}, // numbers, not text
		{"1.10.0", "1.2.0", 1},
		{"0.0.1", "0.0.0", 1},
		{"10.0.0", "9.99.99", 1},
	}
	for _, tt := range tests {
		a, err := ParseVersion(tt.a)
		if err != nil {
			t.Fatal(err)
		}
		b, err := ParseVersion(tt.b)
		if err != nil {
			t.Fatal(err)
		}
		if got := a.Compare(b); got != tt.want {
			t.Errorf("%s.Compare(%s) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
		if got := a.Less(b); got != (tt.want < 0) {
			t.Errorf("%s.Less(%s) = %v", tt.a, tt.b, got)
		}
	}
	var vs []Version
	for _, s := range []string{"1.10.0", "1.2.0", "0.9.9", "1.2.10", "1.2.9"} {
		v, err := ParseVersion(s)
		if err != nil {
			t.Fatal(err)
		}
		vs = append(vs, v)
	}
	sort.Slice(vs, func(i, j int) bool { return vs[i].Less(vs[j]) })
	var got []string
	for _, v := range vs {
		got = append(got, v.String())
	}
	if want := "0.9.9 1.2.0 1.2.9 1.2.10 1.10.0"; len(got) != 5 || got[0]+" "+got[1]+" "+got[2]+" "+got[3]+" "+got[4] != want {
		t.Errorf("sorted = %v, want %s", got, want)
	}
}

func TestVersionJSON(t *testing.T) {
	var v struct{ V Version }
	if err := json.Unmarshal([]byte(`{"V": "2.3.4"}`), &v); err != nil || v.V != (Version{2, 3, 4}) {
		t.Fatalf("v = %v, err = %v", v, err)
	}
	out, err := json.Marshal(v)
	if err != nil || string(out) != `{"V":"2.3.4"}` {
		t.Errorf("Marshal = %s, %v", out, err)
	}
	if err := json.Unmarshal([]byte(`{"V": "2.3"}`), &v); err == nil {
		t.Error("an invalid version was accepted")
	}
	if (Version{}).String() != "0.0.0" {
		t.Error("zero Version is not 0.0.0")
	}
}

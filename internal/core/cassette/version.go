package cassette

import (
	"strconv"
	"strings"

	"github.com/rm4n0s/errors"
)

// maxVersionDigits is the most digits a version number may have, so that every
// number fits an int on every platform.
const maxVersionDigits = 9

// Version is a cassette version, MAJOR.MINOR.PATCH (CASSETTE-FORMAT §2). The
// zero value is 0.0.0.
type Version struct {
	// Major is the first number.
	Major int
	// Minor is the second number.
	Minor int
	// Patch is the third number.
	Patch int
}

// ParseVersion parses "MAJOR.MINOR.PATCH": three numbers of digits only (at
// most nine each), without a sign, and without leading zeros except for "0"
// itself. "1.0", "1.0.0.0", "01.0.0", "1.0.0-beta" and " 1.0.0" are refused.
func ParseVersion(s string) (Version, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, errors.New("VersionNotThreeNumbers", "a version has three numbers, MAJOR.MINOR.PATCH, such as 1.0.0", "version", s)
	}
	var nums [3]int
	for i, p := range parts {
		if p == "" || len(p) > maxVersionDigits || strings.Trim(p, "0123456789") != "" {
			return Version{}, errors.New("VersionNumberInvalid", "each number of a version is 1 to 9 digits, such as 1.0.0", "version", s)
		}
		if len(p) > 1 && p[0] == '0' {
			return Version{}, errors.New("VersionLeadingZero", "a number of a version has no leading zeros", "version", s)
		}
		for _, c := range p {
			nums[i] = nums[i]*10 + int(c-'0')
		}
	}
	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2]}, nil
}

// Compare returns -1 if v is older than w, 0 if they are equal and 1 if v is
// newer.
func (v Version) Compare(w Version) int {
	for _, d := range [3]int{v.Major - w.Major, v.Minor - w.Minor, v.Patch - w.Patch} {
		switch {
		case d < 0:
			return -1
		case d > 0:
			return 1
		}
	}
	return 0
}

// Less reports whether v is older than w.
func (v Version) Less(w Version) bool {
	return v.Compare(w) < 0
}

// String returns "MAJOR.MINOR.PATCH".
func (v Version) String() string {
	return strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Patch)
}

// MarshalText makes a Version a JSON string.
func (v Version) MarshalText() ([]byte, error) {
	return []byte(v.String()), nil
}

// UnmarshalText reads a Version from a JSON string with ParseVersion.
func (v *Version) UnmarshalText(text []byte) error {
	p, err := ParseVersion(string(text))
	if err != nil {
		return err
	}
	*v = p
	return nil
}

// Copied from github.com/jfreymuth/pulse v0.1.3 and changed for Radio Tutor. MIT License, Copyright (c) 2019 Johann Freymuth: see internal/core/pulse/LICENSE.

package proto

import (
	"reflect"
	"strconv"
)

// Version is a protocol version; the upper 16 bits are feature flags.
type Version uint32

// Version returns the version number without the flags.
func (v Version) Version() int { return int(v & 0xFFFF) }

// Min returns the lower version of v and u, with the flags both have.
func (v Version) Min(u Version) Version {
	flags := v & u & 0xFFFF0000
	v &= 0xFFFF
	if v > u&0xFFFF {
		v = u & 0xFFFF
	}
	return v | flags
}

// skipField reports whether a struct field of a protocol message is left out
// at the protocol version v. The field's `pulse` tag is "N" for a field that
// exists from version N on, and "<N" for a field that exists before version N.
func skipField(tag reflect.StructTag, v Version) bool {
	t := tag.Get("pulse")
	if t == "" {
		return false
	}
	if t[0] == '<' {
		ver, err := strconv.Atoi(t[1:])
		return err == nil && ver <= v.Version()
	}
	ver, err := strconv.Atoi(t)
	return err == nil && ver > v.Version()
}

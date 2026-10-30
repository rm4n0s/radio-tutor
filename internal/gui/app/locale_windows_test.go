//go:build windows

package app

import (
	"regexp"
	"testing"
)

func TestSystemLocale(t *testing.T) {
	got := systemLocale()
	if got == "" {
		t.Skip("this Windows reports no user locale")
	}
	if !regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]+)*$`).MatchString(got) {
		t.Errorf("systemLocale = %q, want a BCP 47 name such as el-GR", got)
	}
}

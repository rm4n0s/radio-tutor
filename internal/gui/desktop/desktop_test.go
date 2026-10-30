package desktop_test

import (
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/gui/desktop"
)

func TestScreensHasTheStudentHome(t *testing.T) {
	if desktop.Screens().Student == nil {
		t.Fatal("the student home is not wired")
	}
}

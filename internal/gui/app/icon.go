package app

import (
	"bytes"
	_ "embed" // the icon is embedded
	"image"
	"image/draw"
	"image/png"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/glutter/ui"
	xdraw "golang.org/x/image/draw"
)

// iconPNG is the icon of Radio Tutor, drawn by scripts/icongen. The same file is
// the icon of the Windows .exe (scripts/build-windows.sh).
//
//go:embed icon.png
var iconPNG []byte

// IconPNG returns the icon of Radio Tutor as a PNG file (256 by 256 pixels), the
// picture of the menu entry that radio-tutor -setup-desktop installs. The caller
// must not change the bytes.
func IconPNG() []byte { return iconPNG }

// iconSizes are the sizes of the pictures of the window icon, in pixels.
var iconSizes = []int{32, 48, 64, 128, 256}

// windowIcon returns the icon of the application window: the embedded picture at
// several sizes. On Windows the window takes the nearest one for the title bar and
// the taskbar; on Linux the compositor uses it when it supports window icons
// (Wayland's xdg_toplevel_icon, or the X11 window manager's _NET_WM_ICON) and
// otherwise takes the icon from the app's .desktop file.
func windowIcon() (ui.WindowIcon, error) {
	return iconFromPNG(iconPNG, iconSizes)
}

// iconFromPNG decodes data (a square PNG) and returns a window icon with one
// picture for each size in sizes.
func iconFromPNG(data []byte, sizes []int) (ui.WindowIcon, error) {
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return ui.WindowIcon{}, errors.NewErr("WindowIconDecode", err)
	}
	b := src.Bounds()
	if b.Dx() != b.Dy() || b.Dx() == 0 {
		return ui.WindowIcon{}, errors.New("WindowIconNotSquare", "the window icon picture must be square", "width", b.Dx(), "height", b.Dy())
	}
	var icon ui.WindowIcon
	for _, s := range sizes {
		dst := image.NewNRGBA(image.Rect(0, 0, s, s))
		if s == b.Dx() {
			draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
		} else {
			// The alpha is premultiplied while scaling; NRGBA is the straight alpha
			// that ui.WindowIconImage wants, and the conversion back is exact enough.
			xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
		}
		icon.Images = append(icon.Images, ui.WindowIconImage{Size: s, Scale: 1, RGBA: dst.Pix})
	}
	return icon, nil
}

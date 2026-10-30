//go:build !windows

package menuentry

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
	xdraw "golang.org/x/image/draw"
)

// iconSizes are the sizes, in pixels, of the icon pictures installed under
// icons/hicolor/<n>x<n>/apps.
var iconSizes = []int{48, 64, 128, 256}

// Modes of the folders and files of the menu entry: the desktop reads them, so
// they are world-readable, unlike Radio Tutor's own data.
const (
	menuDirMode  = 0o755
	menuFileMode = 0o644
)

// PlatformEnv is what the Linux part needs from the system.
type PlatformEnv struct {
	// Getenv looks up an environment variable (os.Getenv).
	Getenv func(string) string
}

func defaultPlatformEnv() PlatformEnv { return PlatformEnv{Getenv: os.Getenv} }

// dataHome returns $XDG_DATA_HOME, or ~/.local/share when it is empty or not an
// absolute path (the XDG Base Directory specification). It fails with the tag
// DataHomeUnknown.
func dataHome(p PlatformEnv) (string, error) {
	if v := p.Getenv("XDG_DATA_HOME"); filepath.IsAbs(v) {
		return v, nil
	}
	home := p.Getenv("HOME")
	if !filepath.IsAbs(home) {
		return "", errors.New("DataHomeUnknown", "cannot find the data folder: XDG_DATA_HOME is not set and HOME is not an absolute path", "home", home)
	}
	return filepath.Join(home, ".local", "share"), nil
}

// iconPath is the file of the icon picture of one size.
func iconPath(base, id string, size int) string {
	n := strconv.Itoa(size)
	return filepath.Join(base, "icons", "hicolor", n+"x"+n, "apps", id+".png")
}

// desktopPath is the Desktop Entry file.
func desktopPath(base, id string) string {
	return filepath.Join(base, "applications", id+".desktop")
}

// install writes the icons, then the Desktop Entry file that points at exe.
func install(ctx context.Context, e Entry, exe string, p PlatformEnv) ([]string, error) {
	base, err := dataHome(p)
	if err != nil {
		return nil, err
	}
	icons, err := scaleIcons(e.IconPNG, iconSizes)
	if err != nil {
		return nil, err
	}
	var written []string
	for i, size := range iconSizes {
		path := iconPath(base, e.ID, size)
		if err := storage.WriteFileAtomic(ctx, path, icons[i], menuFileMode, menuDirMode); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	path := desktopPath(base, e.ID)
	if err := storage.WriteFileAtomic(ctx, path, []byte(desktopEntry(e, exe)), menuFileMode, menuDirMode); err != nil {
		return written, err
	}
	return append(written, path), nil
}

// remove deletes the files install writes.
func remove(ctx context.Context, e Entry, p PlatformEnv) ([]string, error) {
	base, err := dataHome(p)
	if err != nil {
		return nil, err
	}
	paths := []string{desktopPath(base, e.ID)}
	for _, size := range iconSizes {
		paths = append(paths, iconPath(base, e.ID, size))
	}
	var removed []string
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return removed, errors.NewErr("RemoveCancelled", err)
		}
		err := os.Remove(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return removed, errors.NewErr("EntryFileRemoveFailed", err).SetMetadata("path", path)
		}
		removed = append(removed, path)
	}
	return removed, nil
}

// scaleIcons decodes the square PNG and returns it scaled to each size, as PNG
// files. It fails with the tags IconDecodeFailed, IconNotSquare, IconTooSmall and
// IconEncodeFailed.
func scaleIcons(data []byte, sizes []int) ([][]byte, error) {
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.NewErr("IconDecodeFailed", err)
	}
	b := src.Bounds()
	if b.Dx() != b.Dy() || b.Dx() == 0 {
		return nil, errors.New("IconNotSquare", "the icon picture must be square", "width", b.Dx(), "height", b.Dy())
	}
	largest := 0
	for _, s := range sizes {
		largest = max(largest, s)
	}
	if b.Dx() < largest {
		return nil, errors.New("IconTooSmall", "the icon picture is smaller than the largest icon size", "width", b.Dx(), "needed", largest)
	}
	out := make([][]byte, 0, len(sizes))
	for _, s := range sizes {
		dst := image.NewNRGBA(image.Rect(0, 0, s, s))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
		var buf bytes.Buffer
		if err := png.Encode(&buf, dst); err != nil {
			return nil, errors.NewErr("IconEncodeFailed", err).SetMetadata("size", s)
		}
		out = append(out, buf.Bytes())
	}
	return out, nil
}

// desktopEntry is the text of the Desktop Entry file (Desktop Entry
// Specification 1.5) that starts exe.
func desktopEntry(e Entry, exe string) string {
	var b strings.Builder
	line := func(key, value string) {
		b.WriteString(key + "=" + escapeValue(value) + "\n")
	}
	b.WriteString("[Desktop Entry]\n")
	line("Type", "Application")
	line("Version", "1.0")
	line("Name", e.Name)
	for _, lang := range sortedKeys(e.Names) {
		line("Name["+lang+"]", e.Names[lang])
	}
	if e.Comment != "" {
		line("Comment", e.Comment)
	}
	for _, lang := range sortedKeys(e.Comments) {
		line("Comment["+lang+"]", e.Comments[lang])
	}
	b.WriteString("Exec=" + escapeValue(execLine(exe)) + "\n")
	line("Icon", e.ID)
	line("Terminal", "false")
	if len(e.Categories) > 0 {
		line("Categories", strings.Join(e.Categories, ";")+";")
	}
	line("StartupWMClass", e.ID)
	return b.String()
}

// sortedKeys returns the keys of m in order, so that the file is the same on
// every run.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// execLine is the value of the Exec key for a program with no arguments, before
// the escaping of the file format: the path in double quotes, in which the
// Desktop Entry Specification wants a backslash before the characters ", `, $ and
// \ (it does not allow a quote to be written otherwise), and every percent sign
// doubled, because "%" starts a field code.
func execLine(path string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range path {
		switch r {
		case '"', '`', '$', '\\':
			b.WriteByte('\\')
		case '%':
			b.WriteByte('%')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

// escapeValue applies the escape sequences of the value of a key in a Desktop
// Entry file: \\ for a backslash, \n, \r and \t for the control characters. (This
// comes on top of the quoting of execLine, which is why a backslash in a path
// ends up as four backslashes in the file.)
func escapeValue(s string) string {
	return strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\r", `\r`, "\t", `\t`).Replace(s)
}

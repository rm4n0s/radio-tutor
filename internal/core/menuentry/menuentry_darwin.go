//go:build darwin

package menuentry

import (
	"bytes"
	"context"
	"encoding/binary"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
)

// On macOS the entry is an application bundle, <Name>.app, in the user's Applications folder: Launchpad, Spotlight and the Dock find
// it there. Its executable is a shell script that runs the program with the arguments it was given, so the bundle stays small and
// follows the program (run -setup-desktop again after the program moved). Its icon is an icns file of the PNG sizes below.

// iconSizes are the sizes, in pixels, of the pictures in the icns file; each has a chunk type (icp6 is 64, ic07 is 128, ic08 is 256).
var iconSizes = []int{64, 128, 256}

var icnsTypes = map[int]string{64: "icp6", 128: "ic07", 256: "ic08"}

// Modes of the folders and files of the bundle: the system reads them, so they are world-readable, unlike Radio Tutor's own data.
const (
	bundleDirMode  = 0o755
	bundleFileMode = 0o644
	bundleExecMode = 0o755
)

// PlatformEnv is what the macOS part needs from the system.
type PlatformEnv struct {
	// Getenv looks up an environment variable (os.Getenv).
	Getenv func(string) string
}

func defaultPlatformEnv() PlatformEnv { return PlatformEnv{Getenv: os.Getenv} }

// applicationsDir is ~/Applications. It fails with the tag ApplicationsDirUnknown when HOME is not an absolute path.
func applicationsDir(p PlatformEnv) (string, error) {
	home := p.Getenv("HOME")
	if !filepath.IsAbs(home) {
		return "", errors.New("ApplicationsDirUnknown", "cannot find the Applications folder: HOME is not an absolute path", "home", home)
	}
	return filepath.Join(home, "Applications"), nil
}

// bundlePath is the folder of the bundle: the name without the characters that a file name cannot hold.
func bundlePath(dir string, e Entry) string {
	name := strings.NewReplacer("/", "", ":", "", "\x00", "").Replace(e.Name)
	return filepath.Join(dir, strings.TrimSpace(name)+".app")
}

// install writes the icon, the Info.plist and, last, the launcher of the bundle that starts exe, and returns the files in that
// order. The bundle appears in Launchpad once its executable exists.
func install(ctx context.Context, e Entry, exe string, p PlatformEnv) ([]string, error) {
	dir, err := applicationsDir(p)
	if err != nil {
		return nil, err
	}
	pngs, err := scaleIcons(e.IconPNG, iconSizes)
	if err != nil {
		return nil, err
	}
	paths := bundleFiles(e, bundlePath(dir, e))
	data := [][]byte{icns(pngs), []byte(infoPlist(e)), []byte(launcher(exe))}
	modes := []os.FileMode{bundleFileMode, bundleFileMode, bundleExecMode}
	var written []string
	for i, path := range paths {
		if err := storage.WriteFileAtomic(ctx, path, data[i], modes[i], bundleDirMode); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	return written, nil
}

// bundleFiles are the files install writes, in the order it writes them.
func bundleFiles(e Entry, app string) []string {
	return []string{
		filepath.Join(app, "Contents", "Resources", e.ID+".icns"),
		filepath.Join(app, "Contents", "Info.plist"),
		filepath.Join(app, "Contents", "MacOS", e.ID),
	}
}

// remove deletes the files install writes and then the folders of the bundle that this leaves empty (a folder that holds anything else
// stays), and returns the files it deleted. A file that is not there is fine. It fails with the tags RemoveCancelled and
// EntryFileRemoveFailed.
func remove(ctx context.Context, e Entry, p PlatformEnv) ([]string, error) {
	dir, err := applicationsDir(p)
	if err != nil {
		return nil, err
	}
	app := bundlePath(dir, e)
	var removed []string
	for _, path := range bundleFiles(e, app) {
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
	for _, folder := range []string{
		filepath.Join(app, "Contents", "MacOS"), filepath.Join(app, "Contents", "Resources"), filepath.Join(app, "Contents"), app,
	} {
		_ = os.Remove(folder) // fails, and is right to, when something else is in it
	}
	return removed, nil
}

// launcher is the executable of the bundle: it hands over to the program, with the arguments it was given. An app that the Finder
// starts has no LANG, which the program reads to choose its language, so the script sets it from the language of the system
// (AppleLocale, "el_GR" for example) when it is empty.
func launcher(exe string) string {
	return `#!/bin/sh
if [ -z "$LANG" ]; then
  locale=$(/usr/bin/defaults read -g AppleLocale 2>/dev/null)
  if [ -n "$locale" ]; then LANG="${locale%%@*}.UTF-8"; export LANG; fi
fi
exec '` + strings.ReplaceAll(exe, "'", `'\''`) + `' "$@"
`
}

// infoPlist is the bundle's Info.plist.
func infoPlist(e Entry) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
`)
	pairs := [][2]string{
		{"CFBundleName", e.Name}, {"CFBundleDisplayName", e.Name}, {"CFBundleIdentifier", e.ID}, {"CFBundleExecutable", e.ID},
		{"CFBundleIconFile", e.ID}, {"CFBundlePackageType", "APPL"}, {"CFBundleVersion", "1"}, {"CFBundleShortVersionString", "1"},
		{"LSMinimumSystemVersion", "12.0"}, {"LSApplicationCategoryType", "public.app-category.education"},
	}
	if e.Comment != "" {
		pairs = append(pairs, [2]string{"NSHumanReadableCopyright", e.Comment})
	}
	for _, kv := range pairs {
		b.WriteString("\t<key>" + xmlEscape(kv[0]) + "</key>\n\t<string>" + xmlEscape(kv[1]) + "</string>\n")
	}
	b.WriteString("\t<key>NSHighResolutionCapable</key>\n\t<true/>\n</dict>\n</plist>\n")
	return b.String()
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

// icns makes an icns file of the PNG pictures, one chunk each (type, length, picture), all big-endian; pngs are in the order of iconSizes.
func icns(pngs [][]byte) []byte {
	var body bytes.Buffer
	for i, size := range iconSizes {
		body.WriteString(icnsTypes[size])
		_ = binary.Write(&body, binary.BigEndian, uint32(8+len(pngs[i]))) // writing to a buffer does not fail
		body.Write(pngs[i])
	}
	var out bytes.Buffer
	out.WriteString("icns")
	_ = binary.Write(&out, binary.BigEndian, uint32(8+body.Len())) // the same
	out.Write(body.Bytes())
	return out.Bytes()
}

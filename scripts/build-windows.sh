#!/usr/bin/env bash
# Builds Radio Tutor for Windows (docs/SPEC.md §4, README "Windows").
#
# Usage: scripts/build-windows.sh [-o dir] [-arch amd64,arm64] [-release] [-version 1.2.3.4]
#
#   -o dir        where the files go (default: dist/windows, git-ignored)
#   -arch list    architectures, comma separated: amd64, arm64 (default: amd64). arm64 is built
#                 but never run here (SPEC D20)
#   -release      a release build: -tags glutter_release and -ldflags "-s -w" (smaller, no
#                 debug information)
#   -version v    the four-part version of the file information (default: 0.0.0.<number of
#                 commits>; the commit shows in `git describe`, printed at the end)
#
# It makes two programs of the same package, ./cmd/radio-tutor, with `glutter-windows build` of
# Glutter (run from Radio Tutor's module with `go run`, so no tool is installed; Glutter's SPEC
# §25.1): the manifest (per-monitor DPI, Common Controls 6, UTF-8) and the icon
# (internal/gui/app/icon.png, drawn by scripts/icongen) are put into the .exe.
#
#   radio-tutor.exe       the application: a GUI program (no console window). Given a command it
#                         only says that the commands are in radio-tutor-cli.exe (cmd/radio-tutor
#                         guiexe.go)
#   radio-tutor-cli.exe   the same program as a console program, for the commands: validate, pack,
#                         reset-password, version (and the window, when given no command)
#
# With one architecture the files are <dir>/radio-tutor.exe and <dir>/radio-tutor-cli.exe; with
# several, glutter-windows adds the architecture: radio-tutor_amd64.exe, radio-tutor-cli_arm64.exe.
# Nothing else is needed on the target PC: no installer, no DLL, no runtime (an unsigned .exe makes
# Windows SmartScreen warn on the first run).
#
# Builds run through the machine's heavy-command slots (scripts/heavy.sh) so that two of them at
# most run at once; set RADIO_TUTOR_NO_HEAVY=1 when the caller holds a slot already.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

out=dist/windows
arch=amd64
release=()
version=""

usage() {
	echo "usage: $0 [-o dir] [-arch amd64,arm64] [-release] [-version 1.2.3.4]" >&2
	exit 64
}

while [ $# -gt 0 ]; do
	case "$1" in
	-o)
		[ $# -ge 2 ] || usage
		out="$2"
		shift 2
		;;
	-arch)
		[ $# -ge 2 ] || usage
		arch="$2"
		shift 2
		;;
	-release)
		release=(-release)
		shift
		;;
	-version)
		[ $# -ge 2 ] || usage
		version="$2"
		shift 2
		;;
	-h | -help | --help) usage ;;
	*) usage ;;
	esac
done

if [ -z "$version" ]; then
	count="$(git rev-list --count HEAD 2>/dev/null || echo 0)"
	version="0.0.0.$((count % 65536))"
fi

mkdir -p "$out"

wrap=()
if [ -z "${RADIO_TUTOR_NO_HEAVY:-}" ]; then
	wrap=(scripts/heavy.sh --)
fi

tool=github.com/rm4n0s/glutter/cmd/glutter-windows
common=(-name "Radio Tutor" -app-id dev.radiotutor.app -icon internal/gui/app/icon.png
	-version "$version" -arch "$arch")

# glutter-windows builds with CGO_ENABLED=0 itself; the environment of this script must not turn
# cgo on for it or point the build at another OS.
unset GOOS GOARCH

"${wrap[@]}" go run "$tool" build "${release[@]}" "${common[@]}" -o "$out/radio-tutor.exe" ./cmd/radio-tutor
"${wrap[@]}" go run "$tool" build "${release[@]}" "${common[@]}" -console -o "$out/radio-tutor-cli.exe" ./cmd/radio-tutor

echo "version: $version ($(git describe --tags --always --dirty 2>/dev/null || echo unknown))"
ls -l "$out"/radio-tutor*.exe

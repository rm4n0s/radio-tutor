#!/usr/bin/env bash
# Runs the tests of Radio Tutor built for Windows, under Wine (docs/SPEC.md §16):
# GOOS=windows GOARCH=amd64 go test -exec "$PWD/scripts/wine-exec.sh" -p 1.
# Adapted from Glutter's scripts/test-windows.sh. Unlike Glutter's, it runs EVERY
# package, not only those with Windows files: the whole app must work on Windows.
#
# Usage: scripts/test-windows.sh [go test flags and packages]
#        (default: ./...). A full run is heavy and slow: start it through
#        scripts/heavy.sh -- scripts/test-windows.sh. While working, run one package:
#        scripts/test-windows.sh ./internal/core/storage/...
#
# The tests run one package at a time (-p 1): they share one prefix and one Wine server. The
# prefix is .wine-test/prefix at the repository root (scripts/wine-exec.sh); the Wine audio
# drivers are disabled there, so nothing plays on the speakers. Tests that need a real window
# or a real sound device carry the radiotutor_display and radiotutor_audio tags and are not
# run by this script.
#
# Needs Wine and Xvfb (sudo dnf install wine-core xorg-x11-server-Xvfb). Exit status: go test's;
# 69 when a tool is missing.
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"

if ! command -v wine64 >/dev/null 2>&1 && ! command -v wine >/dev/null 2>&1; then
	echo "test-windows.sh: Wine is needed (sudo dnf install wine-core)" >&2
	exit 69
fi
if ! command -v Xvfb >/dev/null 2>&1; then
	echo "test-windows.sh: Xvfb is needed (sudo dnf install xorg-x11-server-Xvfb)" >&2
	exit 69
fi

args=("$@")
if [[ ${#args[@]} -eq 0 ]]; then
	args=(./...)
fi

# A debug build of Glutter (-tags debug_mode): since Glutter 1.17 a build without tags is a release
# build, and the GUI tests need its debug-only platform override (internal/gui/testutil.UseLinux).
# A -tags among the arguments comes later and wins.
exec env GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
	go test -exec "$ROOT/scripts/wine-exec.sh" -p 1 -tags debug_mode "${args[@]}"

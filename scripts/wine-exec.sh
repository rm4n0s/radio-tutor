#!/usr/bin/env bash
# Runs a Windows program under Wine on a headless X server (docs/SPEC.md §16). It is the runner
# for `go test -exec` (scripts/test-windows.sh). Adapted from Glutter's scripts/wine-exec.sh.
#
# Usage: scripts/wine-exec.sh <program.exe> [args...]
#        GOOS=windows go test -exec "$PWD/scripts/wine-exec.sh" -p 1 ./internal/core/storage/...
#
# What it does:
#   - starts a headless X server (scripts/headless-x11.sh, WAYLAND_DISPLAY unset), unless one is
#     given already (RADIO_TUTOR_WINE_INSIDE=1);
#   - uses a private prefix, .wine-test/prefix at the repository root (git-ignored);
#     RADIO_TUTOR_WINEPREFIX moves it. Nothing of the user's ~/.wine is read or written. A new prefix is made on the first
#     run, under a lock: its first boot hangs when the wine-cms package is not installed (setupapi
#     loops on the missing mscms.dll), so it is cut after 20 seconds and the server is stopped;
#   - sets WINEDLLOVERRIDES="mscoree,mshtml=;winepulse.drv,winealsa.drv,wineoss.drv=d" (no Mono or
#     Gecko prompt, and NO AUDIO DRIVER: a test never plays on the speakers of the machine, the
#     waveOut of Wine then has no device) with PULSE_SERVER=unix:/nonexistent as a second guard,
#     WINEDEBUG=-all (RADIO_TUTOR_WINEDEBUG overrides it), WINEARCH=win64, and VK_DRIVER_FILES to
#     lavapipe's ICD (the only Vulkan driver that presents on Xvfb) unless it is set;
#   - lowers the core-file limit (ulimit -c 1), so that a crash of Wine never writes a core file or
#     reaches the system's crash reporter;
#   - runs the program under `timeout -k 10` (RADIO_TUTOR_WINE_TIMEOUT seconds, 600 by default);
#   - stops Wine with `wineserver -k` at the end (never `wineserver -w`: winedevice stays alive).
#
# The tests of different packages share the prefix and the server, so run one package at a time
# (go test -p 1).
#
# Exit status: the program's own (124 when the timeout ended it); 69 when Wine or Xvfb is not
# installed; 64 for a usage mistake.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <program.exe> [args...]" >&2
  exit 64
fi

WINE="$(command -v wine64 || command -v wine || true)"
WINESERVER="$(command -v wineserver || true)"
if [[ -z "$WINE" || -z "$WINESERVER" ]]; then
  echo "error: Wine is not installed." >&2
  echo "       install it with: sudo dnf install wine-core" >&2
  exit 69
fi
if ! command -v Xvfb >/dev/null 2>&1; then
  echo "error: Xvfb is not installed." >&2
  echo "       install it with: sudo dnf install xorg-x11-server-Xvfb" >&2
  exit 69
fi

PREFIX="${RADIO_TUTOR_WINEPREFIX:-$ROOT/.wine-test/prefix}"
export WINEPREFIX="$PREFIX"
export WINEARCH=win64
# Audio drivers are always disabled (the owner's rule: nothing run by a test or a check may play on the
# speakers). Do not make this overridable.
export WINEDLLOVERRIDES="mscoree,mshtml=;winepulse.drv,winealsa.drv,wineoss.drv=d"
export PULSE_SERVER="unix:/nonexistent"
unset PIPEWIRE_REMOTE
export WINEDEBUG="${RADIO_TUTOR_WINEDEBUG:--all}"
unset WAYLAND_DISPLAY WAYLAND_SOCKET
if [[ -z "${VK_DRIVER_FILES:-}" && -f /usr/share/vulkan/icd.d/lvp_icd.x86_64.json ]]; then
  export VK_DRIVER_FILES=/usr/share/vulkan/icd.d/lvp_icd.x86_64.json
fi
ulimit -c 1

# Outside the X server: start it and run this script again inside it.
if [[ "${RADIO_TUTOR_WINE_INSIDE:-}" != 1 ]]; then
  export RADIO_TUTOR_WINE_INSIDE=1
  exec "$ROOT/scripts/headless-x11.sh" -- "$0" "$@"
fi

# server_gone waits (at most 15 s) until no wineserver of this prefix is left. `wineserver -k` only asks
# the server to end; a run that starts while the server of the run before is still ending fails at
# once with status 5 before any of its code runs. (Never `wineserver -w`: it can wait for ever.)
server_gone() {
  local i pid
  for i in $(seq 60); do
    local found=""
    for pid in $(pgrep -u "$(id -u)" -x wineserver 2>/dev/null); do
      if tr '\0' '\n' < "/proc/$pid/environ" 2>/dev/null | grep -qx "WINEPREFIX=$PREFIX"; then
        found=1
      fi
    done
    [[ -z "$found" ]] && return 0
    sleep 0.25
  done
}
stop() { "$WINESERVER" -k >/dev/null 2>&1 || true; server_gone; }
trap stop EXIT
server_gone

if [[ ! -f "$PREFIX/system.reg" ]]; then
  mkdir -p "$(dirname "$PREFIX")"
  (
    flock 9
    if [[ ! -f "$PREFIX/system.reg" ]]; then
      timeout -k 5 20 "$WINE" wineboot --init >/dev/null 2>&1
      stop
    fi
  ) 9>"$PREFIX.lock"
fi

# The prefix of Wine has no font directory, and an app that scans the fonts of Windows would draw no
# text: give it a copy of the host's Liberation Sans, the way a Windows machine has C:\Windows\Fonts.
FONTS="$PREFIX/drive_c/windows/Fonts"
if ! compgen -G "$FONTS/*.ttf" >/dev/null; then
  mkdir -p "$FONTS"
  for f in /usr/share/fonts/liberation-sans-fonts/*.ttf /usr/share/fonts/liberation/LiberationSans-*.ttf; do
    [[ -f "$f" ]] && cp "$f" "$FONTS/"
  done
fi

# The program runs in the background and this script waits for it, so that a signal is handled at
# once (bash runs a trap only when its foreground child ends): TERM and INT end the program, then
# the trap of EXIT stops Wine.
child=""
on_signal() {
  [[ -n "$child" ]] && kill -TERM "$child" 2>/dev/null
  [[ -n "$child" ]] && wait "$child" 2>/dev/null
  exit "$1"
}
trap 'on_signal 143' TERM
trap 'on_signal 130' INT
timeout -k 10 "${RADIO_TUTOR_WINE_TIMEOUT:-600}" "$WINE" "$@" &
child=$!
wait "$child"
status=$?
child=""
exit "$status"

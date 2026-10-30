#!/usr/bin/env bash
# Runs a command under a headless X server (Xvfb) and stops the server
# afterwards. It is how the Wine tests run without a real screen
# (scripts/wine-exec.sh). Adapted from Glutter's scripts/headless-x11.sh.
#
# Usage: scripts/headless-x11.sh -- <command> [args...]
#
# Inside the command:
#   DISPLAY           the display of the headless server (a free one is chosen)
#   WAYLAND_DISPLAY   unset, so nothing falls back to a Wayland session
#
# There is no window manager, so windows are not decorated and open at the
# position and size they ask for. The server's own output goes to
# $RUNTIME/xvfb.log, which is printed if it fails to start. Environment:
#   HEADLESS_SIZE   screen size, WIDTHxHEIGHT (default 1280x800)
#   HEADLESS_DEPTH  color depth in bits (default 24)
#
# Needs Xvfb: sudo dnf install xorg-x11-server-Xvfb
#
# Exit status: the command's own; 2 when Xvfb is not installed; 3 when it did not
# start; 64 for a usage mistake.
set -euo pipefail

usage() {
  echo "usage: $0 -- <command> [args...]" >&2
  exit 64
}

[[ "${1:-}" == "--" ]] || usage
shift
[[ $# -gt 0 ]] || usage

if ! command -v Xvfb >/dev/null 2>&1; then
  echo "error: Xvfb is not installed." >&2
  echo "       install it with: sudo dnf install xorg-x11-server-Xvfb" >&2
  exit 2
fi

size="${HEADLESS_SIZE:-1280x800}"
depth="${HEADLESS_DEPTH:-24}"

runtime="$(mktemp -d "${TMPDIR:-/tmp}/radio-tutor-x11.XXXXXX")"
chmod 700 "$runtime"
log="$runtime/xvfb.log"
pid=""

cleanup() {
  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null || true
    for _ in 1 2 3 4 5 6 7 8 9 10; do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.1
    done
    kill -9 "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  fi
  rm -rf "$runtime"
}
trap cleanup EXIT

# -displayfd makes Xvfb pick a free display number and write it to the file
# descriptor once the server is ready, so concurrent runs do not collide.
Xvfb -displayfd 3 -screen 0 "${size}x${depth}" -nolisten tcp -noreset \
  3>"$runtime/display" >"$log" 2>&1 &
pid=$!

number=""
for _ in $(seq 1 100); do
  if ! kill -0 "$pid" 2>/dev/null; then
    break
  fi
  if [[ -s "$runtime/display" ]]; then
    number="$(head -n1 "$runtime/display")"
    break
  fi
  sleep 0.1
done

if [[ -z "$number" ]]; then
  echo "error: Xvfb did not start. Its output:" >&2
  sed 's/^/  /' "$log" >&2 || true
  exit 3
fi

export DISPLAY=":$number"
unset WAYLAND_DISPLAY WAYLAND_SOCKET

# Run the command and keep its exit status; the trap stops the server.
set +e
"$@"
status=$?
exit "$status"

#!/usr/bin/env bash
# Usage: scripts/isolated.sh [--keep] -- <command> [args...]
#
# Runs a command with fresh XDG_CONFIG_HOME, XDG_DATA_HOME, XDG_STATE_HOME and
# XDG_CACHE_HOME folders under a temporary folder, so that it never touches the
# user's real Radio Tutor data (SPEC §16). The folder is deleted when the command
# ends, unless --keep is given (then its path is printed to stderr). The exit
# code is the command's.
set -euo pipefail

usage() {
	echo "usage: isolated.sh [--keep] -- <command> [args...]" >&2
	exit 2
}

keep=0
if [ "${1:-}" = "--keep" ]; then
	keep=1
	shift
fi
[ "${1:-}" = "--" ] || usage
shift
[ "$#" -gt 0 ] || usage

root=$(mktemp -d "${TMPDIR:-/tmp}/radio-tutor-isolated.XXXXXX")

cleanup() {
	if [ "$keep" -eq 1 ]; then
		echo "isolated.sh: kept $root" >&2
	else
		rm -rf "$root"
	fi
}
trap cleanup EXIT

mkdir -p "$root/config" "$root/data" "$root/state" "$root/cache"
export XDG_CONFIG_HOME="$root/config"
export XDG_DATA_HOME="$root/data"
export XDG_STATE_HOME="$root/state"
export XDG_CACHE_HOME="$root/cache"

# Not exec: the trap must run after the command.
status=0
"$@" || status=$?
exit "$status"

#!/usr/bin/env bash
# Usage: scripts/heavy.sh -- <command> [args...]
#
# Runs a heavy command (a full test run, golangci-lint, a release build) while
# holding one of two machine-wide slots, so that at most two heavy commands run
# at the same time on this machine, whichever agent or shell started them
# (CLAUDE.md "Memory (OOM)"). The slots are flock locks on
# /tmp/radio-tutor-heavy.1.lock and /tmp/radio-tutor-heavy.2.lock. The script
# waits until one is free and then replaces itself with the command, so the
# command's exit code and signals are the script's own.
set -euo pipefail

usage() {
	echo "usage: heavy.sh -- <command> [args...]" >&2
	exit 2
}

[ "${1:-}" = "--" ] || usage
shift
[ "$#" -gt 0 ] || usage

slots=(/tmp/radio-tutor-heavy.1.lock /tmp/radio-tutor-heavy.2.lock)

while true; do
	for slot in "${slots[@]}"; do
		# The descriptor stays open in the command (exec below) and so holds the
		# lock until the command exits.
		exec 9>"$slot"
		if flock -n 9; then
			exec "$@"
		fi
		exec 9>&-
	done
	sleep 1
done

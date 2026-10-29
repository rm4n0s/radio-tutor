#!/usr/bin/env bash
# Usage: scripts/error-tags.sh [--untested]
#
# Lists every Tag given to errors.New, errors.NewErr and errors.NewErrf in the
# non-test Go files of cmd and internal, one per line, sorted and unique.
#
# With --untested it prints only the Tags that appear in no _test.go file and are
# not listed in docs/ERROR-TAGS-UNTESTED.md (the only exception list). Every
# error Tag needs a test that asserts its route (CLAUDE.md "Error handling"), so
# a clean run prints nothing.
set -euo pipefail

untested=0
case "${1:-}" in
"") ;;
--untested) untested=1 ;;
*)
	echo "usage: error-tags.sh [--untested]" >&2
	exit 2
	;;
esac
[ "$#" -le 1 ] || {
	echo "usage: error-tags.sh [--untested]" >&2
	exit 2
}

cd "$(dirname "${BASH_SOURCE[0]}")/.."

dirs=()
for d in cmd internal; do
	[ -d "$d" ] && dirs+=("$d")
done

if [ "${#dirs[@]}" -eq 0 ]; then
	exit 0
fi

# Tags of the constructors in the non-test files.
tags() {
	# -z reads each file as one line, so that a call split over several lines
	# (the tag on the line after "errors.New(") is found too.
	grep -rhPzo --include='*.go' --exclude='*_test.go' \
		'errors\.(New|NewErr|NewErrf)\(\s*"[A-Za-z0-9_]+"' "${dirs[@]}" 2>/dev/null |
		tr '\0\n' '  ' | grep -oE '"[A-Za-z0-9_]+"' | tr -d '"' | sort -u || true
}

if [ "$untested" -eq 0 ]; then
	tags
	exit 0
fi

# Names that appear in the exception list: the first word of each list item or
# table row, or any quoted/backticked identifier.
excepted=""
if [ -f docs/ERROR-TAGS-UNTESTED.md ]; then
	excepted=$(grep -oE '[A-Za-z][A-Za-z0-9_]*' docs/ERROR-TAGS-UNTESTED.md | sort -u || true)
fi

while IFS= read -r tag; do
	[ -n "$tag" ] || continue
	# A test mentions the tag as a word, in a route such as "F.Tag" or a string.
	if grep -rqwE --include='*_test.go' "$tag" "${dirs[@]}" 2>/dev/null; then
		continue
	fi
	if [ -n "$excepted" ] && grep -qxF "$tag" <<<"$excepted"; then
		continue
	fi
	echo "$tag"
done < <(tags)

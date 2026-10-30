#!/usr/bin/env bash
# Usage: scripts/make-sample-cassette.sh
#
# Rebuilds the sample cassette (testdata/cassettes/sample.cassette) from its
# sources:
#
#   testdata/cassettes/sample-src/script.json        the narration sentences and pauses
#   testdata/cassettes/sample-src/lessons/<id>.json  lesson.json templates (key times as
#                                                    anchors to the sentences, see
#                                                    scripts/samplegen/main.go)
#   testdata/cassettes/sample/                       the cassette folder: manifest, cover,
#                                                    scenes, exercises (written by hand) and
#                                                    the generated lesson.json, narration.ogg
#                                                    and prompt.ogg files
#
# The script speaks every sentence with espeak-ng (-v en-us -s 140), measures the
# files with ffprobe, joins them with ffmpeg and encodes Ogg Vorbis (mono, 22050
# Hz, -q:a 2 to keep the repository small), writes the captions and the key
# times into the lesson.json files, then validates and packs the folder.
#
# The sources of the cassette folder live outside it because a cassette may only
# hold the files of CASSETTE-FORMAT §1 (script.json and the templates would be
# errors). espeak-ng's output can change between versions, so the committed
# .ogg and lesson.json files are not expected to be reproduced byte for byte.
# Needs: espeak-ng, ffmpeg and ffprobe (with libvorbis) and go.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

for tool in espeak-ng ffmpeg ffprobe go; do
	command -v "$tool" >/dev/null || {
		echo "make-sample-cassette.sh: $tool is not installed" >&2
		exit 1
	}
done

src=testdata/cassettes/sample-src
dir=testdata/cassettes/sample
out=testdata/cassettes/sample.cassette

go run ./scripts/samplegen -src "$src" -out "$dir"
go run ./cmd/radio-tutor validate "$dir"
go run ./cmd/radio-tutor pack --force "$dir" "$out"
go run ./cmd/radio-tutor validate "$out"

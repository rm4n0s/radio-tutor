# Test data of the archive package

`valid/` is a complete, valid cassette (id `org.example.archive-1`, version
1.0.0): the JSON documents of `internal/core/cassette/testdata/minimal` with the
times and captions shortened to fit the audio, plus the files the archive
checks: `cover.svg`, a `scene.svg` for each lesson (with the actors its tracks
use), a `prompt.svg` and a `prompt.ogg` for one exercise, and a
`narration.ogg` for each lesson. Tests copy it into memory, break it in one
place and pack or import it. Hostile zips are built inside the tests.

The audio was made once with ffmpeg (libvorbis) and is committed; the tests do
not need ffmpeg. Every file is a mono 22050 Hz sine tone (the narrations are 3
seconds long, the prompt is 1 second):

```sh
mk() { # file duration rate frequency
  ffmpeg -v error -y -f lavfi -i "sine=frequency=$4:duration=$2:sample_rate=$3" \
    -ac 1 -ar $3 -map_metadata -1 -fflags +bitexact -flags:a +bitexact \
    -c:a libvorbis -q:a 0 "$1"
}
mk valid/lessons/what-is-a-fraction/narration.ogg 3 22050 440
mk valid/lessons/adding-fractions/narration.ogg   3 22050 330
mk valid/exercises/shade-the-half/prompt.ogg      1 22050 220
```

If the narration length changes, change the `t` and `end` values of the
`lesson.json` files so that they stay within it (CASSETTE-FORMAT §6).

# Test audio

Small Ogg Vorbis files for the tests of `internal/core/audio`. They were made
once with ffmpeg (libvorbis) and are committed; the tests do not need ffmpeg.
Every tone is a 1 second, 440 Hz sine (ffmpeg's sine amplitude is 1/8).

```sh
mk() { # file rate channels
  ffmpeg -v error -y -f lavfi -i "sine=frequency=440:duration=1:sample_rate=$2" \
    -ac $3 -ar $2 -map_metadata -1 -fflags +bitexact -flags:a +bitexact \
    -c:a libvorbis -q:a 2 -page_duration 200000 "$1"
}
mk tone-mono-44100.ogg   44100 1
mk tone-stereo-48000.ogg 48000 2
mk tone-mono-22050.ogg   22050 1
mk tone-mono-16000.ogg   16000 1   # an unsupported rate (CASSETTE-FORMAT §6)

# Corrupted: four bytes overwritten inside the third audio page (the page
# checksum no longer matches).
cp tone-mono-44100.ogg tone-mono-44100-corrupt.ogg
printf '\xff\xff\xff\xff' | dd of=tone-mono-44100-corrupt.ogg bs=1 seek=4000 conv=notrunc

# Truncated in the middle of the third audio page.
head -c 4000 tone-mono-44100.ogg > tone-mono-44100-truncated.ogg
```

`-page_duration 200000` makes a page every 0.2 s, so that the corruption and
the truncation are in the audio data, not in the headers (the decoder reads
the headers and the first audio page when it opens the file). Other broken
inputs (no Ogg data, a page with no segments, a patched identification
header) are built inside the tests.

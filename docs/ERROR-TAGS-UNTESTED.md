# Error tags without a route test

The only exceptions to "every error Tag has a test that asserts its route" (CLAUDE.md, Definition of done).
`scripts/error-tags.sh --untested` skips the tags named here.

- `GlutterSinkResumeFailed` (`internal/gui/app/audio_glutter.go`, `glutterSink.resumeLocked`): the sink holds Glutter's
  `audio.Device`, and Glutter's test speaker (`gluttertest.TestAudio`, the fake platform's device) never fails `Resume`,
  so a test cannot make the device refuse. Remove this entry when gluttertest can script that failure.

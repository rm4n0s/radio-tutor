# Implemented steps

Finished steps of `docs/IMPLEMENTATION.md`, newest last.

## 0.1 Scaffold
Date: 2026-10-30.

Built:
- `go.mod`: module `github.com/rm4n0s/radio-tutor`, `go 1.27`, with `github.com/rm4n0s/glutter v1.9.5` and `github.com/rm4n0s/errors v0.2.0` (and `golang.org/x/sys`, `github.com/go-text/typesetting`, `golang.org/x/image`, `golang.org/x/text`, `github.com/godbus/dbus/v5`, `github.com/ebitengine/purego`, which Glutter brings in).
- `cmd/radio-tutor`: `main.go` is thin (signal context, `os.Exit(run(...))`). `run(ctx, args, stdout, stderr, startGUI) int` in `run.go` dispatches: no arguments call `startGUI` (`app.Run`); `validate`, `pack` and `reset-password` print "not implemented yet" to stderr and exit 2; `version` prints the module version of `debug.ReadBuildInfo` (`unknown` without build info); `help`, `-h`, `--help` print the usage; anything else is a usage error (exit 2). A failing GUI start exits 1. A failed write to stdout or stderr turns an exit code 0 into 1 (`printer`), so no returned error is ignored.
- `internal/gui/app`: `Root` (`NewRoot()`), a Material app whose home is a `Scaffold` with "Radio Tutor" centered, and `Run()` which opens the window (title "Radio Tutor", initial and minimum size 800x600, app id `dev.radiotutor.app`) through `material.RunApp`.
- `internal/archtest`: `Scan(fs.FS)` reads the import clauses of every Go file of the module with `go/parser` (`ImportsOnly`; it skips `testdata`, `vendor`, hidden and `_` folders, and needs no package to compile or exist), `Check([]File)` applies the rules of SPEC §5: the core (files under `internal/core`, tests included) never imports Glutter or `internal/gui`; no core package other than `grading/openai`, `providers` and `phone` imports `net/http` in non-test files; `os/exec` is imported nowhere except by a `_test.go` file of `internal/core/audio` carrying the `radiotutor_audio` build tag. `TestModuleBoundaries` runs both over the real module.
- `.golangci.yml` (golangci-lint v2 syntax): the `standard` set plus `errcheck`, `govet`, `staticcheck`, `unused`, `ineffassign`, `misspell`, `gocritic`; `forbidigo` forbids `fmt.Errorf`; `depguard` denies the standard library `errors` package (checked with a throwaway file).
- `scripts/heavy.sh`, `scripts/error-tags.sh`, `scripts/isolated.sh` (bash, `set -euo pipefail`). `heavy.sh` tries `flock -n` on `/tmp/radio-tutor-heavy.1.lock` and `.2.lock` in turn, sleeps one second when both are busy, and `exec`s the command so its exit code and signals pass through. `error-tags.sh` finds tags with `grep -Pz` (so a call split over lines is found too); `--untested` also reads `docs/ERROR-TAGS-UNTESTED.md` (any identifier in it counts as excepted) and prints nothing when there are no tags. `isolated.sh [--keep] -- <command>` gives the command fresh `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME` and `XDG_CACHE_HOME` under a temporary folder and removes it afterwards.
- `.gitignore`: `/radio-tutor` and `*.cassette` outside any `testdata` folder.

Tests:
- `cmd/radio-tutor`: table-driven dispatch (no arguments, GUI failure, cancelled context, the three unimplemented commands, `version`, `help`, unknown command and usage errors exit 2), write failures, `versionOf`.
- `internal/gui/app`: a `gluttertest` test pumps `NewRoot()` at the minimum and a larger size and finds the text "Radio Tutor" once.
- `internal/archtest`: table-driven `Check` for each rule, `Scan` over an in-memory module, route tests for the tags `ScanWalkFailed`, `ScanReadFailed`, `ScanParseFailed`, and the real module.

Deviations from the spec: none. `go.mod` says `go 1.27` (the toolchain is go1.27.3), as Glutter's does.

Manual checks: none. The window itself was not opened on a display; it is covered by the `gluttertest` test only.

Later:
- `radiotutor_display` test that really opens the window.
- `run` takes the GUI starter as a parameter; later steps pass the storage paths and logger to it.

## 4.2 QR codes
Date: 2026-10-30.

Built:
- `go.mod`: `rsc.io/qr v0.2.0` (SPEC D14).
- `internal/core/qrcode`: `Encode(text string, level Level) (Matrix, error)`. `Level` is the package's own type (`LevelL`, `LevelM`, `LevelQ`, `LevelH`, `DefaultLevel = LevelM`) mapped to the library's levels inside `Encode`. `Matrix` holds `Size` (modules per side, without the quiet zone) and a private row-major `[]bool`; `Dark(x, y)` is false outside the matrix, `Total()` is `Size + 2*QuietZone` (`QuietZone = 4`), `DarkQuiet(x, y)` addresses the matrix with the quiet zone included, and `String()` renders `#`/`.` rows. No `rsc.io/qr` type appears in the API. The library always picks the smallest version that fits, with mask 0 (its own TODO), and its codes carry no quiet zone, so the quiet zone is explicit here.
- Error tags: `QRTextEmpty`, `QRLevelInvalid`, `QRTextTooLong` (text longer than version 40 holds).

Tests (`go test ./internal/core/qrcode`):
- Matrix size for the phone URL shape (`http://192.168.100.200:8737/s/<26-char token>`, 56 bytes: version 4, 33 modules, at L and M; 41 at H) and for numeric text; size is always 17+4v.
- All three 7x7 finder patterns, their separators and the timing row at every level.
- Determinism; `Dark` and `DarkQuiet` bounds and an all-light quiet zone; zero `Matrix`; `String` agrees with `Dark`; `Level.String`.
- Route tests for the three tags.
- `TestGoldenMatrix`: the fixed URL at M and H against `testdata/goldens/phone_url_{m,h}.txt`. Update with `go test ./internal/core/qrcode -run Golden -update-goldens` (the package defines the `-update-goldens` flag itself; `go test ./... -run Golden -update-goldens` fails in packages that do not define it, so every package with goldens must).

Deviations from the spec: none. `Encode` returns the module matrix (the plan allowed this or an `*image.Gray`).

Manual checks: scanning the QR code of a real URL with a real phone is left to the step that shows the dialog (M6/M7); no scanner was available here.

Later:
- The GUI widget that draws a `Matrix` (modules scaled to at least 280 logical pixels, quiet zone included).
## 1.1 Timeline model and evaluation
Date: 2026-10-30.

Built:
- `internal/core/problem`: `Problem{Path, Message string; Warning bool}` (JSON tags `path`, `message`, `warning,omitempty`, for `validate --json`), `String()` (`path: message`, `warning: ` prefix for warnings), `Errorf`/`Warnf` constructors, `List` (`[]Problem` with `Add`, `Errorf`, `Warnf`, `HasErrors`; zero value ready, assignable to `[]Problem`), `HasErrors`, `Errors`, `Warnings`, path helpers `Join` and `Index`, and `Prefixed(prefix, ps)` / `InFile(file, ps)` which return prefixed copies. Convention for later validators: a validator builds paths relative to what it validates (`tracks[2].keys[5].opacity`) and returns `[]problem.Problem`; the caller that knows the file applies `InFile`, which writes `lessons/l1/lesson.json#tracks[2].keys[5].opacity`, so the printed form is `file#field.path: message`. A cassette is valid when `HasErrors` is false.
- `internal/core/timeline`: `Track`, `Key` (optional properties are `*float64`; `Float(v)` helper), `Caption`, `Ease` and `RevealFrom` string types with constants, `Valid()`, `RevealFrom.Or()`; `ActorState` and `DefaultState()`; `Ease.Apply(u)` (linear, cubic in/out/in-out, step); `Evaluator` (`NewEvaluator(tracks)`, `Len`, `Actor(i)`, `Evaluate(t, dst) []ActorState`) and the convenience `Evaluate(tracks, t) []ActorState` (track order); `CaptionAt(captions, t)`; `Validate(tracks, captions, length) []problem.Problem`.
- Evaluation design: the player builds an `Evaluator` once per lesson; it splits every track's keys per property (with the key's ease), so a frame does a binary search per property and writes into a slice the caller owns and reuses (zero allocations, asserted with `testing.AllocsPerRun`). Per property: before the first key that sets it the first value, between two keys the later key's ease, after the last the last value, unset properties keep their defaults (opacity 1, x 0, y 0, scale 1, rotate 0, reveal 1). The Evaluator is immutable, so it is safe to share. The player applies the transforms in the spec's order (scale and rotate around `origin`, then move).
- `CaptionAt` shows a caption for `start <= t < end` (end exclusive, so two touching captions never both match); captions must be ordered (binary search).
- `Validate` rules: actor present and unique per track; `origin` two finite numbers and present when a key sets scale or rotate; `reveal_box` four finite numbers, positive width and height, present when a key sets reveal; `reveal_from` valid; 1 to 1000 keys with strictly increasing `t` in `[0, length+0.05]`; opacity and reveal in 0 to 1; scale in (0, 20]; x, y, rotate finite; ease valid; at least one caption; text 1 to 200 characters (runes), not blank, at most two lines; `start >= 0`, `start < end`, `end <= length+0.05`; ordered and not overlapping (touching is fine). Not here (they need other packages): the actor exists in the scene, `reveal_box` inside the view box.

Tests:
- `problem`: `String`, `Join`/`Index`, `List` and the filters, `Prefixed`/`InFile` (input unchanged), JSON.
- `timeline`: easing values and monotonicity; table-driven evaluation (before first key, at the first key, linear and each ease between keys, the later key's ease only, step on both sides of the key, after the last key, properties set by different keys, zero versus absent, several tracks, all six properties); zero allocations and snapshot behavior of the Evaluator; 1000 keys; unordered keys do not panic; `CaptionAt` at every boundary; one test per validation rule with its path and message, all problems collected at once, the 0.05 s tolerance.
- No error Tags are introduced (validators return problems, not errors).

Deviations from the spec: none. Two small readings of the spec: a caption text that is blank (only spaces or newlines) is an error, and more than two lines (more than one `\n`) is an error ("at most two short lines").

Manual checks: none.

Later:
- `Key.T`, `Caption.Start` and `Caption.End` are plain `float64` (absent reads as 0, as `encoding/json` cannot tell); if a missing `t` or `start` should be an error, the cassette package can check for the key during decoding (for example with a second pass into `map[string]json.RawMessage`).
- Validate does not warn about a key that sets no property (allowed by the spec).

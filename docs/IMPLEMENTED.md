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
## 0.2 Storage and logging
Date: 2026-10-30.

Built:
- `internal/core/storage`: `Paths{Config, Data, State}` (the three `radio-tutor` folders of SPEC §4) with `Resolve()` (process environment) and `ResolveFrom(getenv)`; an empty or relative `XDG_*` value is ignored (XDG specification) and the default under `$HOME` is used; no usable `HOME` fails with `HomeUnknown`. Resolving creates nothing. File helpers on `Paths`: `SettingsFile`, `ProvidersFile`, `ProfilesFile`, `CassettesDir`, `ProgressDir`, `PhotosDir`, `LogFile`, `LockFile`. `DirMode` (0700) and `FileMode` (0600), `EnsureDir(path)` (`DirCreateFailed`).
- `WriteJSONAtomic(ctx, path, v)`: indented JSON plus a newline; temporary file in the same folder (mode 0600), write, `fsync`, close, rename, `fsync` of the folder; the folder is created when missing; any failure removes the temporary file and leaves the old file. Tags: `WriteCancelled`, `JSONMarshalFailed`, `DirCreateFailed`, `TempFileCreateFailed`, `TempFileWriteFailed`, `TempFileSyncFailed`, `TempFileCloseFailed`, `RenameFailed`, `DirSyncFailed`. The file operations are an unexported `atomicOps` value passed as a parameter, so tests make each step fail (no global state).
- `ReadJSON(path, v)`: tags `JSONNotFound` (also `errors.Is(err, fs.ErrNotExist)`, so callers start with an empty value without quarantining), `JSONReadFailed`, `JSONInvalid`. `QuarantineBroken(path, now)` renames to `<name>.broken-<20060102T150405Z>` (UTC), appending `-2`, `-3` when the name is taken, and returns the new path (`QuarantineFailed`). The caller logs and shows the error (SPEC §4); the `"format": 1` field belongs to the document types of later steps.
- `Lock(paths)`: creates the state folder and the lock file, takes a non-blocking exclusive `syscall.Flock`, returns an idempotent unlock (closes the descriptor). A held lock, in this process or another, gives `AlreadyRunning`; `LockOpenFailed` and `LockFailed` are the other failures. The caller maps `AlreadyRunning` to the message and exit code 2 of SPEC §4 (not done here).
- `internal/core/storage/storagetest`: `Isolate(t testing.TB) storage.Paths` sets `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME` and `XDG_CACHE_HOME` (the four variables `scripts/isolated.sh` sets) to separate `t.TempDir()` folders with `t.Setenv` and returns `storage.Resolve()`. It cannot be used by parallel tests.
- `internal/core/logging`: `New(paths, debug) (*slog.Logger, io.Closer, error)` gives a JSON `slog` handler (Info, Debug when `debug`) on a `RotatingWriter` (`NewRotatingWriter(path, maxSize, files)`: appends, rotates before a write that would pass `maxSize` to `path.1` ... `path.<files-1>`, drops the oldest; 10 MiB and 3 files, the current one included, in `New`; a single write larger than the limit is not split; a mutex makes it safe for concurrent writes; a failed rotation or reopen is retried by the next write). Tags: `LogOpenFailed`, `LogWriteFailed`, `LogRotateFailed`, `LogCloseFailed`, `LogClosed`. Reading `RADIO_TUTOR_DEBUG` is left to the caller.
- `scripts/isolated.sh` from step 0.1 already matches SPEC §16; checked (the variables are set, the exit code passes through, `--keep` keeps the folder); unchanged.

Tests:
- `storage`: table-driven paths (defaults, variables, mixed, empty/relative ignored, `HomeUnknown`, the real environment, file names); atomic write round trip with modes, failure injection at write, sync, close, rename and folder sync (old file intact, no temporary file left), cancelled context, unmarshalable value, parent that is a file, read-only folder, target that is a folder; `ReadJSON` cases; quarantine (names, collisions, errors); lock (second holder refused with a second `Lock` call in the same process, release, `LockOpenFailed`, `LockFailed`, `DirCreateFailed`).
- `storagetest`: the variables point under the temp folder, are distinct, and match the returned `Paths`; each test gets fresh folders.
- `logging`: levels, JSON lines, `*errors.Error` logged with its Tag, append across runs, rotation (1, 2, 3 files kept), oversized write, 16 concurrent writers and 8 concurrent loggers under `-race`, write after close, open, rotate, close and write failures with route tests.

Deviations from the spec: none. `Isolate` takes a `testing.TB`. `ReadJSON` takes no `ctx`, as the step lists it.

Manual checks: none.

Later:
- `Key.T`, `Caption.Start` and `Caption.End` are plain `float64` (absent reads as 0, as `encoding/json` cannot tell); if a missing `t` or `start` should be an error, the cassette package can check for the key during decoding (for example with a second pass into `map[string]json.RawMessage`).
- Validate does not warn about a key that sets no property (allowed by the spec).
- The `reset-password` command and the app start (step 6.1) map `AlreadyRunning` to "Radio Tutor is already running" and exit code 2.
- `Paths` may gain more helpers (per cassette, per student) when the steps that need them are built.

## 1.4 Ogg Vorbis decoding
Date: 2026-10-30.

Built:
- Dependency `github.com/jfreymuth/oggvorbis@v1.0.5` (with `github.com/jfreymuth/vorbis@v1.0.2`); both are pure Go (no `import "C"`, checked in the module cache).
- `internal/core/audio`: `Clip{SampleRate, Channels int; Samples []float32}` (interleaved, -1..1) with `Frames()`, `Seconds()`, `Duration()`; `Info{SampleRate, Channels int; Frames, Bytes int64}` with `Seconds()`, `Duration()`; `Decode(ctx, r) (*Clip, error)`; `DecodeLimit(ctx, r, maxSeconds)` (Decode that fails with `AudioTooLong` as soon as the audio is longer than the limit, so an unprobed file cannot fill memory); `Probe(ctx, r) (Info, error)`; `CheckNarration(info, maxSeconds) []problem.Problem` with the constants `MaxNarrationSeconds` (600), `MaxPromptSeconds` (60) and `MaxBytes` (20 MiB). Problems have an empty path: the caller applies `problem.InFile` (for example `lessons/l1/narration.ogg`), and passes `MaxPromptSeconds` for `prompt.ogg`.
- Design: one internal `stream` function decodes block by block (4096 frames) and hands each block to a sink; `Decode` appends the blocks, `Probe` drops them (constant memory). The length is counted while decoding, not taken from the container: the library's `Length()` needs a seekable reader and trusts the last page's granule position, while the count is what the player will really play (and works for any `io.Reader`; the reader is hidden behind a struct so the library never seeks). `Probe` also reads the bytes after the last page so `Info.Bytes` is the file size (CheckNarration's 20 MiB rule). The context is checked before every block. The library panics on some hostile input (an Ogg page with no segments, found by hand); `stream` recovers and returns `AudioDecoderPanic`.
- Error tags: `AudioHeaderInvalid` (not Ogg Vorbis, bad or truncated headers), `AudioFormatInvalid` (no sample rate or channels), `AudioDataCorrupt` (bad checksum, truncated data; the original error is kept for `errors.Is`), `AudioDecoderPanic`, `AudioTooLong`, `AudioCanceled` (keeps the context error: `errors.Is(err, context.Canceled)`), `AudioReadFailed` (reading after the last page).
- `CheckNarration` rules: rate 22050/44100/48000, 1 or 2 channels, not empty, at most `maxSeconds`, at most 20 MiB when `Bytes` is known. Every problem is reported.
- Test data in `internal/core/audio/testdata/` (4 to 8 KB each): 1 s tones mono 44100, stereo 48000, mono 22050, mono 16000 (unsupported), a corrupted copy and a truncated copy of the first; `README.md` has the exact ffmpeg commands.

Tests: durations, channels, rates and exact frame counts of every file through `Probe` and `Decode` (and that the two agree); sample content (440 Hz zero crossings, peak in range); a one-byte-at-a-time and half-reader (non-seekable); trailing bytes counted; every error tag with its route (empty input, not Ogg, truncated headers, corrupt page, truncated data, page with no segments, zero rate, zero channels); `DecodeLimit`; cancellation (already cancelled, in the middle, deadline, while reading the trailing bytes); `CheckNarration` for every rule and for the probed files with `InFile`.

Deviations from the spec: none. `DecodeLimit` and the `Bytes` field of `Info` are additions to the plan.

Manual checks: none (no sound is played in this step).

Later:
- Step 5.1 adds the player and the sinks to this package; they take a `*Clip`.
- A truncated file whose cut falls exactly on a page boundary decodes without error (the library cannot tell it from a finished stream without the end-of-stream flag); it is just shorter, and the narration length is the lesson length, so nothing else breaks.
## 2.1 Profiles
Date: 2026-10-30.

Built:
- `internal/core/profile`: `Store` over `profiles.json` (`{"format": 1, "profiles": [{username, display_name, role, password_hash, created_at}]}`), `Open(ctx, path, opts...)` (missing file = empty store; `format` other than 1 fails with `ProfilesFormatUnsupported`; an invalid or repeated username, an invalid role or display name, or a damaged hash fail at open; invalid JSON is the storage tag `JSONInvalid`, and the file is not moved: the app start decides on `storage.QuarantineBroken`, `reset-password` just fails). Options `WithClock(func() time.Time)` and `WithIterations(n)`. Methods: `Create(ctx, username, displayName, role, password) (Profile, error)`, `List`, `Get`, `Rename`, `SetPassword`, `Delete` (the last admin is refused with `LastAdmin`), `Login(username, password) (Profile, error)`, `NeedsFirstAdmin()`. `Profile` has no hash. `Role` (`RoleAdmin`, `RoleStudent`, `Valid`, `MinPasswordLength`: 8 and 4). Usernames are lower-cased on every entry point and must match `^[a-z0-9_]{2,32}$`; display names are trimmed and 1 to 40 characters; password lengths count characters (runes).
- Concurrency and persistence: a mutex guards the store; every change builds the new record list, writes it with `storage.WriteJSONAtomic`, and only then makes it current, so a failed write changes nothing in memory. Hashing (about 0.3 s) is done outside the mutex.
- Hash (`password.go`): `pbkdf2-sha256$<iterations>$<salt b64>$<key b64>` (standard base64 with padding), 16-byte salt from `crypto/rand`, 32-byte key from `crypto/pbkdf2` with SHA-256, 600,000 iterations by default (`DefaultIterations`). Verification takes the iteration count, salt and key length from the stored hash and compares with `crypto/subtle`. A stored hash is range-checked (iterations 1..10,000,000, salt 8..64 bytes, key 1..64 bytes) before use.
- Login throttle: in memory, per lower-cased username (known or not), after 5 failed logins that username is refused with `LoginThrottled` (metadata `retry_after`) for 30 seconds by the injectable clock, without looking at the password; the wait ends by itself, a success or deleting the profile clears the count, a new wait needs 5 new failures. A wrong password and an unknown username are both `LoginFailed` (the unknown one still pays one hash derivation, so timing does not tell them apart).
- `cmd/radio-tutor reset-password <username>` (`resetpassword.go`): resolves the folders, takes `storage.Lock` (`AlreadyRunning` prints "Radio Tutor is already running", exit 2), opens the store, checks the user exists (else `no user named "x"`, exit 1), reads the password twice and calls `SetPassword` (the role's minimum length is enforced there and the message names it). When stdin is a terminal (`terminalEcho`: a `*os.File` that answers `TCGETS`), the prompts go to stderr and the echo is turned off with `unix.IoctlGetTermios/IoctlSetTermios` (`TCGETS`/`TCSETS`, clearing `ECHO` only) and restored by a `defer` and when the context is cancelled (SIGINT/SIGTERM come through `signal.NotifyContext`; the blocked read is abandoned). Otherwise one line per password, `\n` or `\r\n`, a last line without a newline is accepted, no prompts. The terminal sits behind the small `echoControl` interface and `termiosOps`. Usage errors exit 2.
- `run` now takes the stdin `io.Reader` after `args` (`main` passes `os.Stdin`).
- `golang.org/x/sys` became a direct dependency (already allowed; it was indirect through Glutter).

Tests:
- `profile`: hash format (parts, sizes, key equals PBKDF2 of the password, salts differ, default 600,000), verification using the stored iteration count, every damaged-hash case with its tag, random-source and derivation failures; every `Create` rule (username pattern and length limits, case, display name limits and trimming, role, password minimums per role counted in characters, duplicate case-insensitively); `List`/`Get`/`NeedsFirstAdmin`/creation times; `Rename`, `SetPassword`, `Delete` with the last-admin rule and persistence; login (case, wrong, unknown, empty); the throttle (4 failures, the 5th starts the wait, right password refused during it, 29 s still waiting, other users unaffected, expiry at 30 s, success resets, 5 more failures for a new wait, unknown names, in memory only, delete clears it); `Open` on every bad file; the file content and mode; a failed write (read-only folder) leaves memory unchanged for all four mutations; 8 goroutines under `-race`.
- `cmd/radio-tutor`: `reset-password` with stdin not a terminal (success for both roles, CRLF, no final newline, upper-case name, old password stops working, too short, differ, empty input, one line, unknown user, usage errors, lock held by the app, lock released, storage failures); with a fake terminal (echo off exactly once and restored once, prompts, restore after each kind of failure, no terminal use for an unknown user, interrupt through context cancel); route tests for every new tag; `termiosEcho` against a real pseudo-terminal (skipped when `/dev/ptmx` is unavailable) and against faked ioctls.

Deviations from the spec: none. Readings: `NeedsFirstAdmin` is true when there is no admin (SPEC says "no profile exists"; the two are the same unless the file was edited by hand, and without an admin nobody could manage the app); the base64 of the hash is the standard alphabet with padding; `Open` does not quarantine a broken file itself.

Manual checks: none needed for tests; a person may want to try `radio-tutor reset-password <user>` on a real terminal (typing hidden, Ctrl-C restores the echo): `scripts/isolated.sh -- go run ./cmd/radio-tutor reset-password nobody` shows the unknown-user path without touching real data.

Later:
- The app start (step 6.1) calls `profile.Open` and, on `JSONInvalid`, `storage.QuarantineBroken` plus the log and the admin message of SPEC §4.
- Deleting a student's progress and photos (SPEC §3.1) belongs to the code that calls `Store.Delete`.
- The throttle map keeps an entry per username that failed; it is small and per run.

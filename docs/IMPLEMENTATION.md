# Implementation plan

The steps that build Radio Tutor version 1 from `docs/SPEC.md` and `docs/CASSETTE-FORMAT.md`. Finished steps are logged in `docs/IMPLEMENTED.md`.

## Rules

1. Steps are implemented by agents (Sonnet 5.5, effort high) managed by the main session (CLAUDE.md "Working through the steps"). A step starts only when the steps in its **Depends on** are merged into `main`. Independent steps run in parallel, each in its own worktree.
2. Each step has its own branch `step/<id>-<slug>` made from the current `main` (`git worktree add <scratch dir>/<id> -b step/<id>-<slug> main`).
3. Before writing code, read the SPEC and CASSETTE-FORMAT sections the step lists, CLAUDE.md, and `AI_GUIDE.md` of `github.com/rm4n0s/errors`. For UI steps, read the Glutter sources and examples the step needs in the module cache.
4. A step delivers its code, its tests (table-driven; every error Tag with a route test), and doc comments on every exported identifier. Do only what the step says; note ideas for later in the `IMPLEMENTED.md` entry under "Later".
5. The Definition-of-done checks of CLAUDE.md pass on the branch: `gofmt -l cmd internal` prints nothing, `go vet ./...`, `scripts/heavy.sh -- golangci-lint run`, `scripts/heavy.sh -- env GOMEMLIMIT=3GiB go test -race -p 2 ./...`, `CGO_ENABLED=0 go build ./...`, `CGO_ENABLED=0 go test ./...`, `scripts/error-tags.sh --untested` (from step 0.1 on).
6. The step adds its entry to `docs/IMPLEMENTED.md` on its branch: `## <id> <title>`, the date, what was built (packages, main types), the tests, deviations from the spec (with the spec change made on the branch), manual checks (if any), "Later".
7. The agent commits on its branch (imperative messages, no attribution lines) and reports to the manager: the branch, the commits, the check results, the spec changes, new `docs/GLUTTER-ISSUES.md` entries. The agent does not merge.
8. The manager reviews the diff, runs the checks again, rebases on `main` if needed, fast-forward merges into the local `main`, tags `step-<id>`, and removes the worktree.
9. **Stop and ask the user** when: a step is ambiguous or the spec is wrong in a way that changes behavior the user would notice; a tool is missing; a dependency not allowed by CLAUDE.md is needed; `sudo` is needed; a check keeps failing after three honest attempts at a fix. Agents report these to the manager, who asks the user.
10. Spec changes needed by a step (small corrections, details the spec left open) are made on the step's branch and listed in its entry.
11. Golden images are looked at before they are committed (the agent reads the PNG).
12. Glutter workarounds follow CLAUDE.md ("Glutter").
13. Checks that need a person (real sound, a real phone, a real AI provider) go to `docs/MANUAL-CHECKS.md` with the step id and exact commands; they do not stop the work.
14. When the last step of a milestone is merged, the manager pushes: `git push origin main` and `git push origin --tags`. If a push fails twice, stop and ask the user.

## M0 Foundation

### 0.1 Scaffold
Depends on: nothing. Spec: SPEC §5, §14, §15, §16; CLAUDE.md.
- `go.mod` (module `github.com/rm4n0s/radio-tutor`, the installed Go version), with `github.com/rm4n0s/glutter@v1.9.5` and `github.com/rm4n0s/errors@v0.2.0`.
- `cmd/radio-tutor/main.go`: subcommand dispatch (`validate`, `pack`, `reset-password`, `version`; the first three print "not implemented yet" and exit 2 for now); with no arguments, opens a Glutter window titled "Radio Tutor" (800×600 minimum) showing "Radio Tutor" centered. `version` prints the module version from build info.
- `internal/archtest`: tests for the rules of SPEC §5 (core never imports Glutter or gui; no core package other than `grading/openai`, `providers` and `phone` imports `net/http` in non-test files; no `os/exec` outside the audio test files).
- `.golangci.yml` (golangci-lint v2: the default linters plus `errcheck`, `govet`, `staticcheck`, `unused`, `ineffassign`, `misspell`, `gocritic`; `forbidigo` forbidding `fmt.Errorf`; `depguard` forbidding the standard library `errors` package).
- `scripts/heavy.sh`: `heavy.sh -- <command>` takes one of two machine-wide slots (`flock` on `/tmp/radio-tutor-heavy.{1,2}.lock`, waiting for a free one) and runs the command. `scripts/error-tags.sh [--untested]`: lists every Tag given to `errors.New`, `errors.NewErr`, `errors.NewErrf` in non-test Go files; with `--untested`, prints the Tags that appear in no `_test.go` file and are not listed in `docs/ERROR-TAGS-UNTESTED.md`. `scripts/isolated.sh` (SPEC §16).
- `.gitignore`: add `/radio-tutor` (the binary) and `*.cassette` outside `testdata/`.
- `docs/IMPLEMENTED.md` created with this step's entry.
- Tests: archtest; a test of the subcommand dispatch (usage errors exit 2); a `gluttertest` test that the root widget shows "Radio Tutor".

### 0.2 Storage and logging
Depends on: 0.1. Spec: SPEC §4, §15, §16.
- `internal/core/storage`: `Paths` (config, data, state folders from the XDG variables with their defaults; `Resolve() (Paths, error)`), `WriteJSONAtomic(ctx, path, v)` and `ReadJSON(path, v)` (with the `.broken-<time>` rename of SPEC §4 as a separate `QuarantineBroken`), `Lock(paths) (unlock func(), error)` with `flock` through `syscall` (tag `AlreadyRunning`), folder and file modes.
- `internal/core/storage/storagetest`: `Isolate(t) storage.Paths`.
- `internal/core/logging`: `New(paths, debug bool) (*slog.Logger, io.Closer, error)` with a rotating file writer (10 MiB, 3 files).
- `scripts/isolated.sh` used in the docs of CLAUDE.md works.
- Tests: paths from variables and defaults; atomic write survives a failure before rename (no partial file); broken JSON quarantine; the lock refuses a second holder (in a subprocess-free way: two `Lock` calls); rotation.

## M1 Cassettes

### 1.1 Timeline model and evaluation
Depends on: 0.1. Spec: CASSETTE-FORMAT §5.3, §5.4, §5.5; SPEC §8.
- `internal/core/timeline`: types `Track`, `Key` (pointer or `Opt`-style fields for the optional properties), `Caption`, `Ease`, `RevealFrom`; `ActorState{Opacity, X, Y, Scale, Rotate, Reveal float64}`; `Evaluate(tracks, t) map[string]ActorState` (or a slice in track order) and `CaptionAt(captions, t) (Caption, bool)`; the easing functions (cubic in, out, in-out; step).
- JSON decoding with unknown-field rejection is done by the cassette package; this package has the types with JSON tags and a `Validate(tracks, captions, length float64) []Problem` for the rules that need only the timeline (increasing keys, ranges, origin and reveal_box needed, captions ordered and non-overlapping, times within the length + 0.05).
- `Problem{Path, Message string, Warning bool}` lives in a small package `internal/core/problem` used by every validator.
- Tests: table-driven evaluation (before the first key, between keys with each ease, after the last, properties set by different keys, step), captions at boundaries, every validation rule.

### 1.2 Cassette documents
Depends on: 1.1. Spec: CASSETTE-FORMAT §2, §3, §5.3, §7, §8, §10 (2-4); SPEC §6.
- `internal/core/cassette`: `Manifest`, `Item`, `Lesson` (title, summary, tracks, captions), `Exercise` (with the label default applied by the cassette), `Cassette` (manifest plus the parsed lessons and exercises, by id; worksheets computed: `Worksheets() []Worksheet`, `WorksheetOf(exerciseID)`).
- Parsing from an `fs.FS` (so the same code reads a folder, a zip and the store): `Load(ctx, fsys) (*Cassette, []problem.Problem, error)`; JSON with `DisallowUnknownFields`, UTF-8 and size checks, `format` 1 (other values: "This cassette needs a newer Radio Tutor"), every field rule, ids, versions (a `Version` type with comparison), labels unique after defaults, folders vs items, the first-item warning. Collects all problems.
- Tests: a valid minimal cassette in `testdata`; one test per rule with a broken copy (built in the test from the valid one).

### 1.3 Scene layers
Depends on: 1.2. Spec: CASSETTE-FORMAT §5.1, §5.2, §10 (5, 6); SPEC §8.
- `internal/core/scene`: `Split(data []byte) (*Scene, []problem.Problem, error)`: parse with `encoding/xml` token by token; the root's attributes; the `viewBox` (aspect ratio rule for scenes); the shared `<defs>` and `<style>`; the top-level children grouped into `Layer{ActorID string (empty for static), SVG []byte}`, each layer a complete SVG document with the root's attributes, the shared defs and styles, and its element(s); consecutive static elements merged into one layer. Forbidden elements and attributes, external references, actor id rules, at most 64 actors, size limit. `CheckPicture(data)` for `prompt.svg` and `cover.svg` (same forbidden-content rules, a viewBox, no actor rules; cover aspect 3:4 is a warning).
- Cross-check with the lesson's tracks: `CheckTracks(scene, tracks) []problem.Problem` (unknown actors, `reveal_box` inside the view box).
- Tests: splitting keeps order and defs; nested groups stay inside their actor; every forbidden construct; ids; the cross-check.

### 1.4 Ogg Vorbis decoding
Depends on: 0.1. Spec: CASSETTE-FORMAT §6; SPEC §8.1, D7.
- `go get github.com/jfreymuth/oggvorbis@v1.0.5`. `internal/core/audio`: `Decode(ctx, r io.Reader) (*Clip, error)` (`Clip{SampleRate, Channels int; Samples []float32 interleaved}`, `Duration()`), `Probe(ctx, r) (Info, error)` (rate, channels, duration without keeping samples), the checks of CASSETTE-FORMAT §6 as `CheckNarration(info, maxSeconds) []problem.Problem`.
- Test data: tiny Ogg files made once with `ffmpeg` (a 1 s 440 Hz tone, mono and stereo, 44100 and 22050 Hz; an unsupported rate) committed under `testdata/` with the commands in a README there; a corrupted file.
- Tests: durations, channels, rates, corrupt input tags, context cancellation.

### 1.5 Archives and the store
Depends on: 0.2, 1.2, 1.3, 1.4. Spec: CASSETTE-FORMAT §1, §9, §10; SPEC §6.2, §6.3.
- `internal/core/archive`: `OpenZip(ctx, path) (fs.FS, io.Closer, []problem.Problem, error)` with every limit and name rule of §9 (zip-slip, symlinks via the mode bits, ratios, counts, sizes, allowed names, duplicate names, only the files of §1). `Validate(ctx, fsys, render RenderChecker) Report` runs every check of §10 (cassette.Load, scene.Split and CheckTracks for each lesson, pictures, audio probe and the length checks, which need the narration's length passed into the timeline validation); `RenderChecker` is an interface (implemented by `internal/gui/svgcheck` in step 1.6; the core uses a no-op one in its tests).
- `Pack(ctx, dir, out, force)` writes the zip (sorted, deflate).
- `Store` over `Paths`: `List`, `Get(id)`, `Import(ctx, path, confirmUpgrade func(old, new Version) bool) (ImportResult, error)` with the rules of SPEC §6.2 (tmp folder then rename, upgrade replace, same/older version), `Remove(id)`, broken entries listed.
- Tests: hostile zips built in the tests (zip-slip, symlink, bomb ratio, too many entries, bad names, extra files), the valid sample from 1.2 packed and imported, upgrades and refusals, removal, a broken installed manifest.

### 1.6 Validate and pack commands
Depends on: 1.5. Spec: SPEC §14; CASSETTE-FORMAT §10.
- `internal/gui/svgcheck`: `Checker` implementing `archive.RenderChecker` by calling Glutter's `svg.Parse` and recording a picture of each layer and picture.
- `cmd/radio-tutor validate [--json] <path>` and `pack [--force] <dir> <out>` with the outputs and exit codes of SPEC §14.
- Tests: running the command functions (not a subprocess) on the valid and broken test cassettes; output format; exit codes; an SVG that the core accepts but Glutter cannot draw is reported.

## M2 Accounts and progress

### 2.1 Profiles
Depends on: 0.2. Spec: SPEC §3, D11.
- `internal/core/profile`: `Store` over `profiles.json`: `Create`, `List`, `Get`, `Rename`, `SetPassword`, `Delete` (not the last admin), `Login(username, password)` with throttling (an injectable clock), `NeedsFirstAdmin()`; hashing with `crypto/pbkdf2` in the format of SPEC §3.1 (the iteration count read from the stored hash, so tests can use a store option with fewer iterations).
- `cmd/radio-tutor reset-password` (SPEC §14) using `golang.org/x/sys/unix` for echo.
- Tests: every rule (username pattern, password lengths, last admin, case-insensitive login, wrong password, throttle and its expiry), hash format and verification, the command with stdin not a terminal.

### 2.2 Progress
Depends on: 0.2, 1.2. Spec: SPEC §7, D15-D18.
- `internal/core/progress`: the save file types of SPEC §7.4; `Store` (`Load(username, cassette)`, `Save`), the unlock rule as a pure function `States(c *cassette.Cassette, p *Progress) map[string]ItemState`, `CompleteLesson`, `ApplyResults(submissionResults)` (attempts, pass, hints, needs_review per SPEC §7.6), overrides (§7.7), settings, `Continue()` (the item to open), the upgrade merge of §7.5, `ResetAll`.
- Tests: the unlock rule with skip lessons and free navigation over a cassette with several worksheets; attempts and needs_review; hints; results for passed exercises; overrides; the upgrade merge with removed, kept and retyped ids.

## M3 AI grading

### 3.1 Grading core
Depends on: 1.2. Spec: SPEC §10.3, §11 (all but the HTTP details), D9, D10.
- `internal/core/grading`: `NormalizePhoto(ctx, data []byte) ([]byte, error)` (decode, EXIF orientation for JPEG with a small stdlib parser, scale to 2048 with `golang.org/x/image/draw`, JPEG 85; tags for unsupported and undecodable input); `Request` (cassette data for the selected exercises, photos), `BuildPrompt(req) (system, user string)` per §11.3-§11.4 (values JSON-quoted); `ReplySchema` (the JSON Schema as a `map[string]any`); `ParseReply(text, asked []string, lenient bool) (Reply, error)` per §11.5; the `Provider` interface (`Grade(ctx, Request) (Reply, Meta, error)`, `Name()`); `Grader` that calls a provider with the single retry of §11.5; `fake.Provider` with scripted replies.
- Tests: photo orientation (test JPEGs with each EXIF orientation made in the test), scaling, prompt contents (exercise data quoted, injection-like text stays inside the block), parsing (fences, extra text, missing and unknown ids, duplicates, bad verdicts), the retry.

### 3.2 OpenAI-compatible provider
Depends on: 3.1. Spec: SPEC §11.1, §11.2, §11.5, D8.
- `internal/core/grading/openai`: `Provider` from a config (base URL, model, key, timeout, structured_output): builds the Chat Completions request (system, user text and image parts as data URLs), parses `choices[0].message.content` and `finish_reason`, the downgrade retry of §11.2, status code tags (401/403 auth, 404 model or URL, 429 rate limit, 5xx), timeouts through `ctx`.
- Tests with `httptest`: request shape (headers, JSON body, images), each `structured_output`, the downgrade, every error status, `length`, slow server and cancellation.

### 3.3 Provider settings
Depends on: 3.2, 0.2. Spec: SPEC §11.1, §11.6.
- `internal/core/providers`: `Store` over `providers.json` (0600; refuses to read a file readable by others and tells the admin to fix it, tag `ProvidersFileTooOpen`), CRUD, active provider, `Presets()` (the examples of §11.1 with their base URLs and suggested vision models), `Test(ctx, cfg) (TestResult, error)` using an embedded test image (a PNG of "2 + 3 = 5" drawn in handwriting style, generated once by a small program in `internal/core/providers/testimage/gen` and committed).
- Tests: file mode handling, CRUD, the test against an `httptest` server.

## M4 Phone and submissions

### 4.1 Phone server
Depends on: 0.2, 3.1. Spec: SPEC §10.2, §10.3, §17.
- `internal/core/phone`: `Server` (`Start(ctx, port) error`, `Stop`, `Open(page PageData) (token, url string)`, `Close(token)`, events channel or callbacks: page opened, photo received, sent), LAN address selection (`/proc/net/route` parsing, interface choice), the embedded page (HTML template + CSS + small JS: thumbnails, remove, send with progress), limits, rate limit per address, content sniffing, `grading.NormalizePhoto` on receipt, storing through a callback (the server does not know the storage layout).
- Texts through `internal/core/i18n` (create the package here with the keys this step needs, `en` and `el`).
- Tests: the page renders the exercises (and escapes them), upload of 1 and 6 photos, too many, too big, wrong type, expired and unknown tokens (410/404), rate limit, token single use, address selection from sample `/proc/net/route` contents.

### 4.2 QR codes
Depends on: 0.1. Spec: D14, SPEC §10.1.
- `go get rsc.io/qr@v0.2.0`. `internal/core/qrcode`: `Encode(text string, level) (*image.Gray or a module matrix, error)`; the GUI draws the matrix itself (crisp at any size).
- Tests: a URL of the expected length encodes; the matrix has the expected size and finder patterns; a golden matrix for a fixed URL.

### 4.3 Submission service
Depends on: 2.2, 3.3, 4.1. Spec: SPEC §9, §10.1, §11, §7.6.
- `internal/core/submission`: `Service` tying profiles, the store, progress, the phone server, the grader and the active provider: `Open(ctx, username, cassetteID, exerciseIDs) (*Session, error)`, session events (`PageOpened`, `PhotoReceived(n)`, `Grading`, `Graded(results, unlocked hints)`, `Failed(err)`), `Retry`, `Cancel`, photo storage (SPEC §4 paths), save-file updates (submission records, `progress.ApplyResults`), one open submission per student, the 30-minute expiry (injectable clock).
- Tests: the whole flow with the fake provider and a real phone server on 127.0.0.1 (upload with `net/http` client), failure then retry, cancel, expiry, a second open cancels the first, results for unknown exercises ignored.

## M5 Audio playback

### 5.1 Player and sinks
Depends on: 1.4. Spec: SPEC §8, §8.1, §8.2.
- `go get github.com/jfreymuth/pulse@v0.1.3`. `internal/core/audio`: `Sink` interface (start, write, pause, position, close), `Player` (`Load(clip)`, `Play`, `Pause`, `Stop`, `Position() float64`, `State`, `Ended` notification), `FakeSink`, `SilentSink` (wall clock, injectable), `PulseSink` (connects through the PulseAudio protocol; position = played samples minus latency), `OpenDefaultSink(ctx)` that falls back to `SilentSink` and reports why.
- Tests with `FakeSink` and a fake clock: play/pause/stop positions, end detection, play after end restarts, never runs backwards; a `radiotutor_audio` test that plays a short tone through the real server (manual check entry in `docs/MANUAL-CHECKS.md`: "you should hear a 1 s tone").

## M6 Desktop UI

### 6.1 App shell and login
Depends on: 2.1, 0.2. Spec: SPEC §3, §12.1 (frame only), §13 (frame only), §17, D6, D19.
- `internal/gui/app`: startup (paths, lock, logging, stores; `AlreadyRunning` dialog), theme (Material, large type, child-friendly colors), navigation (a route stack), the first-run "Create the parent account" screen, the login screen (display-name buttons, password field, throttle message), log out, placeholder admin and student homes; `internal/core/i18n` extended with these keys; `internal/core/settings` (`settings.json`, SPEC §12.4).
- `internal/gui/testutil`: pumping the app with isolated storage and the test fonts.
- Tests: first run creates the admin; login success and failure; language switch; golden images of first run and login (en, el).

### 6.2 Admin screens
Depends on: 6.1, 1.6, 3.3. Spec: SPEC §12 (without progress), §6.2, §6.3, §11.1, §11.6, §11.7.
- Students (add, rename, reset password, delete, hidden cassettes); Cassettes (import with the file dialog, progress, problems list, upgrade confirmation, remove with the option to delete progress and photos); Settings (language, students can import, phone port and interface); Providers (list, add from presets, edit, set active, Test with the result, the cloud/local note). The admin home shows the count of exercises waiting for review (0 until 6.6).
- Tests: widget tests of each flow with isolated storage and an `httptest` provider; golden images of each screen.

### 6.3 Student screens
Depends on: 6.1, 1.5, 2.2. Spec: SPEC §13, §7.
- Shelf (cards with cover via `svg.SvgPicture`, progress bars, import when allowed), cassette screen (items, worksheets grouped, locks, states, Continue, the locked message). Opening a lesson or worksheet navigates to placeholders filled by 6.4 and 6.5. The admin's **Play as me** uses the same screens.
- Tests: states shown for a progress fixture, Continue target, locked tap, golden images.

### 6.4 Lesson player
Depends on: 6.3, 1.3, 5.1. Spec: SPEC §8, CASSETTE-FORMAT §5.
- `internal/gui/player`: the stage (layers as `svg.SvgPicture` from bytes, `Stack`, fit and letterbox, per-actor `Transform` around the origin, `Opacity`, reveal with a clip), the frame ticker reading the player's position, captions bar, controls and keys, completion (saves through progress), Next and Skip, "No sound device" notice, loading indicator.
- Tests: with `FakeSink` and the fake clock, golden images of the sample-like test lesson at fixed times (before, during and after moves, a half reveal, rotation around an origin), Play/Pause/Stop through taps and Space, completion saved, Skip only with skip lessons.

### 6.5 Worksheet, QR dialog and results
Depends on: 6.3, 4.2, 4.3. Spec: SPEC §9, §10.1, §7.6.
- Worksheet screen (cards with label badges, prompt pictures, prompt audio button, states, attempts left, hints), selection and **Check my answers**, the QR dialog (QR drawn from the matrix, URL text, counters, firewall hint after 60 s, Cancel), grading indicator, results screen, failure with Try again. All through the submission service's events.
- Tests: the full flow in `gluttertest` with the fake provider and an upload made by the test's HTTP client; golden images of the worksheet, QR dialog (fixed URL), results.

### 6.6 Parent progress and review
Depends on: 6.2, 4.3. Spec: SPEC §12.3, §7.7, §11.7.
- Progress screen (student, cassette, items with states and overrides, switches), submissions list and detail (photos with zoom, page notes, per-exercise results, override buttons, delete photos), needs-review first and the count on the admin home, delete a student's photos.
- Tests: each override changes the save file and the states; deleting photos; golden images.

## M7 Content and release

### 7.1 Sample cassette
Depends on: 1.6. Spec: CASSETTE-FORMAT (all); `docs/CASSETTE-AUTHORING-PROMPT.md`.
- `testdata/cassettes/sample/`: a small English cassette (2 lessons of about 30 s, a worksheet of 2 exercises after each), written by following the authoring prompt's method; narration made with `espeak-ng` and `ffmpeg` by `scripts/make-sample-cassette.sh` (which regenerates the audio and the caption times from `testdata/cassettes/sample-src/script.json` and the lesson templates next to it, and packs `testdata/cassettes/sample.cassette`; the sources are not inside `sample/` because a cassette folder may only hold the files of CASSETTE-FORMAT §1). Both the sources and the packed file are committed.
- Tests: the sample validates (through the command function); a test that the script reproduces the committed timings is not needed (espeak output may vary by version).

### 7.2 End-to-end tests
Depends on: 6.4, 6.5, 6.6, 7.1. Spec: SPEC §16.
- The core end-to-end test of SPEC §16 with the sample cassette; a `gluttertest` test that drives first run → add a student → import the sample → log in as the student → play lesson 1 to the end (fake sink) → worksheet → submission (test upload) → results → lesson 2 unlocked → admin sees the submission and its photo.
- Fix what these tests find (in this step, listed in the entry).

### 7.3 User documentation
Depends on: 7.2. Spec: SPEC (all).
- `README.md`: what Radio Tutor is, install (`go install`), first run, the firewall command, choosing an AI provider (cloud vs local, Ollama with a vision model as the offline example), making cassettes (link to the authoring prompt), where data lives, backing up, resetting a password. `docs/MANUAL-CHECKS.md` gains the checks for a real phone and a real provider.

# CLAUDE.md

Guidance for Claude Code when working in this repository (Go, hosted on GitHub).

## Project
Radio Tutor is a desktop tutoring application for children, written in Go. Its user interface is built on [Glutter](https://github.com/rm4n0s/glutter), a cgo-free GUI toolkit for Go with Flutter's framework API. Radio Tutor targets the Linux desktop, Windows 10/11 (amd64 tested under Wine, arm64 built and vetted only; SPEC D20) and macOS 12+ (Apple silicon and Intel, tested on a Mac: the system's part is the `_darwin.go` files, and the sound is Glutter's audio, `internal/gui/app/audio_glutter.go`). Content comes in **cassettes**: zip files with lessons (an SVG scene animated by a timeline and narrated by an audio track, with captions) and exercises. The child solves exercises on paper, takes photos of the page with a phone (the app serves a page on the local network and shows its address as a QR code), and a vision-capable AI model, cloud or offline, grades each solution by the instructions the cassette gives. Parents (admins) manage the students, see the progress and review every photo.

**`docs/SPEC.md` is the source of truth**, together with `docs/CASSETTE-FORMAT.md`, which defines the cassette format and is part of the spec. Read the sections that cover a task before starting it. If the code and the spec disagree, or a task needs a decision the spec doesn't make, ask instead of guessing. When a decision changes, update the spec on the same branch.

## Commands
- Build: `go build ./...`. Radio Tutor is cgo-free; `CGO_ENABLED=0 go build ./...` must always work
- Test all: `GOMEMLIMIT=3GiB go test -race -p 2 -tags debug_mode ./...` (the race detector needs cgo, so this one needs a C compiler). `-tags debug_mode` makes a debug build of Glutter: since Glutter 1.17 a build without tags is a release build, and the GUI tests need Glutter's debug-only platform override (`internal/gui/testutil.UseLinux` skips them without it)
- Test the cgo-free path: `CGO_ENABLED=0 go test ./...`
- Windows build and vet: `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...`, `GOOS=windows go vet ./...` (and the same with `GOARCH=arm64`), `GOOS=windows golangci-lint run`
- Test on Windows, under Wine: `scripts/heavy.sh -- scripts/test-windows.sh` (every package; one package: `scripts/test-windows.sh ./internal/core/storage/...`). Wine runs with a private prefix in `.wine-test/`, with audio drivers disabled and Mesa software rendering only (never the GPU, never a sound): see SPEC §16
- Build for Windows: `scripts/build-windows.sh [-o dir] [-arch amd64,arm64] [-release]` makes `dist/windows/radio-tutor.exe` (GUI) and `radio-tutor-cli.exe` (console, for the subcommands) with the icon and manifest. Smoke test of the real app under Wine (builds both, first run in English and Greek with Glutter's self-capture, the CLI's `validate` and `pack`): `scripts/heavy.sh -- go test -tags radiotutor_wine -p 1 ./cmd/radio-tutor`. Redraw the icon with `go run ./scripts/icongen`
- Test one package: `go test -tags debug_mode ./internal/core/timeline -run TestName -v`
- Tests that need a real display or a real audio server: `go test -tags radiotutor_display ./...`, `go test -tags radiotutor_audio ./...`
- Update golden images: `go test -tags debug_mode <package> -run Golden -update-goldens` (per package: each package with goldens defines the flag, so `./...` would fail in the others), then review every changed golden
- Heavy commands (full test runs, lint, release builds): through `scripts/heavy.sh -- <command>`, see "Memory (OOM)"
- Vet: `go vet ./...`
- Lint: `golangci-lint run`
- Format: `gofmt -w cmd internal`
- Run the app: `go run ./cmd/radio-tutor` (it uses your real data folders)
- Add the app to the application menu (Windows: Start menu): `radio-tutor -setup-desktop [-remove]`, from an installed or built program, never `go run`; only ever inside `scripts/isolated.sh` or a test (it writes under `$XDG_DATA_HOME`)
- Run the app isolated from your data: `scripts/isolated.sh -- go run ./cmd/radio-tutor`
- Validate a cassette: `go run ./cmd/radio-tutor validate <file.cassette|dir>`; pack one: `go run ./cmd/radio-tutor pack <dir> <out.cassette>`
- Rebuild the sample cassette: `scripts/make-sample-cassette.sh`
- After changing dependencies: `go mod tidy`

## Definition of done
Before reporting a task as finished, run `gofmt` (on `cmd` and `internal`), `go vet ./...`, `golangci-lint run` and `go test -race -tags debug_mode ./...`, and fix anything they report. Say so explicitly if any step was skipped or failed. Also:
- `CGO_ENABLED=0 go build ./...` and `CGO_ENABLED=0 go test ./...` pass (proves the app stays cgo-free)
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...`, `GOOS=windows go vet ./...`, the same build and vet with `GOARCH=arm64`, `GOOS=windows golangci-lint run`, and `scripts/heavy.sh -- scripts/test-windows.sh` pass (the app stays Windows-clean; a test skipped on Windows says why in `t.Skip`), and so does the Wine smoke test `scripts/heavy.sh -- go test -tags radiotutor_wine -p 1 ./cmd/radio-tutor` when the change touches the app's start, the window, the data folders, the build script or the CLI
- `scripts/error-tags.sh --untested` prints nothing (every error Tag has a test that asserts its route; `docs/ERROR-TAGS-UNTESTED.md` is the only exception list)
- Every new exported identifier has a doc comment
- Every changed golden image has been looked at, not just regenerated

## Layout
- `cmd/radio-tutor/` is the app's `main` package (the desktop app and the CLI subcommands)
- OS-specific code lives in `_unix.go` (constraint `!windows`; a file named `_linux.go` would be left out on macOS), `_windows.go` and `_darwin.go` files with the same function names (a `_unix.go` file that macOS replaces says `!windows && !darwin`); shared logic stays in plain files tested on both (SPEC §5)
- `internal/core/...` is the core: storage, cassettes, timelines, scenes, audio, profiles, progress, grading, the phone server and the submission service. It never imports Glutter or `internal/gui`, so it is tested without a UI
- `internal/gui/...` is the UI on Glutter: screens, the lesson player, dialogs. It imports the core, never the other way round
- `internal/archtest/` checks the package boundaries and forbidden imports (SPEC §5)
- The UI uses Glutter's `widgets` and `material` packages (SPEC D6)
- `testdata/cassettes/` holds the sample cassette's sources; `scripts/make-sample-cassette.sh` builds it
- `docs/SPEC.md` is the specification; `docs/CASSETTE-FORMAT.md` is the cassette format; `docs/CASSETTE-AUTHORING-PROMPT.md` is the prompt template parents give to an AI agent to make cassettes; `docs/IMPLEMENTATION.md` is the step-by-step plan; `docs/IMPLEMENTED.md` logs finished steps
- `scripts/` holds helper scripts
- Tests live next to the code (`foo_test.go`); golden images live in `testdata/goldens/` of their package
- The exact package list is defined in `docs/SPEC.md`

## Go conventions
- Never ignore a returned error; handle it or return it
- Pass `context.Context` as the first parameter of anything that does I/O (files, processes, network, AI requests, audio). Name it `ctx`. Name a Glutter `BuildContext` `bc`
- Tests are table-driven
- No global mutable state and no singletons. Package-level read-only values are fine and are never reassigned
- Prefer the standard library. Allowed dependencies: `github.com/rm4n0s/glutter` (v1.17.0 or newer), `github.com/rm4n0s/errors`, what Glutter brings in (`golang.org/x/text`, `golang.org/x/image`, `golang.org/x/sys`, `github.com/go-text/typesetting`, `github.com/godbus/dbus/v5`), `github.com/jfreymuth/oggvorbis` (Ogg Vorbis decoding, with the `github.com/jfreymuth/vorbis` it brings in), and `rsc.io/qr` (QR codes). Audio output through PulseAudio or PipeWire's PulseAudio server uses `internal/core/pulse`, a copy of `github.com/jfreymuth/pulse` v0.1.3 (MIT, license in `internal/core/pulse/LICENSE`) that is project code: it follows these rules, except that it keeps upstream's API without `context.Context` (SPEC §5), and a change to it keeps it comparable with upstream. A step that first imports one of them adds it with `go get <module>@<version>`. Ask before adding any other
- AI providers are reached with `net/http` and `encoding/json`; no AI SDKs (SPEC D8)
- **No cgo**: never write `import "C"`. If a C library is ever needed, ask first
- Only Glutter's UI thread touches widgets and render objects. Other goroutines (audio, the phone server, grading) hand work over through `SetState` or the binding's `Post`
- Long-running work (grading, imports, audio decoding) never blocks the UI thread, and takes a `ctx` so it can be cancelled

## Glutter
- Follow Glutter's API conventions: `NewX(required..., opts ...XOpts) *X` constructors, options structs, `foundation.Opt[T]` for nullable options. Radio Tutor's own widgets follow the same conventions
- When Glutter is missing something or has a bug, work around it in Radio Tutor without stopping, and mark the workaround with a `// Glutter:` comment that says what it works around and what Glutter should provide instead
- Keep a list of these workarounds in `docs/GLUTTER-ISSUES.md` (what is missing or broken, where the workaround is, and what the fix in Glutter would be), and mention new entries in the report at the end of the task. Create the file with the first entry; remove an entry when the fix is in the Glutter version Radio Tutor uses, and delete the file when no entry is left
- Read Glutter's source in the Go module cache when its behavior is unclear; its `docs/SPEC.md` describes its design (§26 for SVG)

## Error handling
Use `github.com/rm4n0s/errors` for all errors. Never use `fmt.Errorf`, and never import the standard library `errors` package: this package re-exports `errors.Is`, `errors.As` and `errors.Unwrap`, so a single import covers everything. Read `AI_GUIDE.md` of github.com/rm4n0s/errors (in the Go module cache: `go list -m -f '{{.Dir}}' github.com/rm4n0s/errors`) before writing error code.

**Creating errors**
- New failure with a fixed message: `errors.New("Tag", "message", "key", value, ...)` (all metadata in this one call, never `New(...).SetMetadata(...)`)
  - Tag: short PascalCase name of the failure, e.g. `CassetteManifestInvalid`
  - Trailing arguments are key/value metadata pairs, e.g. `"path", path`
- New failure with a formatted message: `errors.NewErrf("Tag", "format %s", arg)`
- Wrapping an error that is NOT already an `*errors.Error` (stdlib, third-party, Glutter errors that are not from this package): `errors.NewErr("Tag", err)`. Add metadata with one `.SetMetadata("key", value, "key2", value2)` call
- Every `New`, `NewErr` and `NewErrf` call captures the stack at that point
- Construct the error in the function that detects the failure, never in a helper that only builds it: the route is the stack at the constructor. The tag is a string literal at the call
- Sentinels are struct literals, `&errors.Error{Tag: "X"}`, never a package-level `errors.New`

**Propagating errors**
- If the error already came from this package, return it untouched: `return err`. Re-wrapping it would replace the Tag and the stack and break route tests
- Functions return the plain `error` interface, never `*errors.Error`
- `errors.FromError(err)` is a plain type assertion. It fails if something else wrapped the error, so don't wrap these errors with anything

**Inspecting errors**
- `appErr, ok := errors.FromError(err)`, then use `appErr.Tag`, `appErr.Metadata`, `appErr.Message`
- Use `errors.Is` / `errors.As` / `errors.Unwrap` for the underlying cause stored in `OriginalErr`

**Testing errors**
- Every new failure case gets its own unique Tag and a test that asserts the route, not just that an error occurred:
  ```go
  appErr, ok := errors.FromError(err)
  if !ok {
      t.Fatalf("expected *errors.Error, got %T", err)
  }
  if !appErr.HasRoute("Store.Import->readArchive.CassetteEntryUnsafe") {
      t.Errorf("unexpected failure path: %s", appErr.Route())
  }
  ```
- A route lists the call chain from outermost to innermost (`Type.Method` or `Func`, joined by `->`) and ends with the Tag. `HasRoute` matches by substring
- Log errors with `slog`, passing the error as an attribute (`l.Error("failed", "err", err)`): `*errors.Error` is a `slog.LogValuer`. Do not call `ToJson` or `json.Marshal` on an error yourself

## Git and GitHub
- Never commit or push directly to `main`; work on a branch named `feat/...` or `fix/...`
- **No pull requests** (the user, 2026-10-30: "no pull requests only merges"). A finished branch (all Definition-of-done checks passing on it, after rebasing it onto `main`) is fast-forward merged into the local `main`, `main` is pushed, and the branch is deleted. Feature branches are not pushed to GitHub
- **Exception:** if `docs/IMPLEMENTATION.md` exists, follow its Rules section for the steps it lists. Each step is done on its own branch. A finished step (all Definition-of-done checks passing, its entry added to `docs/IMPLEMENTED.md` on that branch) is fast-forward merged into the local `main`, without a PR, and tagged `step-<id>` locally. **GitHub gets milestones, not steps:** `main` and the tags are pushed once, when the last step of a milestone (a `## M<n>` section of `docs/IMPLEMENTATION.md`) is merged (rule 14). Never commit work for a step directly on `main`. Everything not covered by that file still follows the rule above
- Commit messages: imperative mood, e.g. "Add retry to client"
- Keep each branch small; a commit that fixes a GitHub issue links it (`Closes #123`)
- No `.github/` directory and no GitHub Actions workflows: never add them. The Definition-of-done checks are run locally

## Working through the steps
- **Never stop until every step in `docs/IMPLEMENTATION.md` is implemented** (through the last one in the file). A finished step is never the end of a turn: the turn's last action is a tool call that works on the next step, not a summary. Report a finished step (and a published milestone) in one or two lines in the middle of the work, and keep working
- **Work in parallel** (the user, 2026-10-30: "use only sonnet 5.5 high for implementing tasks and you be their manager"). Implementation is done by agents (model Sonnet 5.5, effort high); the main session is their manager and does not write the steps' code itself. Hand independent work to several agents at once: steps that do not depend on each other, reviews, fixes, doc changes. Each agent works in its own git worktree on its own branch (`git worktree add <scratch dir> -b <branch> main`), never in a tree that another agent uses, and never switches, stashes or resets. Steps that change the same files or build on each other run one after the other. The manager reviews each branch, runs the checks, and merges the branches into `main` one at a time in the plan's order (rebasing a branch onto the new `main` first when needed); each step is still its own branch, merge and tag
- Don't stop to ask for confirmation, and don't end a turn with a question such as "Should I start the next step?", until every step in `docs/IMPLEMENTATION.md` is finished
- The only reasons to stop are the ones `docs/IMPLEMENTATION.md` names: its rule 9, a push of a milestone that keeps failing (rule 14), and a step that says **ask the user**. Then stop, ask, and continue with the same step after the answer
- **Manual checks** don't stop the work: add them to `docs/MANUAL-CHECKS.md` (create it if needed) with the step id and the exact command for the user, write "pending, see docs/MANUAL-CHECKS.md" in the step's `IMPLEMENTED.md` entry, and go on. When the user answers, record the answer in that entry and remove the item from the list (delete the file when it is empty)

## Don't
- Don't edit generated files (`*_gen.go`); regenerate them with `go generate ./...`
- Don't commit secrets, API keys or `.env` files
- Don't let a Glutter type appear in the core's packages
- Don't leave processes running: every app instance, phone server or audio test started for a test or a check is stopped when it is no longer needed (run it under `timeout -k`, in its own process group), and `pgrep -a radio-tutor` shows nothing left before a task is reported done
- Never look for, list, open or read files outside this project folder (no home-directory databases, configs, logs or other projects). If something outside it is needed, ask the user to provide it. Exceptions: dependency source in the Go module cache (including Glutter's), and temporary folders your own tests and scripts create
- Never let a test or check use the user's real Radio Tutor data, config, state or photos: Go tests isolate the `XDG_*` folders (the helper of `internal/core/storage`), and scripts run the app through `scripts/isolated.sh` (SPEC §16)
- Never let a test call a real AI provider or play sound on the user's speakers: tests use the fake provider and the fake audio sink; tests against real ones carry their own build tags and are skipped by default

## Memory (OOM)
A run that uses too much memory is killed by the OS (exit 137) and takes the session with it. Be careful:
- Run the race tests with `GOMEMLIMIT=3GiB go test -race -p 2 -tags debug_mode ./...`. At most two heavy test or build commands run at the same time on this machine, counting those of every agent. `scripts/heavy.sh -- <command>` enforces it: it waits for one of two machine-wide slots. Run every full test run (`./...`), `golangci-lint run` and release build through it
- Never run Go fuzzing (`-fuzz`) without `-parallel=1`, a short `-fuzztime` (30s at most) and `GOMEMLIMIT=2GiB`
- Run the tests of one package while working; run `./...` only for the final checks
- If a command is killed (137), find out why before running it again: it may be a real memory bug that needs a fix and a regression test

## Testing notes
- The core is tested with plain Go tests; no UI needed
- Widget tests use `gluttertest` (`PumpWidget`, `Pump`, `Tap`, finders) with a fake clock and the fake platform; no display needed
- Golden tests use `gluttertest`'s `MatchesGolden`, which renders through Glutter's software renderer, so they are deterministic and need no GPU
- Tests that open real windows carry the `radiotutor_display` build tag; tests that play through a real audio server carry `radiotutor_audio`; both are skipped by the default test run
- AI grading is tested with the fake provider and with `httptest` servers that speak the provider's protocol; tests against a real provider carry the build tag `radiotutor_ai` and read the endpoint from environment variables
- The phone server is tested with `httptest` and a real listener on `127.0.0.1`
- Every error Tag has a route test

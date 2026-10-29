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

# Glutter issues worked around

Missing features and bugs of Glutter (the version in `go.mod`) that Radio Tutor works around. Each entry says what is wrong, where the workaround is, and what Glutter should do. An entry is removed when the fix is in the Glutter version Radio Tutor uses; the file is deleted when no entry is left.

## RunApp cannot be ended from outside the widget tree

- **Where seen:** `internal/gui/app/quit.go` (`QuitOnDone`), used by `RunWith`.
- **Problem:** `RunApp` takes no `context.Context` and does not react to SIGTERM or SIGINT, so a process told to terminate keeps running. The only ways to end the run are `WidgetsBinding.ExitApplication` and `PlatformDispatcher.RequestAppExit`, which need the binding, and that is reachable only with a `BuildContext` inside the tree.
- **Workaround:** `QuitOnDone` wraps the root widget, takes the binding in `InitState`, waits for the command's context on a goroutine started with `State.Go` and posts `ExitApplication(AppExitTypeRequired)` to the UI thread with `binding.Post`.
- **Fix in Glutter:** a `RunAppOpts.Context` that ends the run when it is done (the same clean exit as a closed window).

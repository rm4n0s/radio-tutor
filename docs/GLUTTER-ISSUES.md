# Glutter issues worked around

Missing features and bugs of Glutter (the version in `go.mod`) that Radio Tutor works around. Each entry says what is wrong, where the workaround is, and what Glutter should do. An entry is removed when the fix is in the Glutter version Radio Tutor uses; the file is deleted when no entry is left.

## ButtonStyle.TextStyle hides the label

- **Where seen:** step 6.1, `internal/gui/app/theme.go` (`NewTheme`).
- **Problem:** a button whose style was made with `FilledButtonStyleFrom(ButtonStyleFromOpts{TextStyle: ...})` (also `OutlinedButtonStyleFrom` and `TextButtonStyleFrom`), set on the button (`FilledButtonOpts.Style`) or on the theme (`FilledButtonThemeData.Style`), draws no label: the button background and shape are right, the text is missing. Adding `ForegroundColor` to the same `ButtonStyleFromOpts` does not bring it back. Without `TextStyle` the label is drawn.
- **Workaround:** the theme's `TextTheme.LabelLarge` is set to the larger size and the button styles give no `TextStyle`.
- **Fix in Glutter:** a `ButtonStyle.TextStyle` (a size, a weight) must merge with the label style the button would have had, and the label must keep the foreground color.

## RunApp cannot be ended from outside the widget tree

- **Where seen:** `internal/gui/app/quit.go` (`QuitOnDone`), used by `RunWith`.
- **Problem:** `RunApp` takes no `context.Context` and does not react to SIGTERM or SIGINT, so a process told to terminate keeps running. The only ways to end the run are `WidgetsBinding.ExitApplication` and `PlatformDispatcher.RequestAppExit`, which need the binding, and that is reachable only with a `BuildContext` inside the tree.
- **Workaround:** `QuitOnDone` wraps the root widget, takes the binding in `InitState`, waits for the command's context on a goroutine started with `State.Go` and posts `ExitApplication(AppExitTypeRequired)` to the UI thread with `binding.Post`.
- **Fix in Glutter:** a `RunAppOpts.Context` that ends the run when it is done (the same clean exit as a closed window).

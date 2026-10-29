# Glutter issues worked around

Missing features and bugs of Glutter (the version in `go.mod`) that Radio Tutor works around. Each entry says what is wrong, where the workaround is, and what Glutter should do. An entry is removed when the fix is in the Glutter version Radio Tutor uses; the file is deleted when no entry is left.

## ButtonStyle.TextStyle hides the label

- **Where seen:** step 6.1, `internal/gui/app/theme.go` (`NewTheme`).
- **Problem:** a button whose style was made with `FilledButtonStyleFrom(ButtonStyleFromOpts{TextStyle: ...})` (also `OutlinedButtonStyleFrom` and `TextButtonStyleFrom`), set on the button (`FilledButtonOpts.Style`) or on the theme (`FilledButtonThemeData.Style`), draws no label: the button background and shape are right, the text is missing. Adding `ForegroundColor` to the same `ButtonStyleFromOpts` does not bring it back. Without `TextStyle` the label is drawn.
- **Workaround:** the theme's `TextTheme.LabelLarge` is set to the larger size and the button styles give no `TextStyle`.
- **Fix in Glutter:** a `ButtonStyle.TextStyle` (a size, a weight) must merge with the label style the button would have had, and the label must keep the foreground color.

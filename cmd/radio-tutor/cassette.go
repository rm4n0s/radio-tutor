package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"strconv"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/archive"
	"github.com/rm4n0s/radio-tutor/internal/core/problem"
	"github.com/rm4n0s/radio-tutor/internal/gui/svgcheck"
)

const (
	validateUsage = "usage: radio-tutor validate [--json] <file.cassette|folder>\n"
	packUsage     = "usage: radio-tutor pack [--force] <folder> <out.cassette>\n"
)

// cassetteDeps is what validate and pack need from their surroundings, so that
// tests can replace the renderer.
type cassetteDeps struct {
	stdout, stderr *printer
	// render checks that each picture can be drawn; nil skips the check.
	render archive.RenderChecker
}

// defaultRender is the renderer the real commands use: Glutter's SVG parser.
func defaultRender() archive.RenderChecker { return svgcheck.Checker{} }

// parseFlags parses args with fs, which has no output of its own: a bad flag
// prints the message and usage on stderr and gives exitUsage, and -h or --help
// prints the usage on stdout and gives exitOK. ok is true when the command
// goes on, with the positional arguments in fs.Args().
func parseFlags(fs *flag.FlagSet, args []string, usage string, d cassetteDeps) (code int, ok bool) {
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	err := fs.Parse(args)
	switch {
	case err == nil:
		return exitOK, true
	case errors.Is(err, flag.ErrHelp):
		d.stdout.printf("%s", usage)
		return exitOK, false
	default:
		d.stderr.printf("radio-tutor %s: %v\n%s", fs.Name(), err, usage)
		return exitUsage, false
	}
}

// validateCommand runs `radio-tutor validate [--json] <file.cassette|folder>`
// (SPEC §14) and returns the exit code: 0 when the cassette has no errors
// (warnings are allowed), 1 when it has, or when it could not be read at all, 2
// for a usage error. The problems go to stdout, one per line as "path: message"
// or as a JSON array; the summary line goes to stderr.
func validateCommand(ctx context.Context, args []string, d cassetteDeps) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print the problems as a JSON array")
	if code, ok := parseFlags(fs, args, validateUsage, d); !ok {
		return code
	}
	if fs.NArg() != 1 {
		d.stderr.printf("%s", validateUsage)
		return exitUsage
	}
	path := fs.Arg(0)

	report, err := archive.ValidatePath(ctx, path, d.render)
	if err != nil {
		d.stderr.printf("radio-tutor validate: %v\n", err)
		return exitError
	}
	if *asJSON {
		if err := writeJSON(d.stdout, report.Problems); err != nil {
			d.stderr.printf("radio-tutor validate: %v\n", err)
			return exitError
		}
	} else {
		writeText(d.stdout, report.Problems)
		d.stderr.printf("%s\n", summary(path, report.Problems))
	}
	if !report.Valid() {
		return exitError
	}
	return exitOK
}

// packCommand runs `radio-tutor pack [--force] <folder> <out.cassette>`
// (SPEC §14) and returns the exit code: 0 when the cassette was written, 1
// when the folder has errors (they are printed on stderr and nothing is
// written) or the file could not be written, 2 for a usage error.
func packCommand(ctx context.Context, args []string, d cassetteDeps) int {
	fs := flag.NewFlagSet("pack", flag.ContinueOnError)
	force := fs.Bool("force", false, "replace the output file when it exists")
	if code, ok := parseFlags(fs, args, packUsage, d); !ok {
		return code
	}
	if fs.NArg() != 2 {
		d.stderr.printf("%s", packUsage)
		return exitUsage
	}
	dir, out := fs.Arg(0), fs.Arg(1)

	report, err := archive.ValidatePath(ctx, dir, d.render)
	if err != nil {
		d.stderr.printf("radio-tutor pack: %v\n", err)
		return exitError
	}
	if !report.Valid() {
		writeText(d.stderr, report.Problems)
		d.stderr.printf("radio-tutor pack: %s; nothing was written\n", summary(dir, report.Problems))
		return exitError
	}
	writeText(d.stderr, report.Problems) // the warnings
	ps, err := archive.Pack(ctx, dir, out, *force)
	if err != nil {
		d.stderr.printf("radio-tutor pack: %v\n", err)
		return exitError
	}
	if problem.HasErrors(ps) {
		writeText(d.stderr, ps)
		d.stderr.printf("radio-tutor pack: %s; nothing was written\n", summary(dir, ps))
		return exitError
	}
	d.stdout.printf("Packed %s into %s\n", dir, out)
	return exitOK
}

// writeText prints each problem on its own line, as problem.String formats it.
func writeText(p *printer, ps []problem.Problem) {
	for _, pr := range ps {
		p.printf("%s\n", pr.String())
	}
}

// jsonProblem is how a problem is written by `validate --json`; unlike
// problem.Problem it always has the "warning" field.
type jsonProblem struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Warning bool   `json:"warning"`
}

// writeJSON writes the problems as a JSON array (an empty one is "[]").
// Tag: ValidateJSONFailed.
func writeJSON(w io.Writer, ps []problem.Problem) error {
	list := make([]jsonProblem, 0, len(ps))
	for _, pr := range ps {
		list = append(list, jsonProblem{Path: pr.Path, Message: pr.Message, Warning: pr.Warning})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(list); err != nil {
		return errors.NewErr("ValidateJSONFailed", err)
	}
	return nil
}

// summary says in one line how a validation of path went.
func summary(path string, ps []problem.Problem) string {
	errs := len(problem.Errors(ps))
	warns := len(ps) - errs
	counts := plural(errs, "error") + ", " + plural(warns, "warning")
	if errs == 0 {
		return path + " is valid (" + counts + ")"
	}
	return path + " is not valid (" + counts + ")"
}

// plural returns "1 error", "2 errors".
func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}

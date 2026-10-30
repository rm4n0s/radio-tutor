// Package archtest checks the package boundaries and the forbidden imports of
// SPEC §5. It has no production code besides the checker used by its tests:
// the checker reads the import clauses of every Go file of the module and does
// not need the packages to compile, or even to exist yet.
package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/rm4n0s/errors"
)

const (
	// GlutterPrefix is the import path of Glutter and all its packages.
	GlutterPrefix = "github.com/rm4n0s/glutter"
	// ModulePath is the module path of Radio Tutor.
	ModulePath = "github.com/rm4n0s/radio-tutor"

	corePrefix  = "internal/core"
	guiPrefix   = "internal/gui"
	toolsPrefix = "scripts/"
	audioDir    = "internal/core/audio"
	audioTag    = "radiotutor_audio"
	wineDir     = "cmd/radio-tutor"
	wineTag     = "radiotutor_wine"
)

// httpAllowed lists the core packages (directories relative to the module root)
// that may import net/http in non-test files.
var httpAllowed = map[string]bool{
	"internal/core/grading/openai": true,
	"internal/core/providers":      true,
	"internal/core/phone":          true,
}

// File is the import information of one Go file.
type File struct {
	// Path is the slash-separated path of the file relative to the module root.
	Path string
	// Imports are the import paths of the file.
	Imports []string
	// BuildTags is the text of the file's //go:build line, or empty.
	BuildTags string
}

// Violation is a broken rule in one file.
type Violation struct {
	// File is the path of the file relative to the module root.
	File string
	// Import is the offending import path.
	Import string
	// Rule says which rule the file breaks.
	Rule string
}

// String formats the violation for a test message.
func (v Violation) String() string {
	return v.File + ": imports " + strconv.Quote(v.Import) + ": " + v.Rule
}

// isCore reports whether the slash path (a file or directory relative to the
// module root) is inside internal/core.
func isCore(p string) bool { return p == corePrefix || strings.HasPrefix(p, corePrefix+"/") }

// isTestFile reports whether the file is a _test.go file.
func isTestFile(p string) bool { return strings.HasSuffix(p, "_test.go") }

// Check applies the rules of SPEC §5 to the files and returns the violations,
// sorted by file and import.
func Check(files []File) []Violation {
	var out []Violation
	for _, f := range files {
		dir := path.Dir(f.Path)
		for _, imp := range f.Imports {
			if isCore(f.Path) {
				if imp == GlutterPrefix || strings.HasPrefix(imp, GlutterPrefix+"/") {
					out = append(out, Violation{f.Path, imp, "the core never imports Glutter"})
				}
				if imp == ModulePath+"/"+guiPrefix || strings.HasPrefix(imp, ModulePath+"/"+guiPrefix+"/") {
					out = append(out, Violation{f.Path, imp, "the core never imports internal/gui"})
				}
				if imp == "net/http" && !isTestFile(f.Path) && !httpAllowed[dir] {
					out = append(out, Violation{f.Path, imp, "only grading/openai, providers and phone import net/http in the core"})
				}
			}
			if imp == "os/exec" && !execAllowed(f, dir) {
				out = append(out, Violation{f.Path, imp, "os/exec is only for the audio test helpers behind the " + audioTag + " tag, the Wine smoke test of cmd/radio-tutor behind the " + wineTag + " tag and the developer tools under " + toolsPrefix})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Import < out[j].Import
	})
	return out
}

// execAllowed reports whether the file may import os/exec: a test helper of
// internal/core/audio behind the radiotutor_audio build tag, the Wine smoke
// test of cmd/radio-tutor behind the radiotutor_wine tag (it starts Wine through
// scripts/wine-exec.sh, SPEC §16), or a developer tool under scripts/ (such as the sample cassette's generator), which is not
// part of the application.
func execAllowed(f File, dir string) bool {
	if strings.HasPrefix(f.Path, toolsPrefix) {
		return true
	}
	if dir == wineDir && isTestFile(f.Path) && strings.Contains(f.BuildTags, wineTag) {
		return true
	}
	return dir == audioDir && isTestFile(f.Path) && strings.Contains(f.BuildTags, audioTag)
}

// Scan reads the import clauses of every Go file in fsys (the module root),
// skipping testdata folders, hidden folders and vendor, and returns them sorted
// by path.
func Scan(fsys fs.FS) ([]File, error) {
	var files []File
	fset := token.NewFileSet()
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return errors.NewErr("ScanWalkFailed", err).SetMetadata("path", p)
		}
		name := d.Name()
		if d.IsDir() {
			if p != "." && (name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		src, err := fs.ReadFile(fsys, p)
		if err != nil {
			return errors.NewErr("ScanReadFailed", err).SetMetadata("path", p)
		}
		parsed, err := parser.ParseFile(fset, p, src, parser.ImportsOnly|parser.ParseComments)
		if err != nil {
			return errors.NewErr("ScanParseFailed", err).SetMetadata("path", p)
		}
		f := File{Path: p, BuildTags: buildLine(parsed)}
		for _, spec := range parsed.Imports {
			f.Imports = append(f.Imports, strings.Trim(spec.Path.Value, "\"`"))
		}
		files = append(files, f)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// buildLine returns the text of the //go:build line that comes before the
// package clause, or an empty string.
func buildLine(f *ast.File) string {
	for _, group := range f.Comments {
		if group.End() >= f.Package {
			break
		}
		for _, c := range group.List {
			if rest, ok := strings.CutPrefix(c.Text, "//go:build "); ok {
				return rest
			}
		}
	}
	return ""
}

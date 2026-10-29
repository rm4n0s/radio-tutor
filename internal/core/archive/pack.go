package archive

import (
	"archive/zip"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/problem"
)

// The modification time of every entry of a packed cassette: the zip epoch,
// 1980-01-01 00:00:00, as a MS-DOS date (year-1980 << 9 | month << 5 | day) and
// time. A fixed time, not the files' own, makes the zip depend on the contents
// only: packing the same folder twice gives the same bytes, whatever the
// files' times, which is what a parent who shares a cassette expects to compare.
const (
	packDOSDate = 0<<9 | 1<<5 | 1
	packDOSTime = 0
)

// Pack writes the cassette folder dir as the zip file out, and returns the
// problems that stopped it: when they have errors nothing was written.
//
// The folder must follow the layout of CASSETTE-FORMAT §1 and the limits of §9
// (the same rules as Validate's first check; Pack does not check the contents of
// the documents, run Validate for that). The zip has one entry per file, with
// names sorted bytewise, deflate compression, the fixed modification time
// above, mode 0644 and no extra fields, so the same folder always gives the same
// bytes with the same Go version. Folders are not written as entries.
//
// The zip is written to a temporary file next to out, opened again with
// OpenZip to make sure that every limit is met (a very compressible file can
// break the ratio of §9), and renamed to out. An existing out is an error
// unless force is true. Nothing is left behind on failure.
func Pack(ctx context.Context, dir, out string, force bool) ([]problem.Problem, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, errors.NewErr("ArchivePackFolderStatFailed", err).SetMetadata("path", dir)
	}
	if !info.IsDir() {
		return nil, errors.New("ArchivePackNotFolder", "the cassette to pack must be a folder", "path", dir)
	}
	if !force {
		if _, err := os.Lstat(out); err == nil {
			return nil, errors.New("ArchiveOutputExists", "the output file already exists; use force to replace it", "path", out)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, errors.NewErr("ArchiveOutputStatFailed", err).SetMetadata("path", out)
		}
	}

	tree, ps, _, err := scanFS(ctx, os.DirFS(dir))
	if err != nil {
		return nil, err
	}
	if problem.HasErrors(ps) {
		return ps, nil
	}

	tmp, err := os.CreateTemp(filepath.Dir(out), "."+filepath.Base(out)+".tmp-*")
	if err != nil {
		return nil, errors.NewErr("ArchivePackCreateFailed", err).SetMetadata("path", out)
	}
	tmpName := tmp.Name()
	done := false
	defer func() {
		if !done {
			_ = tmp.Close() // the temporary file is being thrown away
			_ = os.Remove(tmpName)
		}
	}()

	zw := zip.NewWriter(tmp)
	if err := writeEntries(ctx, zw, tree); err != nil {
		return nil, err
	}
	if err := finishZip(zw, tmp); err != nil {
		return nil, err
	}

	_, closer, vps, err := OpenZip(ctx, tmpName)
	if err != nil {
		return nil, err
	}
	_ = closer.Close() // read only
	if problem.HasErrors(vps) {
		return vps, nil
	}
	if err := os.Rename(tmpName, out); err != nil {
		return nil, errors.NewErr("ArchivePackRenameFailed", err).SetMetadata("path", out)
	}
	done = true
	return nil, nil
}

// writeEntries writes the files of tree, sorted, as entries of zw.
func writeEntries(ctx context.Context, zw *zip.Writer, tree *treeFS) error {
	names := make([]string, 0, len(tree.files))
	for name := range tree.files {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		if err := cancelCheck(ctx, "packing "+name); err != nil {
			return err
		}
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		// The MS-DOS fields are the only way to give a time without the
		// extended-timestamp extra field that Modified adds.
		h.ModifiedDate, h.ModifiedTime = packDOSDate, packDOSTime //nolint:staticcheck // see above
		h.SetMode(0o644)
		if err := writeEntry(ctx, zw, h, tree); err != nil {
			if _, ok := errors.FromError(err); ok {
				return err // from tree.Open or the context
			}
			return errors.NewErr("ArchivePackEntryFailed", err).SetMetadata("path", name)
		}
	}
	return nil
}

// writeEntry writes the file h.Name of tree as one entry of zw. The error is
// the zip writer's own, or one of the tree's.
func writeEntry(ctx context.Context, zw *zip.Writer, h *zip.FileHeader, tree *treeFS) error {
	ew, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	src, err := tree.Open(h.Name)
	if err != nil {
		return err
	}
	_, err = io.Copy(ew, ctxReader{ctx: ctx, r: src})
	_ = src.Close() // read only
	return err
}

// finishZip writes the end of the zip, makes the file readable by everyone and
// closes it. The first step that fails is the failure; the file is closed even
// when an earlier step failed.
func finishZip(zw *zip.Writer, f *os.File) error {
	err := zw.Close()
	if cerr := f.Chmod(0o644); err == nil {
		err = cerr
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return errors.NewErr("ArchivePackFinishFailed", err).SetMetadata("path", f.Name())
	}
	return nil
}

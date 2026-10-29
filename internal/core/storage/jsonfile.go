package storage

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rm4n0s/errors"
)

// brokenTimeLayout is the layout of the time in a quarantined file's name.
const brokenTimeLayout = "20060102T150405Z"

// EnsureDir makes the folder and its parents with DirMode. It fails with the tag
// DirCreateFailed.
func EnsureDir(path string) error {
	if err := os.MkdirAll(path, DirMode); err != nil {
		return errors.NewErr("DirCreateFailed", err).SetMetadata("path", path)
	}
	return nil
}

// atomicOps are the file operations of an atomic write. Production code uses
// realAtomicOps; tests replace single operations to make them fail.
type atomicOps struct {
	write   func(f *os.File, data []byte) error
	sync    func(f *os.File) error
	close   func(f *os.File) error
	rename  func(oldPath, newPath string) error
	syncDir func(dir string) error
}

func realAtomicOps() atomicOps {
	return atomicOps{
		write:  func(f *os.File, data []byte) error { _, err := f.Write(data); return err },
		sync:   func(f *os.File) error { return f.Sync() },
		close:  func(f *os.File) error { return f.Close() },
		rename: os.Rename,
		syncDir: func(dir string) error {
			d, err := os.Open(dir)
			if err != nil {
				return err
			}
			if err := d.Sync(); err != nil {
				_ = d.Close() // the sync error is the one to report
				return err
			}
			return d.Close()
		},
	}
}

// WriteJSONAtomic writes v as indented JSON to path so that a reader sees
// either the old file or the new one, never a partial file (SPEC §4): the data
// goes to a temporary file in the same folder, which is synced and renamed over
// path, and then the folder is synced. The folder is created when missing. The
// file has FileMode. A failure leaves the previous file untouched and removes
// the temporary file. It fails with the tags WriteCancelled, JSONMarshalFailed,
// DirCreateFailed, TempFileCreateFailed, TempFileWriteFailed,
// TempFileSyncFailed, TempFileCloseFailed, RenameFailed and DirSyncFailed.
func WriteJSONAtomic(ctx context.Context, path string, v any) error {
	return writeJSONAtomic(ctx, path, v, realAtomicOps())
}

func writeJSONAtomic(ctx context.Context, path string, v any, ops atomicOps) error {
	if err := ctx.Err(); err != nil {
		return errors.NewErr("WriteCancelled", err).SetMetadata("path", path)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errors.NewErr("JSONMarshalFailed", err).SetMetadata("path", path)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := EnsureDir(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return errors.NewErr("TempFileCreateFailed", err).SetMetadata("path", path)
	}
	tmpName := tmp.Name()
	closed := false
	renamed := false
	defer func() {
		if !closed {
			_ = tmp.Close() // already failing; the original error is reported
		}
		if !renamed {
			_ = os.Remove(tmpName) // best effort: the temporary file is garbage now
		}
	}()

	if err := tmp.Chmod(FileMode); err != nil {
		return errors.NewErr("TempFileCreateFailed", err).SetMetadata("path", path)
	}
	if err := ops.write(tmp, data); err != nil {
		return errors.NewErr("TempFileWriteFailed", err).SetMetadata("path", path)
	}
	if err := ops.sync(tmp); err != nil {
		return errors.NewErr("TempFileSyncFailed", err).SetMetadata("path", path)
	}
	closed = true
	if err := ops.close(tmp); err != nil {
		return errors.NewErr("TempFileCloseFailed", err).SetMetadata("path", path)
	}
	if err := ops.rename(tmpName, path); err != nil {
		return errors.NewErr("RenameFailed", err).SetMetadata("path", path)
	}
	renamed = true
	if err := ops.syncDir(dir); err != nil {
		return errors.NewErr("DirSyncFailed", err).SetMetadata("path", path)
	}
	return nil
}

// ReadJSON reads the JSON file at path into v. A missing file fails with the tag
// JSONNotFound (errors.Is(err, fs.ErrNotExist) is true too), so the caller can
// start with an empty value; a file that cannot be read fails with
// JSONReadFailed; a file that is not valid JSON for v fails with JSONInvalid,
// and the caller then passes it to QuarantineBroken.
func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return errors.NewErr("JSONNotFound", err).SetMetadata("path", path)
		}
		return errors.NewErr("JSONReadFailed", err).SetMetadata("path", path)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return errors.NewErr("JSONInvalid", err).SetMetadata("path", path)
	}
	return nil
}

// QuarantineBroken renames the file at path to "<name>.broken-<time>" in the same
// folder, where <time> is now in UTC as 20060102T150405Z (SPEC §4), and returns
// the new path. It never overwrites an earlier quarantined file: when the name
// is taken it appends "-2", "-3" and so on. It fails with the tag
// QuarantineFailed.
func QuarantineBroken(path string, now time.Time) (string, error) {
	base := path + ".broken-" + now.UTC().Format(brokenTimeLayout)
	target := base
	for n := 2; ; n++ {
		_, err := os.Lstat(target)
		if errors.Is(err, fs.ErrNotExist) {
			break
		}
		if err != nil {
			return "", errors.NewErr("QuarantineFailed", err).SetMetadata("path", path)
		}
		target = base + "-" + strconv.Itoa(n)
	}
	if err := os.Rename(path, target); err != nil {
		return "", errors.NewErr("QuarantineFailed", err).SetMetadata("path", path, "target", target)
	}
	return target, nil
}

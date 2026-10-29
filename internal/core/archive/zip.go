package archive

import (
	"archive/zip"
	"context"
	"encoding/binary"
	"io"
	"io/fs"
	"os"
	"strconv"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/problem"
)

// maxDirectoryBytes bounds the central directory of a zip (the list of its
// entries, at the end of the file). Real entries need about 100 bytes of it,
// so this is far above MaxEntries entries; a bigger directory is refused before
// it is parsed, because parsing allocates for every entry.
const maxDirectoryBytes = 4 << 20

// OpenZip opens the .cassette file at path with the limits and name rules of
// CASSETTE-FORMAT §9 and returns a read-only view of the entries that passed.
//
// The checks, in this order: the size of the file; the size of its central
// directory; the number of entries; for each entry its name and place in the
// layout of §1 (zip-slip names, absolute names, backslashes, characters,
// duplicates, files that are not part of a cassette), its mode bits (a symbolic
// link, a device, a socket, a pipe and the setuid bits are refused), its
// compression method, the size it declares (at most 200 MiB), the ratio between
// that and its compressed size (at most 100), and the sum of the declared sizes
// (at most 1 GiB); then every accepted entry is unpacked once, counting the
// bytes that really come out, and an entry whose real size differs from the
// declared one, or whose checksum is wrong, is refused. The view returns only
// the entries that passed, and reading an entry is again capped to its checked
// size.
//
// The problems name the entries ("lessons/l1/scene.svg: ..."); problem.HasErrors
// says whether the archive can be used. When the archive is refused as a whole
// (too large, too many entries, too many bytes, not a zip) the view is empty.
// The Closer closes the file and must be called when the view is no longer
// used. The error is for failures that are not about the archive: the file
// cannot be opened or read, or ctx is done. When the error is nil the view and
// the Closer are not nil.
func OpenZip(ctx context.Context, path string) (fs.FS, io.Closer, []problem.Problem, error) {
	t, closer, ps, _, err := openZip(ctx, path)
	if err != nil {
		return nil, nil, nil, err
	}
	return t, closer, ps, nil
}

// openZip is OpenZip, also saying whether the archive was refused as a whole.
func openZip(ctx context.Context, path string) (*treeFS, io.Closer, []problem.Problem, bool, error) {
	if err := cancelCheck(ctx, "opening "+path); err != nil {
		return nil, nil, nil, false, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, nil, false, errors.NewErr("ArchiveStatFailed", err).SetMetadata("path", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, false, errors.NewErr("ArchiveOpenFailed", err).SetMetadata("path", path)
	}
	t, ps, fatal, err := readZip(ctx, f, info)
	if err != nil {
		_ = f.Close() // read only; the error is the one that matters
		return nil, nil, nil, false, err
	}
	return t, f, ps, fatal, nil
}

// readZip does the work of OpenZip on an open file.
func readZip(ctx context.Context, f *os.File, info fs.FileInfo) (*treeFS, []problem.Problem, bool, error) {
	refuse := func(format string, args ...any) (*treeFS, []problem.Problem, bool, error) {
		return newTree(), problem.List{problem.Errorf("", format, args...)}, true, nil
	}
	if info.IsDir() {
		return refuse("is a folder, not a .cassette file")
	}
	if info.Size() > MaxZipBytes {
		return refuse("the file is %d bytes; the limit is %d bytes (512 MiB)", info.Size(), MaxZipBytes)
	}
	if n, ok := directoryBytes(f, info.Size()); ok && n > maxDirectoryBytes {
		return refuse("the zip's list of entries is %d bytes, far more than %d entries need; the zip is refused", n, MaxEntries)
	}
	zr, err := zip.NewReader(f, info.Size())
	if err != nil && (zr == nil || !errors.Is(err, zip.ErrInsecurePath)) {
		// Insecure names are reported entry by entry below.
		return refuse("is not a zip file: %s", err.Error())
	}

	if len(zr.File) > MaxEntries {
		return refuse("the archive has %d entries; the limit is %d", len(zr.File), MaxEntries)
	}
	s := newScan()
	type candidate struct {
		file *zip.File
		name string
		dir  bool
	}
	var accepted []candidate
	for _, zf := range zr.File {
		name, isDir := trimDir(zf.Name)
		if !s.admitZip(zf, name, isDir) {
			if s.fatal {
				break
			}
			continue
		}
		accepted = append(accepted, candidate{zf, name, isDir})
		if isDir {
			s.tree.addDir(name)
		}
	}
	if s.fatal {
		t, ps, fatal := s.result()
		return t, ps, fatal, nil
	}

	for _, c := range accepted {
		if c.dir {
			continue
		}
		if err := cancelCheck(ctx, "unpacking "+c.name); err != nil {
			return nil, nil, false, err
		}
		msg, err := verifyEntry(ctx, c.file)
		if err != nil {
			return nil, nil, false, err
		}
		if msg != "" {
			s.c.errorf(display(c.name), "%s", msg)
			continue
		}
		zf := c.file
		s.tree.addFile(c.name, &treeEntry{size: int64(zf.UncompressedSize64), mod: zf.Modified, open: zf.Open})
	}
	t, ps, fatal := s.result()
	return t, ps, fatal, nil
}

// admitZip applies the rules of one zip entry: the common ones (admit) and the
// ones that need the zip's headers.
func (s *scan) admitZip(zf *zip.File, name string, isDir bool) bool {
	if !s.next() {
		return false
	}
	shown := display(name)
	mode := zf.Mode()
	const special = fs.ModeSymlink | fs.ModeDevice | fs.ModeNamedPipe | fs.ModeSocket | fs.ModeCharDevice | fs.ModeIrregular | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky
	switch {
	case mode&special != 0:
		s.c.errorf(shown, "is a symbolic link or a special file; only plain files and folders are allowed")
		return false
	case !isDir && mode.IsDir():
		s.c.errorf(shown, "is marked as a folder but its name does not end in /")
		return false
	}
	declared := zf.UncompressedSize64
	if !s.admit(name, isDir, int64(min(declared, 1<<62))) {
		return false
	}
	if isDir {
		if declared != 0 {
			s.c.errorf(shown, "is a folder with data")
			return false
		}
		return true
	}
	switch {
	case zf.Flags&0x1 != 0:
		s.c.errorf(shown, "is encrypted; encrypted entries are not supported")
		return false
	case zf.Method != zip.Store && zf.Method != zip.Deflate:
		s.c.errorf(shown, "uses compression method %d; only stored and deflate are supported", zf.Method)
		return false
	case zf.UncompressedSize64 > 0 && zf.CompressedSize64 == 0,
		zf.CompressedSize64 > 0 && zf.UncompressedSize64/zf.CompressedSize64 > MaxRatio:
		s.c.errorf(shown, "unpacks to %d bytes from %d, more than %d times its size; it looks like a zip bomb", zf.UncompressedSize64, zf.CompressedSize64, MaxRatio)
		return false
	}
	return true
}

// verifyEntry unpacks one entry into nothing and returns a message when it is
// damaged or its real size is not the declared one. The error is for a
// cancelled context.
func verifyEntry(ctx context.Context, zf *zip.File) (string, error) {
	rc, err := zf.Open()
	if err != nil {
		return "cannot be opened: " + err.Error(), nil
	}
	defer func() { _ = rc.Close() }() // read only
	lim := &capReader{r: rc, name: zf.Name, want: int64(zf.UncompressedSize64)}
	_, err = io.Copy(io.Discard, ctxReader{ctx: ctx, r: lim})
	if err == nil {
		return "", nil
	}
	if appErr, ok := errors.FromError(err); ok {
		switch appErr.Tag {
		case "ArchiveCanceled":
			return "", err
		case "ArchiveEntryTooLarge":
			return "has more data than its header says (" + sizeString(lim.want) + "); the zip lies about the size", nil
		case "ArchiveEntryTruncated":
			return "has less data than its header says (" + sizeString(lim.want) + ")", nil
		}
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return "has less data than its header says (" + sizeString(lim.want) + ")", nil
	}
	if errors.Is(err, zip.ErrFormat) {
		// The zip package stops an entry that has more data than it declares
		// with ErrFormat before the counter above sees it.
		return "has more data than its header says (" + sizeString(lim.want) + "), or a damaged header", nil
	}
	return "is damaged: " + err.Error(), nil
}

func sizeString(n int64) string {
	return strconv.FormatInt(n, 10) + " bytes"
}

// directoryBytes returns the size in bytes of the central directory of the zip
// in r, found from the end-of-central-directory record (and the zip64 record
// when there is one). ok is false when the records cannot be found; the zip
// package then makes its own judgement.
func directoryBytes(r io.ReaderAt, size int64) (int64, bool) {
	const eocdLen, locatorLen, zip64Len = 22, 20, 56
	const eocdSig, locatorSig, zip64Sig = 0x06054b50, 0x07064b50, 0x06064b50
	tail := min(size, 65535+eocdLen)
	if tail < eocdLen {
		return 0, false
	}
	buf := make([]byte, tail)
	if _, err := r.ReadAt(buf, size-tail); err != nil && err != io.EOF {
		return 0, false
	}
	le := binary.LittleEndian
	pos := -1
	for i := len(buf) - eocdLen; i >= 0; i-- {
		if le.Uint32(buf[i:]) == eocdSig {
			pos = i
			break
		}
	}
	if pos < 0 {
		return 0, false
	}
	end := size - tail + int64(pos) // where the central directory ends
	offset := int64(le.Uint32(buf[pos+16:]))
	if pos >= locatorLen && le.Uint32(buf[pos-locatorLen:]) == locatorSig {
		// A zip64 record sits between the directory and the end record.
		zip64At := int64(le.Uint64(buf[pos-locatorLen+8:]))
		var rec [zip64Len]byte
		if zip64At < 0 || zip64At > size-zip64Len {
			return 0, false
		}
		if _, err := r.ReadAt(rec[:], zip64At); err != nil || le.Uint32(rec[:]) != zip64Sig {
			return 0, false
		}
		end = zip64At
		if offset == 0xFFFFFFFF {
			offset = int64(le.Uint64(rec[48:]))
		}
	}
	if offset < 0 || offset > end {
		return 0, false
	}
	return end - offset, true
}

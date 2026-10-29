package archive

import (
	"context"
	"crypto/rand"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/problem"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
)

// The markers in the names of the temporary folders next to the installed
// cassettes. An underscore is not allowed in a cassette id, so these names never
// collide with an installed cassette.
const (
	tmpMarker = "_tmp-"
	oldMarker = "_old-"
)

// Store keeps the installed cassettes (SPEC §6.2): one folder per cassette id in
// the cassettes folder of the data folder, holding the cassette as it was in
// the zip. It is safe for concurrent use: a mutex orders List, Get, Remove and
// the moment an import changes the folder; the long parts of an import
// (reading, validating and extracting the archive, asking for a confirmation)
// run without it, so List does not wait for them.
type Store struct {
	dir    string
	render RenderChecker

	mu sync.Mutex
	// ops are the file operations that can fail halfway; tests replace them.
	ops storeOps
}

// storeOps are the operations of a Store that change the cassettes folder.
type storeOps struct {
	rename    func(oldPath, newPath string) error
	removeAll func(path string) error
}

// NewStore returns the Store over paths.CassettesDir(). render checks the SVG
// files of an imported cassette (nil skips that check).
func NewStore(paths storage.Paths, render RenderChecker) *Store {
	return &Store{dir: paths.CassettesDir(), render: render, ops: storeOps{rename: os.Rename, removeAll: os.RemoveAll}}
}

// Installed is a cassette folder of the store.
type Installed struct {
	// ID is the cassette id, which is the name of its folder.
	ID string
	// Manifest is the cassette's manifest; the zero value when Broken.
	Manifest cassette.Manifest
	// Broken is true when the manifest cannot be read or does not hold the id
	// of the folder. A broken cassette can only be removed (SPEC §6.2).
	Broken bool
	// Problems say why the cassette is broken.
	Problems []problem.Problem
}

// state is what the store has for one cassette id.
type state struct {
	exists  bool
	broken  bool
	version cassette.Version
}

// List returns the installed cassettes sorted by id, the broken ones too.
// Temporary folders of imports and removals are not listed, and neither are
// files and folders whose names are not cassette ids. The error is for a
// failure to read the cassettes folder, or a done ctx.
func (s *Store) List(ctx context.Context) ([]Installed, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, errors.NewErr("StoreListFailed", err).SetMetadata("path", s.dir)
	}
	var out []Installed
	for _, e := range entries {
		if !e.IsDir() || !cassette.ValidCassetteID(e.Name()) {
			continue
		}
		m, ps, err := s.manifestOf(ctx, e.Name())
		if err != nil {
			return nil, err
		}
		in := Installed{ID: e.Name()}
		if m == nil || problem.HasErrors(ps) {
			in.Broken, in.Problems = true, ps
		} else {
			in.Manifest = *m
		}
		out = append(out, in)
	}
	return out, nil
}

// manifestOf reads the manifest of the installed cassette id. The manifest is
// nil when it cannot be used; the problems say why (an id that differs from the
// folder's is one).
func (s *Store) manifestOf(ctx context.Context, id string) (*cassette.Manifest, []problem.Problem, error) {
	m, ps, err := cassette.LoadManifest(ctx, os.DirFS(filepath.Join(s.dir, id)))
	if err != nil {
		return nil, nil, err
	}
	if m != nil && m.ID != id {
		ps = append(ps, problem.Errorf("cassette.json#id", "the manifest's id %q is not the name of its folder, %q", m.ID, id))
		return nil, ps, nil
	}
	if problem.HasErrors(ps) {
		return nil, ps, nil
	}
	return m, ps, nil
}

// Get returns the installed cassette id as a read-only view of its folder, with
// its manifest. It fails with the tags StoreIDInvalid, StoreNotInstalled and
// StoreBroken (the manifest cannot be read; the problems are in the error's
// metadata).
func (s *Store) Get(ctx context.Context, id string) (fs.FS, cassette.Manifest, error) {
	if !cassette.ValidCassetteID(id) {
		return nil, cassette.Manifest{}, errors.New("StoreIDInvalid", "not a cassette id", "id", id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.stateOf(ctx, id)
	if err != nil {
		return nil, cassette.Manifest{}, err
	}
	if !st.exists {
		return nil, cassette.Manifest{}, errors.New("StoreNotInstalled", "the cassette is not installed", "id", id)
	}
	m, ps, err := s.manifestOf(ctx, id)
	if err != nil {
		return nil, cassette.Manifest{}, err
	}
	if m == nil {
		return nil, cassette.Manifest{}, errors.New("StoreBroken", "the installed cassette's manifest cannot be read; remove it", "id", id, "problems", ps)
	}
	return os.DirFS(filepath.Join(s.dir, id)), *m, nil
}

// stateOf looks at the installed folder of id. The caller holds s.mu.
func (s *Store) stateOf(ctx context.Context, id string) (state, error) {
	info, err := os.Stat(filepath.Join(s.dir, id))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return state{}, nil
		}
		return state{}, errors.NewErr("StoreStatFailed", err).SetMetadata("id", id)
	}
	if !info.IsDir() {
		return state{}, nil
	}
	m, _, err := s.manifestOf(ctx, id)
	if err != nil {
		return state{}, err
	}
	if m == nil {
		return state{exists: true, broken: true}, nil
	}
	return state{exists: true, version: m.Version}, nil
}

// ImportStatus says what Import did.
type ImportStatus int

// The results of an import.
const (
	// ImportInvalid: the archive has errors (ImportResult.Problems); nothing
	// was installed.
	ImportInvalid ImportStatus = iota
	// ImportInstalled: the cassette was not installed and now is (also a
	// cassette whose installed copy was broken).
	ImportInstalled
	// ImportUpgraded: a newer version replaced the installed one.
	ImportUpgraded
	// ImportAlreadyInstalled: the same version is installed; nothing changed.
	ImportAlreadyInstalled
	// ImportNewerInstalled: a newer version is installed; nothing changed.
	ImportNewerInstalled
	// ImportDeclined: a newer version than the installed one was offered and
	// the confirmation said no; nothing changed.
	ImportDeclined
)

// String names the status.
func (s ImportStatus) String() string {
	switch s {
	case ImportInvalid:
		return "invalid"
	case ImportInstalled:
		return "installed"
	case ImportUpgraded:
		return "upgraded"
	case ImportAlreadyInstalled:
		return "already installed"
	case ImportNewerInstalled:
		return "newer version installed"
	case ImportDeclined:
		return "declined"
	}
	return "unknown"
}

// ImportResult is what Import did and found.
type ImportResult struct {
	// Status says what happened.
	Status ImportStatus
	// ID is the cassette's id; empty when the manifest could not be read.
	ID string
	// Version is the version in the archive; zero when the manifest could not
	// be read.
	Version cassette.Version
	// Previous is the installed version that was replaced (ImportUpgraded),
	// that is installed (ImportAlreadyInstalled, ImportNewerInstalled) or that
	// was asked about (ImportDeclined).
	Previous cassette.Version
	// Problems are all the problems for ImportInvalid; for the other results
	// the warnings of the cassette, plus a warning when the old version could
	// not be deleted.
	Problems []problem.Problem
}

// Import installs the cassette at path, a .cassette file (or a folder), by the
// rules of SPEC §6.2: it is read with the limits of CASSETTE-FORMAT §9 and fully
// validated before anything is written; an invalid one gives ImportInvalid and
// its problems. A valid one is extracted into a temporary folder
// cassettes/<id>_tmp-<random>/ and renamed to cassettes/<id>/.
//
// When the id is installed already: the same version gives
// ImportAlreadyInstalled and an older one ImportNewerInstalled, both without
// changes; a newer one is installed only if confirmUpgrade(old, new) returns
// true (a nil function declines), by renaming the old folder aside, renaming the
// new one in and deleting the old one; when the second rename fails the old
// folder is renamed back. An installed copy that is broken is replaced without
// asking. confirmUpgrade runs without the store's lock, so it may take as long
// as a dialog takes; if the installed cassette changed meanwhile the import
// fails with StoreChangedDuringImport and changes nothing.
//
// When ctx is cancelled the temporary folder is deleted and the error satisfies
// errors.Is(err, context.Canceled).
func (s *Store) Import(ctx context.Context, path string, confirmUpgrade func(old, new cassette.Version) bool) (ImportResult, error) {
	src, err := openPath(ctx, path)
	if err != nil {
		return ImportResult{}, err
	}
	defer func() { _ = src.closer.Close() }() // read only
	if src.fatal {
		return ImportResult{Status: ImportInvalid, Problems: src.problems}, nil
	}
	rep, err := validateView(ctx, src.fsys, src.problems, s.render)
	if err != nil {
		return ImportResult{}, err
	}
	if !rep.Valid() || rep.Cassette == nil {
		return ImportResult{Status: ImportInvalid, Problems: rep.Problems}, nil
	}
	m := rep.Cassette.Manifest
	res := ImportResult{ID: m.ID, Version: m.Version, Problems: problem.Warnings(rep.Problems)}

	before, err := s.lockedState(ctx, m.ID)
	if err != nil {
		return ImportResult{}, err
	}
	if status, done := decide(before, m.Version); done {
		res.Status, res.Previous = status, before.version
		return res, nil
	}
	if before.exists && !before.broken {
		res.Previous = before.version
		if confirmUpgrade == nil || !confirmUpgrade(before.version, m.Version) {
			res.Status = ImportDeclined
			return res, nil
		}
	}

	if err := storage.EnsureDir(s.dir); err != nil {
		return ImportResult{}, err
	}
	tmp := filepath.Join(s.dir, m.ID+tmpMarker+randomName())
	moved := false
	defer func() {
		if !moved {
			_ = s.ops.removeAll(tmp) // a failure leaves a folder that Recover deletes
		}
	}()
	if err := extract(ctx, src.fsys, tmp); err != nil {
		return ImportResult{}, err
	}
	if err := checkExtracted(ctx, tmp, m); err != nil {
		return ImportResult{}, err
	}
	if err := cancelCheck(ctx, "installing "+m.ID); err != nil {
		return ImportResult{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	after, err := s.stateOf(ctx, m.ID)
	if err != nil {
		return ImportResult{}, err
	}
	if after != before {
		// Another import got there first. If it left the same or a newer
		// version that is the answer; anything else would be an upgrade nobody
		// confirmed.
		if status, done := decide(after, m.Version); done {
			res.Status, res.Previous = status, after.version
			return res, nil
		}
		if after.exists && !after.broken {
			return ImportResult{}, errors.New("StoreChangedDuringImport", "the installed cassette changed while it was being imported; try again", "id", m.ID)
		}
	}
	leftover, err := s.swap(m.ID, tmp, after.exists)
	if err != nil {
		return ImportResult{}, err
	}
	moved = true
	if leftover != nil {
		res.Problems = append(res.Problems, problem.Warnf("", "the previous version could not be deleted (%s); it is removed the next time Radio Tutor starts", leftover.Error()))
	}
	res.Status = ImportInstalled
	if after.exists && !after.broken {
		res.Status = ImportUpgraded
	}
	return res, nil
}

// lockedState is stateOf under the lock.
func (s *Store) lockedState(ctx context.Context, id string) (state, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stateOf(ctx, id)
}

// decide compares what is installed with the version in the archive. done is
// true when the answer is final: the same or a newer version is installed.
func decide(st state, v cassette.Version) (status ImportStatus, done bool) {
	if !st.exists || st.broken {
		return ImportInstalled, false
	}
	switch v.Compare(st.version) {
	case 0:
		return ImportAlreadyInstalled, true
	case -1:
		return ImportNewerInstalled, true
	}
	return ImportUpgraded, false
}

// swap puts the extracted folder tmp in place of cassettes/<id>. The caller
// holds s.mu. leftover is the failure to delete the old folder after the swap,
// which is not a failure of the import.
func (s *Store) swap(id, tmp string, replace bool) (leftover, err error) {
	final := filepath.Join(s.dir, id)
	if !replace {
		if err := s.ops.rename(tmp, final); err != nil {
			return nil, errors.NewErr("StoreInstallFailed", err).SetMetadata("id", id)
		}
		return nil, nil
	}
	old := final + oldMarker + randomName()
	if err := s.ops.rename(final, old); err != nil {
		return nil, errors.NewErr("StoreAsideFailed", err).SetMetadata("id", id)
	}
	if err := s.ops.rename(tmp, final); err != nil {
		restoreErr := s.ops.rename(old, final)
		e := errors.NewErr("StoreReplaceFailed", err).SetMetadata("id", id)
		if restoreErr != nil {
			return nil, e.SetMetadata("restore_error", restoreErr.Error(), "old_folder", old)
		}
		return nil, e
	}
	return s.ops.removeAll(old), nil
}

// Remove deletes the installed cassette id, a broken one too. The folder is
// renamed away first, so the cassette disappears at once and a failure to delete
// it fully leaves only a temporary folder that Recover deletes. Progress and
// photos are not touched: they belong to the progress package. It fails with the
// tags StoreIDInvalid, StoreNotInstalled and StoreRemoveFailed.
func (s *Store) Remove(ctx context.Context, id string) error {
	if !cassette.ValidCassetteID(id) {
		return errors.New("StoreIDInvalid", "not a cassette id", "id", id)
	}
	if err := cancelCheck(ctx, "removing "+id); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	final := filepath.Join(s.dir, id)
	info, err := os.Stat(final)
	if err != nil || !info.IsDir() {
		return errors.New("StoreNotInstalled", "the cassette is not installed", "id", id)
	}
	gone := final + oldMarker + randomName()
	if err := s.ops.rename(final, gone); err != nil {
		return errors.NewErr("StoreRemoveFailed", err).SetMetadata("id", id)
	}
	if err := s.ops.removeAll(gone); err != nil {
		return errors.NewErr("StoreRemoveLeftoverFailed", err).SetMetadata("id", id, "folder", gone)
	}
	return nil
}

// Recover cleans up after an import or a removal that was interrupted (a crash,
// a power cut): temporary folders of imports are deleted; a folder set aside by
// an upgrade is renamed back when the cassette's own folder is missing and
// deleted otherwise. Call it once when the app starts, before any import: it
// would delete the folder of an import that is running.
func (s *Store) Recover(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return errors.NewErr("StoreRecoverListFailed", err).SetMetadata("path", s.dir)
	}
	var first error
	for _, e := range entries {
		if err := cancelCheck(ctx, "recovering"); err != nil {
			return err
		}
		name := e.Name()
		var op func() error
		switch {
		case !e.IsDir():
			continue
		case strings.Contains(name, tmpMarker):
			op = func() error { return s.ops.removeAll(filepath.Join(s.dir, name)) }
		case strings.Contains(name, oldMarker):
			id, _, _ := strings.Cut(name, oldMarker)
			_, statErr := os.Stat(filepath.Join(s.dir, id))
			switch {
			case !cassette.ValidCassetteID(id):
				continue
			case errors.Is(statErr, fs.ErrNotExist):
				op = func() error { return s.ops.rename(filepath.Join(s.dir, name), filepath.Join(s.dir, id)) }
			case statErr == nil:
				op = func() error { return s.ops.removeAll(filepath.Join(s.dir, name)) }
			default:
				continue // cannot tell; keep the old version
			}
		default:
			continue
		}
		if err := op(); err != nil && first == nil {
			first = errors.NewErr("StoreRecoverFailed", err).SetMetadata("folder", name)
		}
	}
	return first
}

// randomName returns a random string for the names of temporary folders.
func randomName() string {
	return strings.ToLower(rand.Text()[:10])
}

// extract copies the files and folders of src into the new folder dst, which
// must not exist, with the store's modes (folders 0700, files 0600), checking
// ctx between chunks. Every file is synced.
func extract(ctx context.Context, src fs.FS, dst string) error {
	if err := os.Mkdir(dst, storage.DirMode); err != nil {
		return errors.NewErr("StoreExtractMkdirFailed", err).SetMetadata("path", dst)
	}
	return fs.WalkDir(src, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return errors.NewErr("StoreExtractReadFailed", err).SetMetadata("path", name)
		}
		if err := cancelCheck(ctx, "extracting "+name); err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		target := filepath.Join(dst, filepath.FromSlash(name))
		if d.IsDir() {
			if err := os.Mkdir(target, storage.DirMode); err != nil {
				return errors.NewErr("StoreExtractFolderFailed", err).SetMetadata("path", target)
			}
			return nil
		}
		if err := copyFile(ctx, src, name, target); err != nil {
			if _, ok := errors.FromError(err); ok {
				return err
			}
			return errors.NewErr("StoreExtractFileFailed", err).SetMetadata("path", target)
		}
		return nil
	})
}

// copyFile copies the file name of src to the new file target and syncs it. The
// errors are not tagged, except for the ones of src itself.
func copyFile(ctx context.Context, src fs.FS, name, target string) (err error) {
	in, err := src.Open(name)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }() // read only
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, storage.FileMode)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, ctxReader{ctx: ctx, r: in})
	if err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}

// checkExtracted makes sure that the folder that was written holds the
// manifest that was validated: an archive that changed on disk between the
// validation and the extraction must not install something else.
func checkExtracted(ctx context.Context, dir string, want cassette.Manifest) error {
	m, ps, err := cassette.LoadManifest(ctx, os.DirFS(dir))
	if err != nil {
		return err
	}
	if m == nil || problem.HasErrors(ps) || m.ID != want.ID || m.Version != want.Version {
		return errors.New("StoreSourceChanged", "the cassette changed while it was being imported; try again", "id", want.ID)
	}
	return nil
}

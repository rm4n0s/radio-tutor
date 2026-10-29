package progress

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
)

// Option configures a Store.
type Option func(*Store)

// WithClock sets the clock used to name a quarantined file. The default is
// time.Now.
func WithClock(now func() time.Time) Option {
	return func(s *Store) {
		if now != nil {
			s.now = now
		}
	}
}

// Store reads and writes the save files under progress/<username>/ (SPEC §4,
// §7.4). It is safe for concurrent use: one mutex serializes all its file
// operations (the files are small and the app has one student at a time). A
// caller that reads a Progress, changes it and saves it should use Update, so
// that a change made meanwhile by another goroutine (the submission service
// applying results) is not lost.
type Store struct {
	dir string
	now func() time.Time

	mu sync.Mutex
}

// NewStore returns a Store over paths.ProgressDir(). It creates nothing until
// the first Save.
func NewStore(paths storage.Paths, opts ...Option) *Store {
	s := &Store{dir: paths.ProgressDir(), now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// safeName reports whether name can be one path element of the progress
// folder: 1 to 128 letters, digits, '_', '-' or '.', not starting with '.'. It
// refuses "", ".", "..", separators and everything else that could leave the
// folder. Usernames and cassette ids are validated elsewhere; this is the
// guard against a path traversal if one of them ever slips through.
func safeName(name string) bool {
	if name == "" || len(name) > 128 || name[0] == '.' {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
		default:
			return false
		}
	}
	return true
}

// file returns the path of the save file. It fails with the tags
// ProgressUsernameInvalid and ProgressCassetteIDInvalid.
func (s *Store) file(username, cassetteID string) (string, error) {
	if !safeName(username) {
		return "", errors.New("ProgressUsernameInvalid", "the username cannot name a progress folder", "username", username)
	}
	if !safeName(cassetteID) {
		return "", errors.New("ProgressCassetteIDInvalid", "the cassette id cannot name a progress file", "cassette_id", cassetteID)
	}
	return filepath.Join(s.dir, username, cassetteID+".json"), nil
}

// userDir returns the folder of a student's save files. It fails with the tag
// ProgressUserDirUsernameInvalid.
func (s *Store) userDir(username string) (string, error) {
	if !safeName(username) {
		return "", errors.New("ProgressUserDirUsernameInvalid", "the username cannot name a progress folder", "username", username)
	}
	return filepath.Join(s.dir, username), nil
}

// Load reads the save file of the student in the cassette. A missing file
// gives a fresh Progress (New) and no error.
//
// A file that cannot be parsed, or that Validate refuses (SPEC §4: "A JSON
// file that cannot be parsed is not overwritten"), is renamed to
// "<name>.broken-<time>" and Load returns a fresh, usable Progress together with
// an error with the tag ProgressFileBroken, whose metadata has the path, the
// new name ("quarantined_to") and the reason; the UI shows it to the admin. In
// every other failure the Progress is nil: ProgressLoadCancelled,
// ProgressUsernameInvalid, ProgressCassetteIDInvalid, the storage tag
// JSONReadFailed and the storage tag QuarantineFailed.
func (s *Store) Load(ctx context.Context, username, cassetteID string) (*Progress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, errors.NewErr("ProgressLoadCancelled", err).SetMetadata("username", username, "cassette_id", cassetteID)
	}
	path, err := s.file(username, cassetteID)
	if err != nil {
		return nil, err
	}
	return s.loadFile(path, cassetteID)
}

// loadFile is Load for a path that is already checked. The caller holds s.mu.
func (s *Store) loadFile(path, cassetteID string) (*Progress, error) {
	p := &Progress{} // not New: a file without "format" must fail Validate
	if err := storage.ReadJSON(path, p); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return New(cassetteID), nil
		}
		appErr, ok := errors.FromError(err)
		if !ok || appErr.Tag != "JSONInvalid" {
			return nil, err
		}
		return s.quarantine(path, cassetteID, err)
	}
	if err := p.Validate(cassetteID); err != nil {
		return s.quarantine(path, cassetteID, err)
	}
	p.normalize() // after Validate: it would hide a missing or newer format
	return p, nil
}

// quarantine renames the broken file and returns a fresh Progress with the
// ProgressFileBroken error.
func (s *Store) quarantine(path, cassetteID string, reason error) (*Progress, error) {
	moved, err := storage.QuarantineBroken(path, s.now())
	if err != nil {
		return nil, err
	}
	return New(cassetteID), errors.New("ProgressFileBroken",
		"the save file could not be used; it was set aside and the progress starts empty",
		"path", path, "quarantined_to", moved, "reason", reason.Error())
}

// Save writes p as the student's save file for p.CassetteID, atomically. The
// Progress is normalized (Format set, nil maps made) and checked with Validate
// first. Concurrent callers should prefer Update. It fails with the tags of
// Validate, ProgressUsernameInvalid, ProgressCassetteIDInvalid and the tags of
// storage.WriteJSONAtomic.
func (s *Store) Save(ctx context.Context, username string, p *Progress) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.file(username, p.CassetteID)
	if err != nil {
		return err
	}
	return s.saveFile(ctx, path, p)
}

// saveFile is Save for a path that is already checked. The caller holds s.mu.
func (s *Store) saveFile(ctx context.Context, path string, p *Progress) error {
	p.normalize()
	if err := p.Validate(p.CassetteID); err != nil {
		return err
	}
	return storage.WriteJSONAtomic(ctx, path, p)
}

// Update loads the save file, runs fn on it and saves the result, all while
// holding the store's lock, so concurrent updates never lose each other's
// changes. When fn returns an error nothing is saved and Update returns that
// error. When the file was broken (see Load) fn runs on the fresh Progress, the
// result is saved, and Update returns the ProgressFileBroken error after
// saving, so the caller can tell the admin.
func (s *Store) Update(ctx context.Context, username, cassetteID string, fn func(p *Progress) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return errors.NewErr("ProgressUpdateCancelled", err).SetMetadata("username", username, "cassette_id", cassetteID)
	}
	path, err := s.file(username, cassetteID)
	if err != nil {
		return err
	}
	p, loadErr := s.loadFile(path, cassetteID)
	if p == nil {
		return loadErr
	}
	if err := fn(p); err != nil {
		return err
	}
	if err := s.saveFile(ctx, path, p); err != nil {
		return err
	}
	return loadErr
}

// DeleteAll removes the student's save file for the cassette, and the files
// that were set aside as broken. A student without a save file is not an
// error. It fails with the tags ProgressDeleteCancelled, ProgressUsernameInvalid,
// ProgressCassetteIDInvalid, ProgressDirReadFailed and ProgressFileDeleteFailed.
func (s *Store) DeleteAll(ctx context.Context, username, cassetteID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return errors.NewErr("ProgressDeleteCancelled", err).SetMetadata("username", username, "cassette_id", cassetteID)
	}
	path, err := s.file(username, cassetteID)
	if err != nil {
		return err
	}
	return removeCassetteFiles(filepath.Dir(path), cassetteID)
}

// DeleteUser removes every save file of the student. A student without a
// folder is not an error. It fails with the tags ProgressUserDeleteCancelled,
// ProgressUserDirUsernameInvalid and ProgressUserDeleteFailed.
func (s *Store) DeleteUser(ctx context.Context, username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return errors.NewErr("ProgressUserDeleteCancelled", err).SetMetadata("username", username)
	}
	dir, err := s.userDir(username)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return errors.NewErr("ProgressUserDeleteFailed", err).SetMetadata("username", username)
	}
	return nil
}

// DeleteCassette removes the save files of every student for the cassette (the
// checkbox of SPEC §6.2). It fails with the tags ProgressCassetteDeleteCancelled,
// ProgressCassetteDeleteIDInvalid, ProgressUsersListFailed, ProgressDirReadFailed and
// ProgressFileDeleteFailed.
func (s *Store) DeleteCassette(ctx context.Context, cassetteID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return errors.NewErr("ProgressCassetteDeleteCancelled", err).SetMetadata("cassette_id", cassetteID)
	}
	if !safeName(cassetteID) {
		return errors.New("ProgressCassetteDeleteIDInvalid", "the cassette id cannot name a progress file", "cassette_id", cassetteID)
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return errors.NewErr("ProgressUsersListFailed", err).SetMetadata("path", s.dir)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if err := removeCassetteFiles(filepath.Join(s.dir, e.Name()), cassetteID); err != nil {
			return err
		}
	}
	return nil
}

// removeCassetteFiles removes "<id>.json" and "<id>.json.broken-*" from a
// student's folder. A missing folder is fine.
func removeCassetteFiles(userDir, cassetteID string) error {
	entries, err := os.ReadDir(userDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return errors.NewErr("ProgressDirReadFailed", err).SetMetadata("path", userDir)
	}
	name := cassetteID + ".json"
	for _, e := range entries {
		if e.IsDir() || (e.Name() != name && !strings.HasPrefix(e.Name(), name+".broken-")) {
			continue
		}
		if err := os.Remove(filepath.Join(userDir, e.Name())); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return errors.NewErr("ProgressFileDeleteFailed", err).SetMetadata("path", filepath.Join(userDir, e.Name()))
		}
	}
	return nil
}

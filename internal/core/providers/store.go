package providers

import (
	"context"
	"io/fs"
	"os"
	"sync"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/storage"
)

// formatVersion is the format of providers.json.
const formatVersion = 1

// file is the content of providers.json.
type file struct {
	Format    int      `json:"format"`
	Active    string   `json:"active"`
	Providers []Config `json:"providers"`
}

// Store is the provider list in one providers.json (storage.Paths.ProvidersFile).
// It is safe for concurrent use; a change is checked and written atomically
// (mode 0600) before it becomes visible, and a failed change leaves the list as
// it was. Names are unique. The active provider is named in the file; there may
// be none.
type Store struct {
	path string

	mu     sync.Mutex
	active string
	list   []Config
}

// Open reads the providers file at path. A missing file is an empty list (it is
// written by the first change). A file that group or others can access is not
// read: it fails with ProvidersFileTooOpen, whose message tells the admin to
// run chmod 600 on it. A file that is not valid JSON fails with the storage tag
// JSONInvalid (the caller may then storage.QuarantineBroken it, SPEC §4). It
// also fails with ProvidersOpenCancelled (ctx done), ProvidersStatFailed,
// ProvidersFormatUnsupported, ProviderNameDuplicate and ProvidersActiveUnknown,
// and with the tags of Config checking (see Build) for an invalid provider.
func Open(ctx context.Context, path string) (*Store, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.NewErr("ProvidersOpenCancelled", err).SetMetadata("path", path)
	}
	s := &Store{path: path}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return s, nil
		}
		return nil, errors.NewErr("ProvidersStatFailed", err).SetMetadata("path", path)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return nil, errors.New("ProvidersFileTooOpen",
			"providers.json holds API keys but other users can read it; make it private with: chmod 600 "+path,
			"path", path, "mode", perm.String())
	}
	var f file
	if err := storage.ReadJSON(path, &f); err != nil {
		return nil, err
	}
	list, err := check(f)
	if err != nil {
		return nil, err
	}
	s.list, s.active = list, f.Active
	return s, nil
}

// check validates a file's content and returns the normalized providers.
func check(f file) ([]Config, error) {
	if f.Format != formatVersion {
		return nil, errors.New("ProvidersFormatUnsupported", "providers.json has an unsupported format", "format", f.Format)
	}
	list := make([]Config, 0, len(f.Providers))
	seen := make(map[string]bool, len(f.Providers))
	for _, c := range f.Providers {
		c, err := validate(c)
		if err != nil {
			return nil, err
		}
		if seen[c.Name] {
			return nil, errors.New("ProviderNameDuplicate", "two providers have the same name", "name", c.Name)
		}
		seen[c.Name] = true
		list = append(list, c)
	}
	if f.Active != "" && !seen[f.Active] {
		return nil, errors.New("ProvidersActiveUnknown", "the active provider is not in the list", "active", f.Active)
	}
	return list, nil
}

// List returns a copy of the providers in the order they were added.
func (s *Store) List() []Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Config{}, s.list...)
}

// Get returns the provider with the name.
func (s *Store) Get(name string) (Config, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := s.index(name); i >= 0 {
		return s.list[i], true
	}
	return Config{}, false
}

// Active returns the active provider; false when none is set.
func (s *Store) Active() (Config, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := s.index(s.active); s.active != "" && i >= 0 {
		return s.list[i], true
	}
	return Config{}, false
}

// ActiveName returns the name of the active provider, empty when none is set.
func (s *Store) ActiveName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

// index returns the position of the provider, or -1.
func (s *Store) index(name string) int {
	for i, c := range s.list {
		if c.Name == name {
			return i
		}
	}
	return -1
}

// commit writes the new state and, when that worked, makes it current.
func (s *Store) commit(ctx context.Context, active string, list []Config) error {
	if err := storage.WriteJSONAtomic(ctx, s.path, file{Format: formatVersion, Active: active, Providers: list}); err != nil {
		return err
	}
	s.active, s.list = active, list
	return nil
}

// Add checks the provider (defaults are filled: protocol openai, timeout 180 s,
// json_schema) and appends it. It does not make it active. It fails with
// ProviderNameDuplicate when the name is taken, with the tags of Config checking
// (see Build), and with the failures of the atomic write.
func (s *Store) Add(ctx context.Context, cfg Config) error {
	c, err := validate(cfg)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index(c.Name) >= 0 {
		return errors.New("ProviderNameDuplicate", "a provider with this name exists", "name", c.Name)
	}
	list := append(append([]Config{}, s.list...), c)
	return s.commit(ctx, s.active, list)
}

// Update replaces the provider called name with cfg, which is checked like in
// Add. The name may change (to one nobody else has); the active pointer follows
// the rename. cfg replaces every field, the API key included: to keep the key,
// pass it. It fails with ProviderNotFound, ProviderNameDuplicate, the tags of
// Config checking, and the failures of the atomic write.
func (s *Store) Update(ctx context.Context, name string, cfg Config) error {
	c, err := validate(cfg)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.index(name)
	if i < 0 {
		return errors.New("ProviderNotFound", "there is no provider with this name", "name", name)
	}
	if j := s.index(c.Name); j >= 0 && j != i {
		return errors.New("ProviderNameDuplicate", "a provider with this name exists", "name", c.Name)
	}
	list := append([]Config{}, s.list...)
	list[i] = c
	active := s.active
	if active == name {
		active = c.Name
	}
	return s.commit(ctx, active, list)
}

// Delete removes the provider; when it was the active one, no provider is
// active afterwards. It fails with ProviderNotFound and the failures of the
// atomic write.
func (s *Store) Delete(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.index(name)
	if i < 0 {
		return errors.New("ProviderNotFound", "there is no provider with this name", "name", name)
	}
	list := append(append([]Config{}, s.list[:i]...), s.list[i+1:]...)
	active := s.active
	if active == name {
		active = ""
	}
	return s.commit(ctx, active, list)
}

// SetActive makes the provider the active one. It fails with ProviderNotFound
// and the failures of the atomic write.
func (s *Store) SetActive(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index(name) < 0 {
		return errors.New("ProviderNotFound", "there is no provider with this name", "name", name)
	}
	return s.commit(ctx, name, s.list)
}

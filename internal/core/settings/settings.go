// Package settings keeps the app settings of Radio Tutor in settings.json
// (SPEC §12.4): the language of the UI, whether students may import cassettes,
// the phone page's port and interface, and the cassettes hidden from each
// student.
package settings

import (
	"context"
	"io/fs"
	"sync"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
)

const (
	formatVersion = 1
	// DefaultPhonePort is the default port of the phone page (SPEC D13).
	DefaultPhonePort = 8737
)

// Phone holds the settings of the phone page.
type Phone struct {
	// Port is the TCP port the phone server listens on, 1 to 65535.
	Port int `json:"port"`
	// Interface is the name of the network interface to take the address from;
	// empty means automatic.
	Interface string `json:"interface"`
}

// Settings is the content of settings.json.
type Settings struct {
	// Format is the file format version, always 1.
	Format int `json:"format"`
	// Language is the language of the UI: "en" or "el".
	Language i18n.Lang `json:"language"`
	// StudentsCanImport says whether students may import cassettes.
	StudentsCanImport bool `json:"students_can_import"`
	// Phone are the settings of the phone page.
	Phone Phone `json:"phone"`
	// HiddenCassettes lists, per username, the ids of the cassettes hidden from
	// that student.
	HiddenCassettes map[string][]string `json:"hidden_cassettes"`
}

// Defaults returns the settings of a first run: the language from the locale
// (the LANG variable: Greek for "el...", else English), students may import,
// port 8737, automatic interface, nothing hidden.
func Defaults(locale string) Settings {
	return Settings{
		Format:            formatVersion,
		Language:          i18n.FromLocale(locale),
		StudentsCanImport: true,
		Phone:             Phone{Port: DefaultPhonePort},
		HiddenCassettes:   map[string][]string{},
	}
}

// clone returns a copy that shares no map or slice with s.
func (s Settings) clone() Settings {
	hidden := make(map[string][]string, len(s.HiddenCassettes))
	for user, ids := range s.HiddenCassettes {
		hidden[user] = append([]string(nil), ids...)
	}
	s.HiddenCassettes = hidden
	return s
}

// validate checks the values. Tags: SettingsFormatUnsupported,
// SettingsLanguageInvalid, SettingsPortInvalid.
func validate(s Settings) error {
	switch {
	case s.Format != formatVersion:
		return errors.New("SettingsFormatUnsupported", "settings.json has an unsupported format", "format", s.Format)
	case !s.Language.Valid():
		return errors.New("SettingsLanguageInvalid", "the language must be en or el", "language", string(s.Language))
	case s.Phone.Port < 1 || s.Phone.Port > 65535:
		return errors.New("SettingsPortInvalid", "the phone port must be 1 to 65535", "port", s.Phone.Port)
	}
	return nil
}

// Store is the settings in one settings.json. It is safe for concurrent use; a
// change is written atomically before it becomes visible.
type Store struct {
	path string

	mu  sync.Mutex
	cur Settings
}

// Open reads the settings file at path (storage.Paths.SettingsFile). A missing
// file gives the defaults for the locale (the file is written by the first
// change); fields missing from the file keep their defaults. A file that is
// not valid JSON fails with the storage tag JSONInvalid (the caller may then
// storage.QuarantineBroken it, SPEC §4); an unsupported format, language or port
// fail with SettingsFormatUnsupported, SettingsLanguageInvalid or
// SettingsPortInvalid; a cancelled ctx with SettingsOpenCancelled.
func Open(ctx context.Context, path, locale string) (*Store, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.NewErr("SettingsOpenCancelled", err).SetMetadata("path", path)
	}
	s := &Store{path: path, cur: Defaults(locale)}
	read := s.cur.clone()
	if err := storage.ReadJSON(path, &read); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return s, nil
		}
		return nil, err
	}
	if read.HiddenCassettes == nil {
		read.HiddenCassettes = map[string][]string{}
	}
	if err := validate(read); err != nil {
		return nil, err
	}
	s.cur = read
	return s, nil
}

// Get returns a copy of the current settings.
func (s *Store) Get() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur.clone()
}

// Update applies change to a copy of the settings, checks the result and writes
// it. When the check or the write fails, the settings stay as they were. Tags:
// SettingsFormatUnsupported, SettingsLanguageInvalid, SettingsPortInvalid, and
// the failures of the atomic write.
func (s *Store) Update(ctx context.Context, change func(*Settings)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cur.clone()
	change(&next)
	if err := validate(next); err != nil {
		return err
	}
	if err := storage.WriteJSONAtomic(ctx, s.path, next); err != nil {
		return err
	}
	s.cur = next
	return nil
}

// SetLanguage changes the language of the UI and writes the file. It fails with
// SettingsLanguageInvalid for a language the UI does not have.
func (s *Store) SetLanguage(ctx context.Context, l i18n.Lang) error {
	return s.Update(ctx, func(st *Settings) { st.Language = l })
}

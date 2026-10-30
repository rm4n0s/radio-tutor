// Package profile keeps the profiles of Radio Tutor (SPEC §3.1): usernames,
// display names, roles and password hashes in profiles.json, and the login with
// its throttling.
package profile

import (
	"context"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
)

// Role is what a profile may do (SPEC §3.3).
type Role string

// The two roles.
const (
	RoleAdmin   Role = "admin"
	RoleStudent Role = "student"
)

// Valid reports whether r is one of the two roles.
func (r Role) Valid() bool { return r == RoleAdmin || r == RoleStudent }

// MinPasswordLength is the shortest password, in characters, that the role may
// have: 8 for admins, 4 for students. An invalid role gets the longer one.
func (r Role) MinPasswordLength() int {
	if r == RoleStudent {
		return 4
	}
	return 8
}

// Limits of SPEC §3.1 and of the throttle.
const (
	formatVersion      = 1
	maxDisplayNameLen  = 40
	maxFailedLogins    = 5
	throttleDuration   = 30 * time.Second
	unknownUserSaltLen = 16
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9_]{2,32}$`)

// Profile is a profile without its password hash.
type Profile struct {
	// Username is lower case and matches ^[a-z0-9_]{2,32}$.
	Username string
	// DisplayName is 1 to 40 characters.
	DisplayName string
	// Role is RoleAdmin or RoleStudent.
	Role Role
	// CreatedAt is the creation time (UTC).
	CreatedAt time.Time
}

// record is a profile as stored in profiles.json.
type record struct {
	Username     string    `json:"username"`
	DisplayName  string    `json:"display_name"`
	Role         Role      `json:"role"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

func (r record) profile() Profile {
	return Profile{Username: r.Username, DisplayName: r.DisplayName, Role: r.Role, CreatedAt: r.CreatedAt}
}

// document is the content of profiles.json.
type document struct {
	Format   int      `json:"format"`
	Profiles []record `json:"profiles"`
}

// Option configures a Store.
type Option func(*Store)

// WithClock sets the clock used for creation times and the login throttle. The
// default is time.Now.
func WithClock(now func() time.Time) Option {
	return func(s *Store) {
		if now != nil {
			s.now = now
		}
	}
}

// WithIterations sets the PBKDF2 iteration count of new hashes (the default is
// 600,000, SPEC §3.1). Hashes already stored keep, and are checked with, the
// count they carry. It is meant for tests; values below 1 are ignored.
func WithIterations(n int) Option {
	return func(s *Store) {
		if n >= 1 {
			s.hasher.iterations = n
		}
	}
}

// attempts counts the failed logins of one username.
type attempts struct {
	failed int
	until  time.Time
}

// Store is the set of profiles in one profiles.json. It is safe for concurrent
// use; every change is written to the file atomically before it becomes visible.
type Store struct {
	path   string
	now    func() time.Time
	hasher hasher

	mu       sync.Mutex
	records  []record
	throttle map[string]*attempts
}

// Open reads the profiles file at path (storage.Paths.ProfilesFile). A missing
// file is an empty store. A file that is not valid JSON fails with the storage
// tag JSONInvalid (the caller may then storage.QuarantineBroken it, SPEC §4); a
// "format" other than 1 fails with ProfilesFormatUnsupported; an invalid or
// repeated username, an invalid role or display name fail with
// ProfilesFileUsernameInvalid, ProfilesFileUsernameTwice, ProfilesFileRoleInvalid
// or ProfilesFileDisplayNameInvalid; a damaged password hash fails with one of
// the PasswordHash... tags of parseHash.
func Open(ctx context.Context, path string, opts ...Option) (*Store, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.NewErr("ProfilesOpenCancelled", err).SetMetadata("path", path)
	}
	s := &Store{path: path, now: time.Now, hasher: newHasher(), throttle: map[string]*attempts{}}
	for _, opt := range opts {
		opt(s)
	}
	var doc document
	if err := storage.ReadJSON(path, &doc); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return s, nil
		}
		return nil, err
	}
	if doc.Format != formatVersion {
		return nil, errors.New("ProfilesFormatUnsupported", "profiles.json has an unsupported format",
			"path", path, "format", doc.Format)
	}
	seen := map[string]bool{}
	for i, r := range doc.Profiles {
		switch {
		case !usernamePattern.MatchString(r.Username) || r.Username != strings.ToLower(r.Username):
			return nil, errors.New("ProfilesFileUsernameInvalid", "profiles.json has an invalid username", "path", path, "index", i)
		case seen[r.Username]:
			return nil, errors.New("ProfilesFileUsernameTwice", "profiles.json has a username twice", "path", path, "username", r.Username)
		case !r.Role.Valid():
			return nil, errors.New("ProfilesFileRoleInvalid", "profiles.json has an invalid role", "path", path, "username", r.Username)
		case !validDisplayName(r.DisplayName):
			return nil, errors.New("ProfilesFileDisplayNameInvalid", "profiles.json has an invalid display name", "path", path, "username", r.Username)
		}
		seen[r.Username] = true
		if _, err := parseHash(r.PasswordHash); err != nil {
			return nil, err
		}
	}
	s.records = doc.Profiles
	return s, nil
}

// validDisplayName reports whether name is 1 to 40 characters.
func validDisplayName(name string) bool {
	n := utf8.RuneCountInString(name)
	return utf8.ValidString(name) && n >= 1 && n <= maxDisplayNameLen && strings.TrimSpace(name) == name
}

// commit writes records as the new content of the file and, only when that
// succeeds, makes them the current ones. The caller holds s.mu.
func (s *Store) commit(ctx context.Context, records []record) error {
	if err := storage.WriteJSONAtomic(ctx, s.path, document{Format: formatVersion, Profiles: records}); err != nil {
		return err
	}
	s.records = records
	return nil
}

// find returns the index of the record of the (lower-cased) username, or -1.
func (s *Store) find(username string) int {
	for i, r := range s.records {
		if r.Username == username {
			return i
		}
	}
	return -1
}

// copyRecords returns a copy of the records with room for one more.
func (s *Store) copyRecords() []record {
	next := make([]record, len(s.records), len(s.records)+1)
	copy(next, s.records)
	return next
}

// checkPassword enforces the minimum length of the role.
func checkPassword(role Role, password string) error {
	min := role.MinPasswordLength()
	if utf8.RuneCountInString(password) < min {
		return errors.New("PasswordTooShort", "the password must have at least "+strconv.Itoa(min)+" characters", "role", string(role), "min", min)
	}
	return nil
}

// Create adds a profile. The username is lower-cased and must match
// ^[a-z0-9_]{2,32}$, the display name (trimmed) is 1 to 40 characters, the
// password has at least the role's minimum length. Tags: UsernameInvalid,
// UsernameReserved, DisplayNameInvalid, RoleInvalid, PasswordTooShort, UsernameTaken, plus the
// failures of hashing and of the atomic write.
func (s *Store) Create(ctx context.Context, username, displayName string, role Role, password string) (Profile, error) {
	username = strings.ToLower(username)
	displayName = strings.TrimSpace(displayName)
	if !usernamePattern.MatchString(username) {
		return Profile{}, errors.New("UsernameInvalid", "username must be 2 to 32 characters: a-z, 0-9 and _", "username", username)
	}
	if storage.ReservedName(username) {
		return Profile{}, errors.New("UsernameReserved", "this username is a name Windows reserves for a device (con, prn, aux, nul, com1 to com9, lpt1 to lpt9); choose another", "username", username)
	}
	if !validDisplayName(displayName) {
		return Profile{}, errors.New("DisplayNameInvalid", "display name must be 1 to 40 characters")
	}
	if !role.Valid() {
		return Profile{}, errors.New("RoleInvalid", "role must be admin or student", "role", string(role))
	}
	if err := checkPassword(role, password); err != nil {
		return Profile{}, err
	}
	hash, err := s.hasher.hash(password) // slow: done before taking the lock
	if err != nil {
		return Profile{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.find(username) >= 0 {
		return Profile{}, errors.New("UsernameTaken", "a profile with this username exists", "username", username)
	}
	rec := record{
		Username: username, DisplayName: displayName, Role: role,
		PasswordHash: hash, CreatedAt: s.now().UTC().Truncate(time.Second),
	}
	if err := s.commit(ctx, append(s.copyRecords(), rec)); err != nil {
		return Profile{}, err
	}
	return rec.profile(), nil
}

// List returns the profiles in creation order.
func (s *Store) List() []Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Profile, len(s.records))
	for i, r := range s.records {
		out[i] = r.profile()
	}
	return out
}

// Get returns the profile of username (case-insensitive), or fails with
// ProfileNotFound.
func (s *Store) Get(username string) (Profile, error) {
	username = strings.ToLower(username)
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.find(username)
	if i < 0 {
		return Profile{}, errors.New("ProfileNotFound", "no profile with this username", "username", username)
	}
	return s.records[i].profile(), nil
}

// NeedsFirstAdmin reports whether there is no admin yet, so the app must offer
// only "Create the parent account" (SPEC §3.1).
func (s *Store) NeedsFirstAdmin() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.admins() == 0
}

// admins counts the admins. The caller holds s.mu.
func (s *Store) admins() int {
	n := 0
	for _, r := range s.records {
		if r.Role == RoleAdmin {
			n++
		}
	}
	return n
}

// Rename changes the display name of username (the username never changes).
// Tags: DisplayNameInvalid, ProfileNotFound.
func (s *Store) Rename(ctx context.Context, username, displayName string) error {
	username = strings.ToLower(username)
	displayName = strings.TrimSpace(displayName)
	if !validDisplayName(displayName) {
		return errors.New("DisplayNameInvalid", "display name must be 1 to 40 characters")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.find(username)
	if i < 0 {
		return errors.New("ProfileNotFound", "no profile with this username", "username", username)
	}
	next := s.copyRecords()
	next[i].DisplayName = displayName
	return s.commit(ctx, next)
}

// SetPassword replaces the password of username. Tags: ProfileNotFound,
// PasswordTooShort (the minimum depends on the role).
func (s *Store) SetPassword(ctx context.Context, username, password string) error {
	username = strings.ToLower(username)
	p, err := s.Get(username)
	if err != nil {
		return err
	}
	if err := checkPassword(p.Role, password); err != nil {
		return err
	}
	hash, err := s.hasher.hash(password) // slow: done before taking the lock
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.find(username)
	if i < 0 { // deleted while hashing
		return errors.New("ProfileNotFound", "no profile with this username", "username", username)
	}
	next := s.copyRecords()
	next[i].PasswordHash = hash
	return s.commit(ctx, next)
}

// Delete removes the profile of username. The last admin cannot be deleted
// (LastAdmin); removing the student's progress and photos is the caller's job.
// Tags: ProfileNotFound, LastAdmin.
func (s *Store) Delete(ctx context.Context, username string) error {
	username = strings.ToLower(username)
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.find(username)
	if i < 0 {
		return errors.New("ProfileNotFound", "no profile with this username", "username", username)
	}
	if s.records[i].Role == RoleAdmin && s.admins() == 1 {
		return errors.New("LastAdmin", "the last admin cannot be deleted", "username", username)
	}
	next := make([]record, 0, len(s.records))
	next = append(next, s.records[:i]...)
	next = append(next, s.records[i+1:]...)
	if err := s.commit(ctx, next); err != nil {
		return err
	}
	delete(s.throttle, username)
	return nil
}

// Login checks the password of username (case-insensitive) and returns the
// profile. After 5 failed logins for a username, that username is refused for
// 30 seconds (LoginThrottled, in memory, per run); the refusal does not look at
// the password. A wrong password and an unknown username both fail with
// LoginFailed.
func (s *Store) Login(username, password string) (Profile, error) {
	key := strings.ToLower(username)
	now := s.now()
	s.mu.Lock()
	if t := s.throttle[key]; t != nil && !t.until.IsZero() {
		if now.Before(t.until) {
			wait := t.until.Sub(now)
			s.mu.Unlock()
			return Profile{}, errors.New("LoginThrottled", "too many failed logins, wait and try again",
				"username", key, "retry_after", wait.String())
		}
		delete(s.throttle, key) // the wait is over
	}
	var rec record
	i := s.find(key)
	if i >= 0 {
		rec = s.records[i]
	}
	s.mu.Unlock()

	ok := false
	if i >= 0 {
		var err error
		if ok, err = s.hasher.verify(password, rec.PasswordHash); err != nil {
			return Profile{}, err
		}
	} else {
		// An unknown username costs the same time as a wrong password.
		_, _ = s.hasher.derive(password, make([]byte, unknownUserSaltLen), s.hasher.iterations, keyLength)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if ok {
		delete(s.throttle, key)
		return rec.profile(), nil
	}
	t := s.throttle[key]
	if t == nil {
		t = &attempts{}
		s.throttle[key] = t
	}
	t.failed++
	if t.failed >= maxFailedLogins {
		t.failed = 0
		t.until = s.now().Add(throttleDuration)
	}
	return Profile{}, errors.New("LoginFailed", "wrong username or password", "username", key)
}

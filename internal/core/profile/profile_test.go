package profile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
)

// fakeClock is a clock the test moves by hand.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)}
}

// openStore opens a store on an isolated profiles.json with few iterations.
func openStore(t *testing.T, clock *fakeClock) (*Store, string) {
	t.Helper()
	path := storagetest.Isolate(t).ProfilesFile()
	return reopen(t, path, clock), path
}

func reopen(t *testing.T, path string, clock *fakeClock) *Store {
	t.Helper()
	s, err := Open(context.Background(), path, WithIterations(2), WithClock(clock.now))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

func mustCreate(t *testing.T, s *Store, username string, role Role, password string) Profile {
	t.Helper()
	p, err := s.Create(context.Background(), username, "Display "+username, role, password)
	if err != nil {
		t.Fatalf("Create %s: %v", username, err)
	}
	return p
}

// wantFailure checks the tag and route of err.
func wantFailure(t *testing.T, err error, tag, route string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s, got no error", tag)
	}
	appErr := routeOf(t, err)
	if appErr.Tag != tag || !appErr.HasRoute(route) {
		t.Fatalf("unexpected failure: tag %s, path %s; want %s", appErr.Tag, appErr.Route(), route)
	}
}

func TestCreateRules(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name        string
		username    string
		displayName string
		role        Role
		password    string
		wantTag     string // empty: success
	}{
		{"admin ok", "mama", "Mama", RoleAdmin, "12345678", ""},
		{"student ok", "kid", "Kid", RoleStudent, "1234", ""},
		{"upper case username is lower-cased", "Anna_2", "Anna", RoleStudent, "1234", ""},
		{"display name trimmed", "bob", "  Bob  ", RoleStudent, "1234", ""},
		{"display name of 40 characters", "bob", strings.Repeat("é", 40), RoleStudent, "1234", ""},
		{"password counts characters", "bob", "Bob", RoleStudent, "αβγδ", ""},
		{"username too short", "a", "A", RoleStudent, "1234", "UsernameInvalid"},
		{"username too long", strings.Repeat("a", 33), "A", RoleStudent, "1234", "UsernameInvalid"},
		{"username of 32", strings.Repeat("a", 32), "A", RoleStudent, "1234", ""},
		{"username with a dash", "a-b", "A", RoleStudent, "1234", "UsernameInvalid"},
		{"username with a space", "a b", "A", RoleStudent, "1234", "UsernameInvalid"},
		{"username with a non-ASCII letter", "αβ", "A", RoleStudent, "1234", "UsernameInvalid"},
		{"username is a name Windows reserves", "con", "A", RoleStudent, "1234", "UsernameReserved"},
		{"reserved username in any case", "NUL", "A", RoleStudent, "1234", "UsernameReserved"},
		{"reserved username com1", "com1", "A", RoleStudent, "1234", "UsernameReserved"},
		{"username that only starts like a reserved name", "console", "A", RoleStudent, "1234", ""},
		{"display name empty", "bob", "", RoleStudent, "1234", "DisplayNameInvalid"},
		{"display name blank", "bob", "   ", RoleStudent, "1234", "DisplayNameInvalid"},
		{"display name of 41 characters", "bob", strings.Repeat("é", 41), RoleStudent, "1234", "DisplayNameInvalid"},
		{"role invalid", "bob", "Bob", Role("teacher"), "12345678", "RoleInvalid"},
		{"admin password of 7", "mama", "Mama", RoleAdmin, "1234567", "PasswordTooShort"},
		{"student password of 3", "kid", "Kid", RoleStudent, "123", "PasswordTooShort"},
		{"student password of 3 characters, 6 bytes", "kid", "Kid", RoleStudent, "αβγ", "PasswordTooShort"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := openStore(t, newClock())
			p, err := s.Create(ctx, tt.username, tt.displayName, tt.role, tt.password)
			if tt.wantTag != "" {
				wantFailure(t, err, tt.wantTag, "Store.Create")
				if got := s.List(); len(got) != 0 {
					t.Errorf("a failed Create left %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if p.Username != strings.ToLower(tt.username) || p.Role != tt.role {
				t.Errorf("created %+v", p)
			}
			if got, err := s.Get(p.Username); err != nil || got != p {
				t.Errorf("Get = %+v, %v; want %+v", got, err, p)
			}
		})
	}
}

func TestCreateDuplicateUsernameIsCaseInsensitive(t *testing.T) {
	s, _ := openStore(t, newClock())
	mustCreate(t, s, "anna", RoleStudent, "1234")
	_, err := s.Create(context.Background(), "ANNA", "Other", RoleStudent, "1234")
	wantFailure(t, err, "UsernameTaken", "Store.Create.UsernameTaken")
	appErr := routeOf(t, err)
	if appErr.Metadata["username"] != "anna" {
		t.Errorf("metadata = %v", appErr.Metadata)
	}
	if len(s.List()) != 1 {
		t.Errorf("List = %v", s.List())
	}
}

func TestMinPasswordLength(t *testing.T) {
	for role, want := range map[Role]int{RoleAdmin: 8, RoleStudent: 4, Role("x"): 8} {
		if got := role.MinPasswordLength(); got != want {
			t.Errorf("%q: %d, want %d", role, got, want)
		}
	}
}

func TestListGetAndCreatedAt(t *testing.T) {
	clock := newClock()
	s, _ := openStore(t, clock)
	if !s.NeedsFirstAdmin() {
		t.Error("an empty store needs the first admin")
	}
	first := mustCreate(t, s, "mama", RoleAdmin, "12345678")
	clock.advance(time.Hour)
	second := mustCreate(t, s, "kid", RoleStudent, "1234")
	if s.NeedsFirstAdmin() {
		t.Error("NeedsFirstAdmin after creating an admin")
	}
	if got := s.List(); len(got) != 2 || got[0] != first || got[1] != second {
		t.Errorf("List = %+v", got)
	}
	if !second.CreatedAt.Equal(first.CreatedAt.Add(time.Hour)) || !first.CreatedAt.Equal(clock.t.Add(-time.Hour)) {
		t.Errorf("created at %v and %v", first.CreatedAt, second.CreatedAt)
	}
	// List returns a copy.
	list := s.List()
	list[0].DisplayName = "changed"
	if got, _ := s.Get("mama"); got.DisplayName == "changed" {
		t.Error("List shares memory with the store")
	}
	if got, err := s.Get("KID"); err != nil || got != second {
		t.Errorf("Get(KID) = %+v, %v", got, err)
	}
	_, err := s.Get("nobody")
	wantFailure(t, err, "ProfileNotFound", "Store.Get.ProfileNotFound")
}

func TestNeedsFirstAdminWithOnlyStudents(t *testing.T) {
	path := storagetest.Isolate(t).ProfilesFile()
	s := reopen(t, path, newClock())
	mustCreate(t, s, "kid", RoleStudent, "1234")
	if !s.NeedsFirstAdmin() {
		t.Error("a store with only students has no admin to manage it")
	}
}

func TestRename(t *testing.T) {
	ctx := context.Background()
	s, path := openStore(t, newClock())
	mustCreate(t, s, "kid", RoleStudent, "1234")

	if err := s.Rename(ctx, "KID", "  New Name "); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	got, _ := s.Get("kid")
	if got.DisplayName != "New Name" || got.Username != "kid" {
		t.Errorf("after Rename: %+v", got)
	}
	if again, _ := reopen(t, path, newClock()).Get("kid"); again.DisplayName != "New Name" {
		t.Errorf("Rename was not saved: %+v", again)
	}
	wantFailure(t, s.Rename(ctx, "kid", ""), "DisplayNameInvalid", "Store.Rename.DisplayNameInvalid")
	wantFailure(t, s.Rename(ctx, "kid", strings.Repeat("x", 41)), "DisplayNameInvalid", "Store.Rename.DisplayNameInvalid")
	wantFailure(t, s.Rename(ctx, "nobody", "X"), "ProfileNotFound", "Store.Rename.ProfileNotFound")
}

func TestSetPassword(t *testing.T) {
	ctx := context.Background()
	s, _ := openStore(t, newClock())
	mustCreate(t, s, "mama", RoleAdmin, "12345678")
	mustCreate(t, s, "kid", RoleStudent, "1234")

	if err := s.SetPassword(ctx, "MAMA", "new-password"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	if _, err := s.Login("mama", "12345678"); err == nil {
		t.Error("the old password still works")
	}
	if _, err := s.Login("mama", "new-password"); err != nil {
		t.Errorf("the new password does not work: %v", err)
	}
	wantFailure(t, s.SetPassword(ctx, "mama", "short"), "PasswordTooShort", "Store.SetPassword->checkPassword.PasswordTooShort")
	if err := s.SetPassword(ctx, "kid", "abcd"); err != nil {
		t.Errorf("a student password of 4: %v", err)
	}
	wantFailure(t, s.SetPassword(ctx, "kid", "abc"), "PasswordTooShort", "Store.SetPassword->checkPassword.PasswordTooShort")
	wantFailure(t, s.SetPassword(ctx, "nobody", "abcdefgh"), "ProfileNotFound", "Store.SetPassword->Store.Get.ProfileNotFound")
}

func TestSetPasswordHashFailure(t *testing.T) {
	s, _ := openStore(t, newClock())
	mustCreate(t, s, "kid", RoleStudent, "1234")
	s.hasher.derive = func(string, []byte, int, int) ([]byte, error) { return nil, errors.New("Fake", "boom") }
	wantFailure(t, s.SetPassword(context.Background(), "kid", "abcd"), "KeyDerivationFailed", "Store.SetPassword->hasher.hash.KeyDerivationFailed")
	_, err := s.Create(context.Background(), "other", "O", RoleStudent, "abcd")
	wantFailure(t, err, "KeyDerivationFailed", "Store.Create->hasher.hash.KeyDerivationFailed")
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	s, path := openStore(t, newClock())
	mustCreate(t, s, "mama", RoleAdmin, "12345678")
	mustCreate(t, s, "kid", RoleStudent, "1234")

	// The only admin stays even while students exist.
	wantFailure(t, s.Delete(ctx, "mama"), "LastAdmin", "Store.Delete.LastAdmin")
	if _, err := s.Get("mama"); err != nil {
		t.Errorf("the last admin was removed: %v", err)
	}
	if err := s.Delete(ctx, "KID"); err != nil {
		t.Fatalf("Delete student: %v", err)
	}
	wantFailure(t, s.Delete(ctx, "kid"), "ProfileNotFound", "Store.Delete.ProfileNotFound")

	// With a second admin the first one can go.
	mustCreate(t, s, "papa", RoleAdmin, "12345678")
	if err := s.Delete(ctx, "mama"); err != nil {
		t.Fatalf("Delete admin with another admin present: %v", err)
	}
	wantFailure(t, s.Delete(ctx, "papa"), "LastAdmin", "Store.Delete.LastAdmin")

	again := reopen(t, path, newClock())
	if got := again.List(); len(got) != 1 || got[0].Username != "papa" {
		t.Errorf("after reopening: %+v", got)
	}
}

func TestLogin(t *testing.T) {
	s, _ := openStore(t, newClock())
	mustCreate(t, s, "anna", RoleStudent, "pass")
	tests := []struct {
		name, username, password string
		wantTag                  string
	}{
		{"right", "anna", "pass", ""},
		{"user name in upper case", "ANNA", "pass", ""},
		{"user name mixed case", "AnNa", "pass", ""},
		{"wrong password", "anna", "pasS", "LoginFailed"},
		{"empty password", "anna", "", "LoginFailed"},
		{"unknown user", "nobody", "pass", "LoginFailed"},
		{"empty username", "", "pass", "LoginFailed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := s.Login(tt.username, tt.password)
			if tt.wantTag != "" {
				wantFailure(t, err, tt.wantTag, "Store.Login.LoginFailed")
				return
			}
			if err != nil || p.Username != "anna" || p.Role != RoleStudent {
				t.Errorf("Login = %+v, %v", p, err)
			}
		})
	}
}

func TestLoginThrottle(t *testing.T) {
	clock := newClock()
	s, _ := openStore(t, clock)
	mustCreate(t, s, "anna", RoleStudent, "pass")
	mustCreate(t, s, "ben", RoleStudent, "pass")

	// Four failures are plain failures.
	for i := range 4 {
		_, err := s.Login("anna", "bad")
		wantFailure(t, err, "LoginFailed", "Store.Login.LoginFailed")
		_ = i
	}
	// The fifth is a failure too, and starts the wait.
	_, err := s.Login("Anna", "bad")
	wantFailure(t, err, "LoginFailed", "Store.Login.LoginFailed")

	// While waiting, even the right password is refused, in any case.
	_, err = s.Login("ANNA", "pass")
	wantFailure(t, err, "LoginThrottled", "Store.Login.LoginThrottled")
	appErr := routeOf(t, err)
	if appErr.Metadata["retry_after"] != "30s" {
		t.Errorf("metadata = %v", appErr.Metadata)
	}
	clock.advance(29 * time.Second)
	_, err = s.Login("anna", "pass")
	wantFailure(t, err, "LoginThrottled", "Store.Login.LoginThrottled")
	appErr = routeOf(t, err)
	if appErr.Metadata["retry_after"] != "1s" {
		t.Errorf("metadata = %v", appErr.Metadata)
	}

	// Another username is not affected.
	if _, err := s.Login("ben", "pass"); err != nil {
		t.Errorf("ben was throttled with anna: %v", err)
	}

	// The wait expires after 30 seconds.
	clock.advance(time.Second)
	if _, err := s.Login("anna", "pass"); err != nil {
		t.Errorf("login after the wait: %v", err)
	}
}

func TestLoginThrottleRules(t *testing.T) {
	t.Run("success resets the count", func(t *testing.T) {
		s, _ := openStore(t, newClock())
		mustCreate(t, s, "anna", RoleStudent, "pass")
		for round := range 3 {
			for range 4 {
				_, err := s.Login("anna", "bad")
				wantFailure(t, err, "LoginFailed", "Store.Login")
			}
			if _, err := s.Login("anna", "pass"); err != nil {
				t.Fatalf("round %d: %v", round, err)
			}
		}
	})
	t.Run("a new wait needs five more failures", func(t *testing.T) {
		clock := newClock()
		s, _ := openStore(t, clock)
		mustCreate(t, s, "anna", RoleStudent, "pass")
		for range 5 {
			_, _ = s.Login("anna", "bad")
		}
		clock.advance(30 * time.Second)
		for range 4 {
			_, err := s.Login("anna", "bad")
			wantFailure(t, err, "LoginFailed", "Store.Login.LoginFailed")
		}
		_, err := s.Login("anna", "bad") // fifth after the wait
		wantFailure(t, err, "LoginFailed", "Store.Login.LoginFailed")
		_, err = s.Login("anna", "pass")
		wantFailure(t, err, "LoginThrottled", "Store.Login.LoginThrottled")
	})
	t.Run("unknown usernames are throttled too", func(t *testing.T) {
		s, _ := openStore(t, newClock())
		for range 5 {
			_, _ = s.Login("Ghost", "bad")
		}
		_, err := s.Login("ghost", "bad")
		wantFailure(t, err, "LoginThrottled", "Store.Login.LoginThrottled")
	})
	t.Run("the wait is in memory only", func(t *testing.T) {
		clock := newClock()
		s, path := openStore(t, clock)
		mustCreate(t, s, "anna", RoleStudent, "pass")
		for range 5 {
			_, _ = s.Login("anna", "bad")
		}
		if _, err := reopen(t, path, clock).Login("anna", "pass"); err != nil {
			t.Errorf("a new run starts without waits: %v", err)
		}
	})
	t.Run("deleting a profile clears its wait", func(t *testing.T) {
		s, _ := openStore(t, newClock())
		mustCreate(t, s, "mama", RoleAdmin, "12345678")
		mustCreate(t, s, "anna", RoleStudent, "pass")
		for range 5 {
			_, _ = s.Login("anna", "bad")
		}
		if err := s.Delete(context.Background(), "anna"); err != nil {
			t.Fatal(err)
		}
		mustCreate(t, s, "anna", RoleStudent, "pass")
		if _, err := s.Login("anna", "pass"); err != nil {
			t.Errorf("a new profile inherited the wait: %v", err)
		}
	})
}

func TestLoginStoredHashInvalid(t *testing.T) {
	s, _ := openStore(t, newClock())
	mustCreate(t, s, "anna", RoleStudent, "pass")
	s.records[0].PasswordHash = "garbage"
	_, err := s.Login("anna", "pass")
	wantFailure(t, err, "PasswordHashMalformed", "Store.Login->hasher.verify->parseHash.PasswordHashMalformed")
}

func TestLoginUsesIterationsOfStoredHash(t *testing.T) {
	s, path := openStore(t, newClock())
	mustCreate(t, s, "anna", RoleStudent, "pass") // 2 iterations
	other, err := Open(context.Background(), path, WithIterations(7))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Login("anna", "pass"); err != nil {
		t.Errorf("login with a different store setting: %v", err)
	}
}

func TestPersistence(t *testing.T) {
	clock := newClock()
	s, path := openStore(t, clock)
	mustCreate(t, s, "mama", RoleAdmin, "12345678")
	mustCreate(t, s, "kid", RoleStudent, "1234")

	info, err := os.Stat(path)
	if err != nil || (storagetest.UnixModes && info.Mode().Perm() != storage.FileMode) {
		t.Fatalf("profiles.json: %v, %v", info, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"format": 1`, `"username": "kid"`, `"display_name": "Display kid"`, `"role": "student"`,
		`"password_hash": "pbkdf2-sha256$2$`, `"created_at": "2026-10-30T12:00:00Z"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("profiles.json lacks %s:\n%s", want, data)
		}
	}
	if strings.Contains(string(data), "1234") && strings.Contains(string(data), `"password":`) {
		t.Error("the file seems to hold a plain password")
	}

	again := reopen(t, path, clock)
	if got, want := again.List(), s.List(); len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("reopened %+v, want %+v", got, want)
	}
	if _, err := again.Login("kid", "1234"); err != nil {
		t.Errorf("login after reopening: %v", err)
	}
}

func TestOpen(t *testing.T) {
	good := func() string {
		s := newHasher()
		s.iterations = 1
		h, err := s.hash("password")
		if err != nil {
			t.Fatal(err)
		}
		return h
	}()
	rec := func(username, role, name string) string {
		return fmt.Sprintf(`{"username":%q,"display_name":%q,"role":%q,"password_hash":%q,"created_at":"2026-01-01T00:00:00Z"}`,
			username, name, role, good)
	}
	doc := func(format int, recs ...string) string {
		return fmt.Sprintf(`{"format":%d,"profiles":[%s]}`, format, strings.Join(recs, ","))
	}
	tests := []struct {
		name    string
		content string // empty: no file
		wantTag string
		route   string
		count   int
	}{
		{"missing file is an empty store", "", "", "", 0},
		{"valid", doc(1, rec("mama", "admin", "Mama"), rec("kid", "student", "Kid")), "", "", 2},
		{"no profiles", doc(1), "", "", 0},
		{"format 2", doc(2, rec("mama", "admin", "Mama")), "ProfilesFormatUnsupported", "Open.ProfilesFormatUnsupported", 0},
		{"format missing", `{"profiles":[]}`, "ProfilesFormatUnsupported", "Open.ProfilesFormatUnsupported", 0},
		{"not json", "{", "JSONInvalid", "Open->ReadJSON.JSONInvalid", 0},
		{"username with upper case", doc(1, rec("Mama", "admin", "Mama")), "ProfilesFileUsernameInvalid", "Open.ProfilesFileUsernameInvalid", 0},
		{"username too short", doc(1, rec("m", "admin", "Mama")), "ProfilesFileUsernameInvalid", "Open.ProfilesFileUsernameInvalid", 0},
		{"username twice", doc(1, rec("mama", "admin", "Mama"), rec("mama", "student", "Mama")), "ProfilesFileUsernameTwice", "Open.ProfilesFileUsernameTwice", 0},
		{"role unknown", doc(1, rec("mama", "root", "Mama")), "ProfilesFileRoleInvalid", "Open.ProfilesFileRoleInvalid", 0},
		{"display name empty", doc(1, rec("mama", "admin", "")), "ProfilesFileDisplayNameInvalid", "Open.ProfilesFileDisplayNameInvalid", 0},
		{"password hash damaged", `{"format":1,"profiles":[{"username":"mama","display_name":"M","role":"admin","password_hash":"x"}]}`,
			"PasswordHashMalformed", "Open->parseHash.PasswordHashMalformed", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profiles.json")
			if tt.content != "" {
				if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			s, err := Open(context.Background(), path)
			if tt.wantTag != "" {
				wantFailure(t, err, tt.wantTag, tt.route)
				if _, statErr := os.Stat(path); tt.content != "" && statErr != nil {
					t.Errorf("Open must not move or delete the file: %v", statErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			if got := len(s.List()); got != tt.count {
				t.Errorf("%d profiles, want %d", got, tt.count)
			}
		})
	}
}

func TestOpenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Open(ctx, filepath.Join(t.TempDir(), "profiles.json"))
	wantFailure(t, err, "ProfilesOpenCancelled", "Open.ProfilesOpenCancelled")
}

func TestOpenReadFailure(t *testing.T) {
	dir := t.TempDir() // a folder where the file should be
	_, err := Open(context.Background(), dir)
	wantFailure(t, err, "JSONReadFailed", "Open->ReadJSON.JSONReadFailed")
}

func TestWriteFailureKeepsState(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "data", "profiles.json")
	s, err := Open(ctx, path, WithIterations(2))
	if err != nil {
		t.Fatal(err)
	}
	mustCreate(t, s, "mama", RoleAdmin, "12345678")
	mustCreate(t, s, "kid", RoleStudent, "1234")

	// Make the folder read-only so the atomic write cannot create its temporary file.
	if err := os.Chmod(filepath.Dir(path), 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(path), 0o700) })
	if f, err := os.CreateTemp(filepath.Dir(path), "probe"); err == nil {
		_ = f.Close()
		t.Skip("the folder is writable despite chmod (running as root?)")
	}

	_, err = s.Create(ctx, "new", "New", RoleStudent, "1234")
	wantFailure(t, err, "TempFileCreateFailed", "Store.Create->Store.commit->WriteJSONAtomic")
	wantFailure(t, s.Rename(ctx, "kid", "Other"), "TempFileCreateFailed", "Store.Rename->Store.commit")
	wantFailure(t, s.SetPassword(ctx, "kid", "abcd"), "TempFileCreateFailed", "Store.SetPassword->Store.commit")
	wantFailure(t, s.Delete(ctx, "kid"), "TempFileCreateFailed", "Store.Delete->Store.commit")

	if got := s.List(); len(got) != 2 || got[1].DisplayName != "Display kid" {
		t.Errorf("failed writes changed the store: %+v", got)
	}
	if _, err := s.Login("kid", "1234"); err != nil {
		t.Errorf("failed SetPassword changed the password: %v", err)
	}
}

func TestConcurrentUse(t *testing.T) {
	ctx := context.Background()
	s, path := openStore(t, newClock())
	mustCreate(t, s, "mama", RoleAdmin, "12345678")

	const n = 8
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("kid%d", i)
			if _, err := s.Create(ctx, name, name, RoleStudent, "1234"); err != nil {
				t.Errorf("Create %s: %v", name, err)
				return
			}
			if _, err := s.Login(name, "1234"); err != nil {
				t.Errorf("Login %s: %v", name, err)
			}
			_ = s.List()
			if err := s.Rename(ctx, name, name+"!"); err != nil {
				t.Errorf("Rename %s: %v", name, err)
			}
			if err := s.SetPassword(ctx, name, "abcd"); err != nil {
				t.Errorf("SetPassword %s: %v", name, err)
			}
			_, _ = s.Login(name, "wrong")
		}()
	}
	wg.Wait()
	if got := len(s.List()); got != n+1 {
		t.Errorf("%d profiles, want %d", got, n+1)
	}
	if got := len(reopen(t, path, newClock()).List()); got != n+1 {
		t.Errorf("%d profiles on disk, want %d", got, n+1)
	}
}

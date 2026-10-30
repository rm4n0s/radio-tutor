package providers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/rm4n0s/errors"

	"github.com/rm4n0s/radio-tutor/internal/core/storage"
	"github.com/rm4n0s/radio-tutor/internal/core/storage/storagetest"
)

// writeFile writes content with the mode.
func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil { // WriteFile is subject to the umask
		t.Fatal(err)
	}
}

// openStore opens a store on a file in a folder of the isolated config.
func openStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := storagetest.Isolate(t).ProvidersFile()
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return s, path
}

func names(list []Config) []string {
	out := []string{}
	for _, c := range list {
		out = append(out, c.Name)
	}
	return out
}

func TestOpen(t *testing.T) {
	good := `{"format":1,"active":"B","providers":[
	  {"name":"A","protocol":"openai","base_url":"http://localhost:11434/v1","model":"m","api_key":"","kind":"local","timeout_seconds":30,"structured_output":"none"},
	  {"name":"B","base_url":"https://api.openai.com/v1","model":"gpt-4o","api_key":"sk-1","kind":"cloud"}]}`
	tests := []struct {
		name  string
		file  string // content, or "-" for no file
		mode  os.FileMode
		route string
		check func(*testing.T, *Store)
	}{
		{"missing file is empty", "-", 0, "", func(t *testing.T, s *Store) {
			if len(s.List()) != 0 {
				t.Error("list not empty")
			}
			if _, ok := s.Active(); ok {
				t.Error("an empty store has no active provider")
			}
		}},
		{"two providers, defaults filled", good, 0o600, "", func(t *testing.T, s *Store) {
			if got := names(s.List()); !reflect.DeepEqual(got, []string{"A", "B"}) {
				t.Errorf("names = %v", got)
			}
			b, ok := s.Active()
			if !ok || b.Name != "B" || b.TimeoutSeconds != 180 || b.StructuredOutput != "json_schema" || b.Protocol != "openai" {
				t.Errorf("active = %+v, %v", b, ok)
			}
		}},
		{"owner-only mode 0400 is fine", good, 0o400, "", nil},
		{"group readable", good, 0o640, "Open->checkPrivate.ProvidersFileTooOpen", nil},
		{"world readable", good, 0o644, "Open->checkPrivate.ProvidersFileTooOpen", nil},
		{"group writable", good, 0o620, "Open->checkPrivate.ProvidersFileTooOpen", nil},
		{"world executable", good, 0o601, "Open->checkPrivate.ProvidersFileTooOpen", nil},
		{"not json", `{nope`, 0o600, "Open->ReadJSON.JSONInvalid", nil},
		{"format 2", `{"format":2}`, 0o600, "Open->check.ProvidersFormatUnsupported", nil},
		{"no format", `{"providers":[]}`, 0o600, "Open->check.ProvidersFormatUnsupported", nil},
		{"duplicate names", `{"format":1,"providers":[
		  {"name":"A","base_url":"http://h/v1","model":"m","kind":"local"},
		  {"name":"A","base_url":"http://h/v1","model":"m","kind":"local"}]}`, 0o600, "Open->check.ProviderNameDuplicate", nil},
		{"unknown active", `{"format":1,"active":"X","providers":[]}`, 0o600, "Open->check.ProvidersActiveUnknown", nil},
		{"provider without kind", `{"format":1,"providers":[{"name":"A","base_url":"http://h/v1","model":"m"}]}`, 0o600,
			"Open->check->validate->normalize.ProviderKindInvalid", nil},
		{"provider with protocol grpc", `{"format":1,"providers":[{"name":"A","protocol":"grpc","base_url":"http://h/v1","model":"m","kind":"local"}]}`, 0o600,
			"ProviderProtocolUnsupported", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "providers.json")
			if tt.file != "-" {
				writeFile(t, path, tt.file, tt.mode)
			}
			s, err := Open(context.Background(), path)
			if tt.route != "" {
				meta := wantRoute(t, err, tt.route)
				if s != nil {
					t.Error("a store came back with an error")
				}
				if strings.Contains(tt.route, "TooOpen") {
					appErr, _ := errors.FromError(err)
					if !strings.Contains(appErr.Message, "chmod 600 "+path) {
						t.Errorf("message %q does not tell how to fix it", appErr.Message)
					}
					if meta["path"] != path {
						t.Errorf("path metadata = %v", meta["path"])
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.check != nil {
				tt.check(t, s)
			}
		})
	}
}

func TestOpenFailures(t *testing.T) {
	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := Open(ctx, filepath.Join(t.TempDir(), "providers.json"))
		wantRoute(t, err, "Open.ProvidersOpenCancelled")
	})
	t.Run("stat fails", func(t *testing.T) {
		dir := t.TempDir()
		blocker := filepath.Join(dir, "file")
		writeFile(t, blocker, "x", 0o600)
		// A path below a regular file cannot be inspected (ENOTDIR).
		_, err := Open(context.Background(), filepath.Join(blocker, "providers.json"))
		wantRoute(t, err, "Open.ProvidersStatFailed")
	})
}

func TestStoreCRUDAndActive(t *testing.T) {
	ctx := context.Background()
	s, path := openStore(t)

	a, b := validConfig("A"), validConfig("B")
	b.Kind, b.APIKey, b.StructuredOutput = KindCloud, "sk-b", "json_object"
	for _, c := range []Config{a, b} {
		if err := s.Add(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	if got := names(s.List()); !reflect.DeepEqual(got, []string{"A", "B"}) {
		t.Fatalf("names = %v", got)
	}
	if _, ok := s.Active(); ok || s.ActiveName() != "" {
		t.Error("Add must not activate anything")
	}
	if got, ok := s.Get("B"); !ok || !reflect.DeepEqual(got, b) {
		t.Errorf("Get(B) = %+v, %v; want %+v", got, ok, b)
	}
	if _, ok := s.Get("Z"); ok {
		t.Error("Get(Z) found a provider")
	}

	if err := s.SetActive(ctx, "A"); err != nil {
		t.Fatal(err)
	}
	if got, ok := s.Active(); !ok || got.Name != "A" {
		t.Errorf("active = %+v, %v", got, ok)
	}

	// Update keeps the active pointer when the name changes.
	a2 := a
	a2.Name, a2.Model = "A renamed", "other-model"
	if err := s.Update(ctx, "A", a2); err != nil {
		t.Fatal(err)
	}
	if got, ok := s.Active(); !ok || got.Name != "A renamed" || got.Model != "other-model" {
		t.Errorf("active after rename = %+v, %v", got, ok)
	}
	if _, ok := s.Get("A"); ok {
		t.Error("the old name still exists")
	}
	if got := names(s.List()); !reflect.DeepEqual(got, []string{"A renamed", "B"}) {
		t.Errorf("names = %v (order must be kept)", got)
	}
	// Update of another provider leaves the active one alone; same name is fine.
	b2 := b
	b2.Model = "bm"
	if err := s.Update(ctx, "B", b2); err != nil {
		t.Fatal(err)
	}
	if s.ActiveName() != "A renamed" {
		t.Errorf("active = %q", s.ActiveName())
	}

	// A new Store on the same file sees the same state.
	again, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again.List(), s.List()) || again.ActiveName() != "A renamed" {
		t.Errorf("reopened = %+v active %q", again.List(), again.ActiveName())
	}

	// Delete of the active provider clears it; deleting another keeps it.
	if err := s.Delete(ctx, "B"); err != nil {
		t.Fatal(err)
	}
	if s.ActiveName() != "A renamed" || len(s.List()) != 1 {
		t.Errorf("after deleting B: %v active %q", names(s.List()), s.ActiveName())
	}
	if err := s.Delete(ctx, "A renamed"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Active(); ok || s.ActiveName() != "" || len(s.List()) != 0 {
		t.Errorf("after deleting the active provider: %v active %q", names(s.List()), s.ActiveName())
	}
	again, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.List()) != 0 || again.ActiveName() != "" {
		t.Errorf("reopened after deletes: %v", again.List())
	}
}

func TestStoreErrors(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name  string
		do    func(*Store) error
		route string
	}{
		{"add duplicate", func(s *Store) error { return s.Add(ctx, validConfig("A")) }, "Add.ProviderNameDuplicate"},
		{"add duplicate after trimming", func(s *Store) error { return s.Add(ctx, validConfig(" A ")) }, "Add.ProviderNameDuplicate"},
		{"add invalid kind", func(s *Store) error { c := validConfig("C"); c.Kind = ""; return s.Add(ctx, c) }, "Add->validate->normalize.ProviderKindInvalid"},
		{"add invalid url", func(s *Store) error { c := validConfig("C"); c.BaseURL = "nope"; return s.Add(ctx, c) }, "Add->validate->New.OpenAIConfigBaseURL"},
		{"update unknown", func(s *Store) error { return s.Update(ctx, "Z", validConfig("Z")) }, "Update.ProviderNotFound"},
		{"update rename onto another", func(s *Store) error { return s.Update(ctx, "A", validConfig("B")) }, "Update.ProviderNameDuplicate"},
		{"update invalid", func(s *Store) error { c := validConfig("A"); c.Model = ""; return s.Update(ctx, "A", c) }, "Update->validate->New.OpenAIConfigModel"},
		{"delete unknown", func(s *Store) error { return s.Delete(ctx, "Z") }, "Delete.ProviderNotFound"},
		{"activate unknown", func(s *Store) error { return s.SetActive(ctx, "Z") }, "SetActive.ProviderNotFound"},
		{"add with a cancelled context", func(s *Store) error {
			c, cancel := context.WithCancel(ctx)
			cancel()
			return s.Add(c, validConfig("C"))
		}, "Store.commit->WritePrivateJSONAtomic->writeJSONAtomic.WriteCancelled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, path := openStore(t)
			for _, n := range []string{"A", "B"} {
				if err := s.Add(ctx, validConfig(n)); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.SetActive(ctx, "A"); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			wantRoute(t, tt.do(s), tt.route)
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Error("a failed change altered the file")
			}
			if got := names(s.List()); !reflect.DeepEqual(got, []string{"A", "B"}) || s.ActiveName() != "A" {
				t.Errorf("a failed change altered the list: %v active %q", got, s.ActiveName())
			}
		})
	}
}

// A write that fails (the folder cannot be made) leaves the list as it was.
func TestStoreWriteFailureKeepsState(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	writeFile(t, blocker, "x", 0o600)
	s, err := Open(ctx, filepath.Join(dir, "sub", "providers.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "sub"), "x", 0o600) // "sub" is a file: its folder cannot be created
	err = s.Add(ctx, validConfig("A"))
	wantRoute(t, err, "Store.Add->Store.commit->WritePrivateJSONAtomic->writeJSONAtomic->EnsureDir.DirCreateFailed")
	if len(s.List()) != 0 {
		t.Error("the list changed although the write failed")
	}
}

func TestStoreWritesPrivateFile(t *testing.T) {
	ctx := context.Background()
	s, path := openStore(t)
	if err := s.Add(ctx, validConfig("A")); err != nil {
		t.Fatal(err)
	}
	if err := s.SetActive(ctx, "A"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("providers.json mode = %v, want 0600", info.Mode().Perm())
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != storage.DirMode {
		t.Errorf("config folder mode = %v, want %v", dirInfo.Mode().Perm(), os.FileMode(storage.DirMode))
	}
	var f struct {
		Format    int
		Active    string
		Providers []map[string]any
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	if f.Format != 1 || f.Active != "A" || len(f.Providers) != 1 || f.Providers[0]["api_key"] != "sk-secret-123" {
		t.Errorf("file content = %s", data)
	}
	// An empty list is written as [], not null.
	if err := s.Delete(ctx, "A"); err != nil {
		t.Fatal(err)
	}
	if data, err = os.ReadFile(path); err != nil || !strings.Contains(string(data), `"providers": []`) {
		t.Errorf("empty list file = %s (%v)", data, err)
	}
}

func TestStoreReturnsCopies(t *testing.T) {
	ctx := context.Background()
	s, _ := openStore(t)
	if err := s.Add(ctx, validConfig("A")); err != nil {
		t.Fatal(err)
	}
	list := s.List()
	list[0].Model = "tampered"
	if got, _ := s.Get("A"); got.Model == "tampered" {
		t.Error("List shares its slice with the store")
	}
}

func TestStoreConcurrent(t *testing.T) {
	ctx := context.Background()
	s, path := openStore(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name := string(rune('a' + i))
			if err := s.Add(ctx, validConfig(name)); err != nil {
				t.Error(err)
				return
			}
			if err := s.SetActive(ctx, name); err != nil {
				t.Error(err)
			}
			_ = s.List()
			_, _ = s.Active()
			_, _ = s.Get(name)
		}()
	}
	wg.Wait()
	if len(s.List()) != 8 {
		t.Errorf("%d providers, want 8", len(s.List()))
	}
	again, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.List()) != 8 || again.ActiveName() != s.ActiveName() {
		t.Errorf("file holds %d providers, active %q vs %q", len(again.List()), again.ActiveName(), s.ActiveName())
	}
}

package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// useMockKeychain swaps the system keychain for an in-memory one
func useMockKeychain(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	resetVault(t)
}

func resetVault(t *testing.T) {
	t.Helper()
	vaultMu.Lock()
	vaultKey = nil
	vaultMu.Unlock()
	dir := t.TempDir()
	vaultLockPath = func() (string, error) { return filepath.Join(dir, ".lock"), nil }
}

func TestSealBindsHostAndField(t *testing.T) {
	useMockKeychain(t)
	blob, err := seal([]byte("pw"), "h1", "password")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := unseal(blob, "h1", "password"); err != nil || string(got) != "pw" {
		t.Fatalf("unseal = %q, %v", got, err)
	}
	if _, err := unseal(blob, "h2", "password"); !errors.Is(err, ErrSealBroken) {
		t.Errorf("opened under another host: %v", err)
	}
	if _, err := unseal(blob, "h1", "key"); !errors.Is(err, ErrSealBroken) {
		t.Errorf("opened as another field: %v", err)
	}
	// a fresh process reads the same key back from the keychain
	vaultKey = nil
	if got, err := unseal(blob, "h1", "password"); err != nil || string(got) != "pw" {
		t.Errorf("after reload unseal = %q, %v", got, err)
	}
}

func TestSavedPasswordIsSealed(t *testing.T) {
	useMockKeychain(t)
	s := &Store{Dir: t.TempDir()}
	h, err := s.Add(Host{Host: "h", User: "u", PasswordMode: PasswordSaved, Password: " secret "})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(s.Dir, h.ID+".json"))
	if strings.Contains(string(b), "secret") || !strings.Contains(string(b), `"password_sealed": "v1:`) {
		t.Fatalf("host file:\n%s", b)
	}
	got, _ := s.Find(h.ID)
	if got.Password != "" {
		t.Error("Find decrypted the password")
	}
	if err := Unlock(got); err != nil || got.Password != " secret " {
		t.Errorf("Unlock = %q, %v", got.Password, err)
	}
	// editing other fields without typing the password keeps it
	in := Host{Name: "n2", Host: "h", User: "u", PasswordMode: PasswordSaved}
	if _, err := s.Update(h.ID, in); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Find(h.ID)
	if err := Unlock(got); err != nil || got.Password != " secret " {
		t.Errorf("after edit Unlock = %q, %v", got.Password, err)
	}
	if _, err := s.Add(Host{Host: "h", User: "u", PasswordMode: PasswordSaved}); !errors.Is(err, ErrPasswordRequired) {
		t.Errorf("saved mode with no password: %v", err)
	}
}

func TestPlainPasswordMigrates(t *testing.T) {
	useMockKeychain(t)
	s := &Store{Dir: t.TempDir()}
	old := `{"id":"abc","name":"old","host":"h","port":22,"user":"u","password":"pw1"}`
	os.WriteFile(filepath.Join(s.Dir, "abc.json"), []byte(old), 0o600)
	h, err := s.Find("old")
	if err != nil || h.PasswordMode != PasswordSaved {
		t.Fatalf("old host = %+v, %v", h, err)
	}
	if err := Unlock(h); err != nil || h.Password != "pw1" {
		t.Fatalf("Unlock old = %q, %v", h.Password, err)
	}
	h.Password = ""
	if _, err := s.Update(h.ID, *h); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(s.Dir, "abc.json"))
	if strings.Contains(string(b), "pw1") || !strings.Contains(string(b), "password_sealed") {
		t.Errorf("not migrated:\n%s", b)
	}
}

func TestNoKeychain(t *testing.T) {
	keyring.MockInitWithError(errors.New("no bus"))
	resetVault(t)
	t.Cleanup(keyring.MockInit)
	s := &Store{Dir: t.TempDir()}
	if _, err := s.Add(Host{Host: "h", User: "u", PasswordMode: PasswordSaved, Password: "x"}); !errors.Is(err, ErrNoKeychain) {
		t.Errorf("saved without a keychain: %v", err)
	}
	if _, err := s.Add(Host{Host: "h", User: "u", PasswordMode: PasswordAsk}); err != nil {
		t.Errorf("ask mode needs no keychain: %v", err)
	}
	// an old plain password keeps working and stays until the keychain is back
	old := `{"id":"abc","name":"old","host":"h","port":22,"user":"u","password":"pw1"}`
	os.WriteFile(filepath.Join(s.Dir, "abc.json"), []byte(old), 0o600)
	h, _ := s.Find("old")
	if _, err := s.Update(h.ID, *h); err != nil {
		t.Fatalf("edit of an old host without a keychain: %v", err)
	}
	h, _ = s.Find("old")
	if err := Unlock(h); err != nil || h.Password != "pw1" {
		t.Errorf("old password lost: %q, %v", h.Password, err)
	}
}

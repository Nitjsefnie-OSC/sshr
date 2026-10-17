// Package app holds local vault storage and app settings (no UI).
package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrHostEmpty    = errors.New("host_empty")
	ErrUserRequired = errors.New("user_required")
	ErrNotFound     = errors.New("not_found")
	ErrPortInvalid  = errors.New("port_invalid")
	// ErrPasswordRequired means PasswordSaved was chosen with no password
	ErrPasswordRequired = errors.New("password_required")
	// ErrKeyRequired means KeyVault was chosen with no key pasted
	ErrKeyRequired = errors.New("key_required")
)

// Password modes of a host, an empty mode means no password at all, the
// key file, ssh-agent or ssh config decide
const (
	// PasswordSaved keeps the password sealed with the vault key in the
	// host file, sshr hands it to ssh
	PasswordSaved = "saved"
	// PasswordAsk stores nothing, ssh asks on the terminal every time
	PasswordAsk = "ask"
)

// ParsePort reads a TCP port: digits only, 1–65535.
func ParsePort(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 || n > 65535 {
		return 0, ErrPortInvalid
	}
	return n, nil
}

// AmbiguousError means a query matched several hosts; Hosts lists them so the
// caller can ask for an exact ID instead of guessing.
type AmbiguousError struct {
	Query string
	Hosts []Host
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("%q matches %d hosts", e.Query, len(e.Hosts))
}

// Host is one SSH target
type Host struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
	User string `json:"user"`
	Key  string `json:"key,omitempty"`
	// KeyMode is KeyVault for a private key stored inside sshr, empty for
	// a key file in Key or no key
	KeyMode string `json:"key_mode,omitempty"`
	// KeyData is a pasted private key, in memory only, set it to save a
	// new one
	KeyData []byte `json:"-"`
	// KeyPassphrase opens a passphrase-protected KeyData, never stored
	KeyPassphrase string `json:"-"`
	// KeySealed is the private key encrypted with the vault key
	KeySealed string `json:"key_sealed,omitempty"`
	// KeyPub is the public half of the stored key, authorized_keys format
	KeyPub string `json:"key_pub,omitempty"`
	// PasswordMode is PasswordSaved, PasswordAsk or empty
	PasswordMode string `json:"password_mode,omitempty"`
	// Password is the plain password, in memory only, set it to save a new
	// one, Unlock fills it from PasswordSealed before connecting
	Password string `json:"-"`
	// PasswordSealed is the password encrypted with the vault key
	PasswordSealed string `json:"password_sealed,omitempty"`
	// PlainPassword is the unencrypted "password" field older versions
	// wrote, it is sealed and dropped on the next save that can reach the
	// keychain
	PlainPassword string `json:"password,omitempty"`
	// Jump lists hosts to go through, "a,b": saved host names/IDs or
	// [user@]host[:port], empty means a direct connection
	Jump string `json:"jump,omitempty"`
	// Session is "tmux" or "zellij" to attach to a remote session named
	// SessionName on connect, empty means a plain shell
	Session string `json:"session,omitempty"`
	// Forwards are named port forwards, started with connect -F or as a
	// tunnel with sshr tunnel
	Forwards  []Forward `json:"forwards,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Store persists hosts as ~/.sshr/vault/hosts/<id>.json (0600).
type Store struct {
	Dir string
	// Reuse overrides the "reuse" setting for this run, nil means the
	// setting decides
	Reuse *bool
}

// DefaultStore creates ~/.sshr/vault/hosts if needed.
func DefaultStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".sshr", "vault", "hosts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{Dir: dir}, nil
}

// Add saves a new host, ID and CreatedAt are set here
func (s *Store) Add(in Host) (*Host, error) {
	h, err := cleanHost(in)
	if err != nil {
		return nil, err
	}
	if err := needSecret(&h); err != nil {
		return nil, err
	}
	if h.ID, err = newID(); err != nil {
		return nil, err
	}
	h.CreatedAt = time.Now().UTC()
	if err := s.save(&h); err != nil {
		return nil, err
	}
	return &h, nil
}

// cleanHost trims and checks the fields of a host before it is saved
func cleanHost(h Host) (Host, error) {
	h.Name = strings.TrimSpace(h.Name)
	h.Host = strings.TrimSpace(h.Host)
	h.User = strings.TrimSpace(h.User)
	if h.Host == "" {
		return h, ErrHostEmpty
	}
	if strings.HasPrefix(h.Host, "-") {
		return h, fmt.Errorf("host must not start with '-': %s", h.Host)
	}
	if h.User == "" {
		return h, ErrUserRequired
	}
	if h.Name == "" {
		h.Name = h.Host
	}
	if h.Port == 0 {
		h.Port = 22
	}
	if h.Port < 1 || h.Port > 65535 {
		return h, ErrPortInvalid
	}
	h.Key = strings.TrimSpace(h.Key)
	// a password is kept as typed, spaces at the ends can be part of it
	if h.Password != "" && h.PasswordMode == "" {
		h.PasswordMode = PasswordSaved
	}
	switch h.PasswordMode {
	case "", PasswordAsk, PasswordSaved:
	default:
		return h, fmt.Errorf("unknown password mode %q", h.PasswordMode)
	}
	if len(h.KeyData) > 0 {
		h.KeyMode = KeyVault
	}
	switch h.KeyMode {
	case "":
		h.KeyData, h.KeyPassphrase, h.KeySealed, h.KeyPub = nil, "", "", ""
	case KeyVault:
		h.Key = ""
		if len(h.KeyData) > 0 {
			if err := prepareVaultKey(&h); err != nil {
				return h, err
			}
		}
	default:
		return h, fmt.Errorf("unknown key mode %q", h.KeyMode)
	}
	if h.Key != "" || h.KeyMode == KeyVault {
		h.PasswordMode = ""
	}
	if h.PasswordMode != PasswordSaved {
		h.Password, h.PasswordSealed, h.PlainPassword = "", "", ""
	}
	var err error
	if h.Jump, err = checkJump(h.Jump, h.Name); err != nil {
		return h, err
	}
	h.Session = strings.TrimSpace(h.Session)
	if err := checkSession(h.Session); err != nil {
		return h, err
	}
	if err := checkForwards(h.Forwards); err != nil {
		return h, err
	}
	return h, nil
}

func (s *Store) List() ([]Host, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]Host, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.Dir, e.Name()))
		if err != nil {
			continue
		}
		var h Host
		if err := json.Unmarshal(b, &h); err != nil {
			continue
		}
		// The file name is the source of truth: Delete builds its path from ID.
		h.ID = strings.TrimSuffix(e.Name(), ".json")
		if h.PlainPassword != "" && h.PasswordMode == "" && h.Key == "" {
			h.PasswordMode = PasswordSaved
		}
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// Find resolves a query by exact ID, then name, then host address, then ID
// prefix. A tier with several matches is an error: picking one would let rm or
// connect hit the wrong server.
func (s *Store) Find(query string) (*Host, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrNotFound
	}
	hosts, err := s.List()
	if err != nil {
		return nil, err
	}
	tiers := []func(h Host) bool{
		func(h Host) bool { return h.ID == query },
		func(h Host) bool { return strings.EqualFold(h.Name, query) },
		func(h Host) bool { return strings.EqualFold(h.Host, query) },
		func(h Host) bool { return strings.HasPrefix(h.ID, query) },
	}
	for _, match := range tiers {
		var found []Host
		for _, h := range hosts {
			if match(h) {
				found = append(found, h)
			}
		}
		switch len(found) {
		case 0:
			continue
		case 1:
			return &found[0], nil
		default:
			return nil, &AmbiguousError{Query: query, Hosts: found}
		}
	}
	return nil, ErrNotFound
}

func (s *Store) Delete(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrNotFound
	}
	path := filepath.Join(s.Dir, id+".json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	os.Remove(filepath.Join(s.Dir, id+".pub"))
	return nil
}

// Update rewrites an existing host with the fields of in, ID and
// CreatedAt are kept
func (s *Store) Update(id string, in Host) (*Host, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrNotFound
	}
	cur, err := s.Find(id)
	if err != nil {
		return nil, err
	}
	h, err := cleanHost(in)
	if err != nil {
		return nil, err
	}
	// no new password typed keeps the saved one
	if h.PasswordMode == PasswordSaved && h.Password == "" && h.PasswordSealed == "" && h.PlainPassword == "" {
		h.PasswordSealed, h.PlainPassword = cur.PasswordSealed, cur.PlainPassword
	}
	// no new key pasted keeps the stored one
	if h.KeyMode == KeyVault && len(h.KeyData) == 0 && h.KeySealed == "" {
		h.KeySealed, h.KeyPub = cur.KeySealed, cur.KeyPub
	}
	if err := needSecret(&h); err != nil {
		return nil, err
	}
	h.ID, h.CreatedAt = cur.ID, cur.CreatedAt
	if err := s.save(&h); err != nil {
		return nil, err
	}
	return &h, nil
}

// checkJump normalizes a jump spec and refuses one that names the host
// itself, which would loop
func checkJump(jump, name string) (string, error) {
	jump, err := normalizeJump(jump)
	if err != nil {
		return "", err
	}
	for _, p := range SplitJump(jump) {
		if strings.EqualFold(p, name) {
			return "", fmt.Errorf("host %q cannot jump through itself", name)
		}
		// A saved host name parses as a plain host too, so this only rejects
		// specs that can never work, like a bad port
		if _, err := ParseHop(p); err != nil {
			return "", err
		}
	}
	return jump, nil
}

// needSecret refuses PasswordSaved with no password and KeyVault with no
// key to save
func needSecret(h *Host) error {
	if h.PasswordMode == PasswordSaved && h.Password == "" && h.PasswordSealed == "" && h.PlainPassword == "" {
		return ErrPasswordRequired
	}
	if h.KeyMode == KeyVault && len(h.KeyData) == 0 && h.KeySealed == "" {
		return ErrKeyRequired
	}
	return nil
}

// sealKey encrypts a newly pasted private key before the host is written,
// with no keychain the host is not saved, the key is never written
// unencrypted
func sealKey(h *Host) error {
	if h.KeyMode != KeyVault || len(h.KeyData) == 0 {
		return nil
	}
	sealed, err := seal(h.KeyData, h.ID, "key")
	if err != nil {
		return err
	}
	h.KeySealed = sealed
	return nil
}

// sealPassword encrypts a new password, or an old unencrypted one, before
// the host is written, an old one stays as it was when the keychain is out
// of reach, a new one is refused then, it is never written unencrypted
func sealPassword(h *Host) error {
	if h.PasswordMode != PasswordSaved {
		return nil
	}
	plain := h.Password
	if plain == "" {
		plain = h.PlainPassword
	}
	if plain == "" {
		return nil
	}
	sealed, err := seal([]byte(plain), h.ID, "password")
	if err != nil {
		if h.Password == "" || h.Password == h.PlainPassword {
			return nil
		}
		return err
	}
	h.PasswordSealed, h.PlainPassword = sealed, ""
	return nil
}

// Unlock fills h.Password from the vault before connecting, a host without
// a saved password is left as it is
func Unlock(h *Host) error {
	if h.PasswordMode != PasswordSaved || h.Password != "" {
		return nil
	}
	switch {
	case h.PasswordSealed != "":
		plain, err := unseal(h.PasswordSealed, h.ID, "password")
		if errors.Is(err, ErrSealBroken) {
			return fmt.Errorf("the saved password of %s does not open with the vault key of this machine (keychain reset or the host copied from another machine), save the password again", h.Name)
		}
		if err != nil {
			return err
		}
		h.Password = string(plain)
	case h.PlainPassword != "":
		h.Password = h.PlainPassword
	default:
		return fmt.Errorf("%s has no saved password, save it again", h.Name)
	}
	return nil
}

func (s *Store) save(h *Host) error {
	if err := sealPassword(h); err != nil {
		return err
	}
	if err := sealKey(h); err != nil {
		return err
	}
	b, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.Dir, h.ID+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return s.writePub(h)
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

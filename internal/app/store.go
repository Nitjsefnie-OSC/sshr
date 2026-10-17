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

// Host is one SSH target. Password is stored in the local vault (0600);
// prefer a key when you can.
type Host struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Key      string `json:"key,omitempty"`
	Password string `json:"password,omitempty"`
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
	h.Password = strings.TrimSpace(h.Password)
	if h.Key != "" {
		h.Password = ""
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

func (s *Store) save(h *Host) error {
	b, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.Dir, h.ID+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

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
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	User      string    `json:"user"`
	Key       string    `json:"key,omitempty"`
	Password  string    `json:"password,omitempty"`
	// Jump lists hosts to go through, "a,b": saved host names/IDs or
	// [user@]host[:port], empty means a direct connection
	Jump      string    `json:"jump,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Store persists hosts as ~/.sshr/vault/hosts/<id>.json (0600).
type Store struct {
	Dir string
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

func (s *Store) Add(name, host, user, key, password, jump string, port int) (*Host, error) {
	name = strings.TrimSpace(name)
	host = strings.TrimSpace(host)
	user = strings.TrimSpace(user)
	if host == "" {
		return nil, ErrHostEmpty
	}
	if strings.HasPrefix(host, "-") {
		return nil, fmt.Errorf("host must not start with '-': %s", host)
	}
	if user == "" {
		return nil, ErrUserRequired
	}
	if name == "" {
		name = host
	}
	if port == 0 {
		port = 22
	}
	if port < 1 || port > 65535 {
		return nil, ErrPortInvalid
	}

	id, err := newID()
	if err != nil {
		return nil, err
	}
	key = strings.TrimSpace(key)
	password = strings.TrimSpace(password)
	if key != "" {
		password = ""
	}
	jump, err = checkJump(jump, name)
	if err != nil {
		return nil, err
	}
	h := &Host{
		ID:        id,
		Name:      name,
		Host:      host,
		Port:      port,
		User:      user,
		Key:       key,
		Password:  password,
		Jump:      jump,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.save(h); err != nil {
		return nil, err
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

// Update rewrites an existing host; CreatedAt is kept.
func (s *Store) Update(id, name, host, user, key, password, jump string, port int) (*Host, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrNotFound
	}
	cur, err := s.Find(id)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	host = strings.TrimSpace(host)
	user = strings.TrimSpace(user)
	if host == "" {
		return nil, ErrHostEmpty
	}
	if strings.HasPrefix(host, "-") {
		return nil, fmt.Errorf("host must not start with '-': %s", host)
	}
	if user == "" {
		return nil, ErrUserRequired
	}
	if name == "" {
		name = host
	}
	if port == 0 {
		port = 22
	}
	if port < 1 || port > 65535 {
		return nil, ErrPortInvalid
	}
	key = strings.TrimSpace(key)
	password = strings.TrimSpace(password)
	if key != "" {
		password = ""
	}
	jump, err = checkJump(jump, name)
	if err != nil {
		return nil, err
	}
	h := &Host{
		ID:        cur.ID,
		Name:      name,
		Host:      host,
		Port:      port,
		User:      user,
		Key:       key,
		Password:  password,
		Jump:      jump,
		CreatedAt: cur.CreatedAt,
	}
	if err := s.save(h); err != nil {
		return nil, err
	}
	return h, nil
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

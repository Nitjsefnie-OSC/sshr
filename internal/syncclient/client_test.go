package syncclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"sshr.dev/internal/app"
	"sshr.dev/internal/syncserver"
)

type world struct {
	t       *testing.T
	srv     *httptest.Server
	data    string
	a, b    *Client
	log     bytes.Buffer
	counter int
}

func newWorld(t *testing.T) *world {
	keyring.MockInit()
	data := t.TempDir()
	s, err := syncserver.New(data)
	if err != nil {
		t.Fatal(err)
	}
	w := &world{t: t, srv: httptest.NewServer(s), data: data}
	t.Cleanup(w.srv.Close)
	w.a = w.client("a-")
	w.b = w.client("b-")
	return w
}

func (w *world) client(prefix string) *Client {
	return &Client{Store: &app.Store{Dir: w.t.TempDir()}, Dir: w.t.TempDir(), KeyPrefix: prefix, Log: &w.log}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// pair sets up a with two hosts and joins b to it
func (w *world) pair() (web, db *app.Host) {
	t := w.t
	var err error
	web, err = w.a.Store.Add(app.Host{Name: "web", Host: "10.0.0.1", User: "u", PasswordMode: app.PasswordSaved, Password: " pw 1 "})
	must(t, err)
	db, err = w.a.Store.Add(app.Host{Name: "db", Host: "10.0.0.2", User: "u", Jump: "web"})
	must(t, err)
	must(t, w.a.Init(w.srv.URL, "laptop"))
	code, err := w.a.Invite()
	must(t, err)
	fp, err := w.b.Join(w.srv.URL, code, "desktop")
	must(t, err)
	if err := w.b.Pull(); err == nil || !strings.Contains(err.Error(), "waits for approval") {
		t.Fatalf("pull before approval = %v", err)
	}
	pending, err := w.a.Pending()
	must(t, err)
	if len(pending) != 1 || Fingerprint(pending[0].SignPub) != fp {
		t.Fatalf("pending = %+v, fingerprint on b %s", pending, fp)
	}
	_, err = w.a.Approve(pending[0].ID[:6])
	must(t, err)
	must(t, w.b.Pull())
	return web, db
}

func TestSyncTwoDevices(t *testing.T) {
	w := newWorld(t)
	web, _ := w.pair()
	got, err := w.b.Store.Find(web.ID)
	must(t, err)
	if got.Name != "web" {
		t.Fatalf("b has %+v", got)
	}
	must(t, app.Unlock(got))
	if got.Password != " pw 1 " {
		t.Errorf("password on b = %q", got.Password)
	}
	b, _ := os.ReadFile(filepath.Join(w.b.Store.Dir, web.ID+".json"))
	if strings.Contains(string(b), "pw 1") {
		t.Errorf("password in the clear on b:\n%s", b)
	}
	sa, _ := w.a.Status()
	sb, _ := w.b.Status()
	if sa.State != sb.State || sa.State == "" {
		t.Errorf("state fingerprints differ: %s %s", sa.State, sb.State)
	}

	// b edits, a sees it, a deletes, b sees it
	in := *got
	in.Port = 2222
	in.Password = ""
	_, err = w.b.Store.Update(web.ID, in)
	must(t, err)
	must(t, w.b.Push())
	must(t, w.a.Pull())
	if h, _ := w.a.Store.Find(web.ID); h.Port != 2222 {
		t.Errorf("a did not get the edit: %+v", h)
	}
	must(t, w.a.Store.Delete(web.ID))
	must(t, w.a.Push())
	must(t, w.b.Pull())
	if _, err := w.b.Store.Find(web.ID); err == nil {
		t.Error("b kept a host deleted on a")
	}
}

func TestSyncConflict(t *testing.T) {
	w := newWorld(t)
	_, db := w.pair()
	for i, c := range []*Client{w.a, w.b} {
		h, err := c.Store.Find(db.ID)
		must(t, err)
		h.Port = 3000 + i
		_, err = c.Store.Update(db.ID, *h)
		must(t, err)
	}
	must(t, w.a.Push())
	must(t, w.b.Push())
	must(t, w.a.Pull())
	ha, _ := w.a.Store.Find(db.ID)
	hb, _ := w.b.Store.Find(db.ID)
	if ha.Port != hb.Port {
		t.Errorf("devices disagree after a conflict: %d %d", ha.Port, hb.Port)
	}
	if !strings.Contains(w.log.String(), "conflict") {
		t.Errorf("no conflict reported:\n%s", w.log.String())
	}
}

func TestSyncVaultKeyStaysLocal(t *testing.T) {
	w := newWorld(t)
	w.pair()
	// a host with a key stored in sshr on a
	key := testKeyPEM(t)
	h, err := w.a.Store.Add(app.Host{Name: "k", Host: "10.0.0.3", User: "u", KeyData: key})
	must(t, err)
	must(t, w.a.Push())
	must(t, w.b.Pull())
	hb, err := w.b.Store.Find(h.ID)
	must(t, err)
	if hb.KeySealed != "" || hb.KeyPub != h.KeyPub {
		t.Errorf("b got %+v", hb)
	}
	hb.Port = 2200
	_, err = w.b.Store.Update(h.ID, *hb)
	must(t, err)
	must(t, w.b.Push())
	must(t, w.a.Pull())
	ha, _ := w.a.Store.Find(h.ID)
	if ha.Port != 2200 || ha.KeySealed == "" {
		t.Errorf("a lost its key or the edit: port %d sealed %q", ha.Port, ha.KeySealed)
	}
}

// manifestFiles lists the stored manifests of the only account
func (w *world) manifestDir() string {
	accounts, _ := os.ReadDir(filepath.Join(w.data, "accounts"))
	return filepath.Join(w.data, "accounts", accounts[0].Name(), "manifests")
}

func TestSyncDetectsServerTampering(t *testing.T) {
	w := newWorld(t)
	_, db := w.pair()
	dir := w.manifestDir()

	// a manifest changed on the server fails the signature check
	h, _ := w.a.Store.Find(db.ID)
	h.Port = 4000
	w.a.Store.Update(db.ID, *h)
	must(t, w.a.Push())
	n := len(mustRead(t, dir))
	last := filepath.Join(dir, fmt.Sprintf("%d.json", n))
	orig, _ := os.ReadFile(last)
	var sm struct {
		Body   []byte `json:"body"`
		MAC    []byte `json:"mac"`
		Signer string `json:"signer"`
		Sig    []byte `json:"sig"`
	}
	json.Unmarshal(orig, &sm)
	sm.Body = bytes.Replace(sm.Body, []byte(`"version":1`), []byte(`"version":1 `), 1)
	forged, _ := json.Marshal(sm)
	os.WriteFile(last, forged, 0o600)
	if err := w.b.Pull(); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Errorf("forged manifest = %v", err)
	}
	os.WriteFile(last, orig, 0o600)
	must(t, w.b.Pull())

	// a missing change in the history is a gap
	h.Port = 4001
	w.a.Store.Update(db.ID, *h)
	must(t, w.a.Push())
	h.Port = 4002
	w.a.Store.Update(db.ID, *h)
	must(t, w.a.Push())
	n = len(mustRead(t, dir))
	gap := filepath.Join(dir, fmt.Sprintf("%d.json", n-1))
	saved, _ := os.ReadFile(gap)
	os.Remove(gap)
	if err := w.b.Pull(); err == nil || !strings.Contains(err.Error(), "skipped") {
		t.Errorf("gap = %v", err)
	}
	os.WriteFile(gap, saved, 0o600)
	must(t, w.b.Pull())

	// the server going back to an older state is a rollback
	os.Remove(filepath.Join(dir, fmt.Sprintf("%d.json", n)))
	os.Remove(filepath.Join(dir, fmt.Sprintf("%d.json", n-1)))
	if err := w.b.Pull(); err == nil || !strings.Contains(err.Error(), "rolled") {
		t.Errorf("rollback = %v", err)
	}
}

func TestSyncRemoveDevice(t *testing.T) {
	w := newWorld(t)
	w.pair()
	sb, _ := w.b.Status()
	removed, err := w.a.RemoveDevice(sb.Device[:6])
	must(t, err)
	if removed.ID != sb.Device {
		t.Fatalf("removed %s", removed.ID)
	}
	if err := w.b.Pull(); err == nil {
		t.Error("removed device still pulls")
	}
	sa, _ := w.a.Status()
	if len(sa.Devices) != 1 {
		t.Errorf("devices after removal: %d", len(sa.Devices))
	}
	// a third device joins under the new key and gets every host
	c := w.client("c-")
	code, _ := w.a.Invite()
	_, err = c.Join(w.srv.URL, code, "phone")
	must(t, err)
	_, err = w.a.Approve(mustPending(t, w.a))
	must(t, err)
	must(t, c.Pull())
	hosts, _ := c.Store.List()
	if len(hosts) != 2 {
		t.Errorf("new device after rotation has %d hosts", len(hosts))
	}
}

func mustRead(t *testing.T, dir string) []os.DirEntry {
	es, err := os.ReadDir(dir)
	must(t, err)
	return es
}

func mustPending(t *testing.T, c *Client) string {
	p, err := c.Pending()
	must(t, err)
	if len(p) != 1 {
		t.Fatalf("pending = %d", len(p))
	}
	return p[0].ID
}

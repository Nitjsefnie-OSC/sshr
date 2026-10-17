//go:build !windows

package app

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// testAgent serves two keys, "work" and "home", on a socket in a temp dir
// with a space in its path, like the 1Password socket on macOS
func testAgent(t *testing.T) (sock string, pubs map[string]string) {
	t.Helper()
	// a socket path has room for about 104 bytes, the macOS temp dir under
	// /var/folders with the test name takes most of it
	base, err := os.MkdirTemp("/tmp", "sshr")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(base) })
	dir := filepath.Join(base, "Group Containers")
	os.MkdirAll(dir, 0o700)
	sock = filepath.Join(dir, "agent.sock")
	keys := agent.NewKeyring()
	pubs = map[string]string{}
	for _, name := range []string{"work", "home"} {
		_, priv, _ := ed25519.GenerateKey(rand.Reader)
		keys.Add(agent.AddedKey{PrivateKey: priv, Comment: name})
		signer, _ := ssh.NewSignerFromKey(priv)
		pubs[name] = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	}
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); agent.ServeAgent(keys, c) }()
		}
	}()
	return sock, pubs
}

func TestAgentHostPicksKey(t *testing.T) {
	sock, pubs := testAgent(t)
	s := &Store{Dir: t.TempDir()}
	h, err := s.Add(Host{Host: "h", User: "u", KeyMode: KeyAgent, Agent: sock, AgentKey: "work"})
	if err != nil {
		t.Fatal(err)
	}
	if h.KeyPub != pubs["work"] {
		t.Fatalf("KeyPub = %q", h.KeyPub)
	}
	args := strings.Join(s.agentArgs(h), " ")
	if !strings.Contains(args, `IdentityAgent="`+sock+`"`) || !strings.Contains(args, "IdentitiesOnly=yes") {
		t.Errorf("agentArgs = %q", args)
	}
	if p, _ := os.ReadFile(s.pubPath(h)); strings.TrimSpace(string(p)) != pubs["work"] {
		t.Errorf("pub file = %q", p)
	}
	b, _ := os.ReadFile(filepath.Join(s.Dir, h.ID+".json"))
	if !strings.Contains(string(b), `"key_mode": "agent"`) {
		t.Errorf("host file:\n%s", b)
	}
	if _, err := s.Add(Host{Host: "h", User: "u", KeyMode: KeyAgent, Agent: sock, AgentKey: "nope"}); err == nil {
		t.Error("unknown agent key accepted")
	}
	if _, err := s.Add(Host{Host: "h", User: "u", KeyMode: KeyAgent, Agent: sock, AgentKey: "o"}); err == nil {
		t.Error("ambiguous agent key accepted")
	}
	if _, err := s.Add(Host{Host: "h", User: "u", KeyMode: KeyAgent, Agent: "relative.sock"}); err == nil {
		t.Error("relative agent path accepted")
	}
}

func TestAgentHostSystem(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	h, err := s.Add(Host{Host: "h", User: "u", KeyMode: KeyAgent, Password: "x"})
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(s.agentArgs(h), " ")
	if args != "-o IdentityAgent=SSH_AUTH_SOCK" || h.PasswordMode != "" {
		t.Errorf("agentArgs = %q, host = %+v", args, h)
	}
	if got := agentSocket(AgentOnePassword); !strings.Contains(got, "agent.sock") {
		t.Errorf("1password socket = %q", got)
	}
}

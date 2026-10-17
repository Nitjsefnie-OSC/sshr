//go:build !windows

package app

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
)

// askpass runs the script AttachPasswordAuth writes, in a new session so it
// has no terminal to fall back to
func askpass(t *testing.T, h *Host, prompt string) (string, error) {
	t.Helper()
	cmd := exec.Command("true")
	cleanup := AttachPasswordAuth(cmd, h)
	defer cleanup()
	script := ""
	for _, kv := range cmd.Env {
		if v, ok := strings.CutPrefix(kv, "SSH_ASKPASS="); ok {
			script = v
		}
	}
	if script == "" {
		t.Fatal("no SSH_ASKPASS set")
	}
	run := exec.Command(script, prompt)
	run.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	run.Stdin, _ = os.Open(os.DevNull)
	out, err := run.Output()
	return string(out), err
}

func TestAskpassAnswersOnlyPasswordPrompts(t *testing.T) {
	h := &Host{Password: "p w'x"}
	for _, p := range []string{"u@h's password: ", "(u@h) Password: ", "Password:"} {
		out, err := askpass(t, h, p)
		if err != nil || out != "p w'x\n" {
			t.Errorf("prompt %q: got %q, %v", p, out, err)
		}
	}
	for _, p := range []string{
		"Are you sure you want to continue connecting (yes/no/[fingerprint])? ",
		"Enter passphrase for key '/k': ",
		"Verification code: ",
	} {
		out, err := askpass(t, h, p)
		if err == nil || strings.Contains(out, "p w'x") {
			t.Errorf("prompt %q answered without a terminal: %q, %v", p, out, err)
		}
	}
}

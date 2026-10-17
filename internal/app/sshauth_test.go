package app

import (
	"os/exec"
	"strings"
	"testing"
)

func TestPasswordPrompt(t *testing.T) {
	for _, p := range []string{"u@h's password: ", "(u@h) Password: ", "Password:"} {
		if !isPasswordPrompt(p) {
			t.Errorf("%q not taken for a password prompt", p)
		}
	}
	for _, p := range []string{
		"Are you sure you want to continue connecting (yes/no/[fingerprint])? ",
		"Enter passphrase for key '/k': ",
		"Verification code: ",
		"password: for what? ",
	} {
		if isPasswordPrompt(p) {
			t.Errorf("%q taken for a password prompt", p)
		}
	}
}

func TestPasswordSocketAnswersOnce(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	sock, token, stop, err := servePassword(" p w'x ")
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if _, err := fetchPassword(sock, "wrong"); err == nil {
		t.Error("a wrong token got the password")
	}
	pw, err := fetchPassword(sock, token)
	if err != nil || pw != " p w'x " {
		t.Fatalf("fetchPassword = %q, %v", pw, err)
	}
	if _, err := fetchPassword(sock, token); err == nil {
		t.Error("the password was given twice")
	}
}

func TestAttachPasswordAuthEnv(t *testing.T) {
	useMockKeychain(t)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	cmd := exec.Command("true")
	cmd.Env = []string{"SSH_ASKPASS=/old", "SSHR_ASKPASS_TOKEN=old", "KEEP=1"}
	stop, err := AttachPasswordAuth(cmd, &Host{ID: "a", PasswordMode: PasswordSaved, Password: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	env := strings.Join(cmd.Env, "\n")
	for _, bad := range []string{"/old", "=old", "pw"} {
		if strings.Contains(env, bad) {
			t.Errorf("env has %q:\n%s", bad, env)
		}
	}
	if !strings.Contains(env, "KEEP=1") || !strings.Contains(env, "SSH_ASKPASS_REQUIRE=force") {
		t.Errorf("env lost a variable:\n%s", env)
	}
	cmd = exec.Command("true")
	stop2, err := AttachPasswordAuth(cmd, &Host{PasswordMode: PasswordAsk})
	stop2()
	if err != nil || cmd.Env != nil {
		t.Errorf("ask mode touched the command: %v %v", cmd.Env, err)
	}
}

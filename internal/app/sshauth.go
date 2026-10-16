package app

import (
	"os"
	"os/exec"
	"path/filepath"
)

// AttachPasswordAuth makes ssh read h.Password via SSH_ASKPASS when no key
// is set. Returns a cleanup that removes the temp askpass script.
func AttachPasswordAuth(cmd *exec.Cmd, h *Host) func() {
	noop := func() {}
	if cmd == nil || h == nil || h.Password == "" || h.Key != "" {
		return noop
	}
	dir, err := os.MkdirTemp("", "sshr-askpass-*")
	if err != nil {
		return noop
	}
	script := filepath.Join(dir, "askpass")
	body := "#!/bin/sh\nprintf '%s\\n' " + shQuote(h.Password) + "\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		os.RemoveAll(dir)
		return noop
	}
	env := cmd.Env
	if env == nil {
		env = os.Environ()
	}
	env = append(env,
		"SSH_ASKPASS="+script,
		"SSH_ASKPASS_REQUIRE=force",
	)
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		env = append(env, "DISPLAY=:0")
	}
	cmd.Env = env
	return func() { os.RemoveAll(dir) }
}

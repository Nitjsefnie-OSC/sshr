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
	// SSH_ASKPASS_REQUIRE=force sends every prompt here, the host key
	// question and key passphrases included, only a password prompt gets the
	// password, the rest is asked on the terminal, input hidden unless it is
	// a yes/no question
	body := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"*[Pp]assword:|*[Pp]assword:\\ ) printf '%s\\n' " + shQuote(h.Password) + " ;;\n" +
		"*)\n" +
		"\t( : </dev/tty ) 2>/dev/null || exit 1\n" +
		"\tprintf '%s' \"$1\" >/dev/tty\n" +
		"\tcase \"$1\" in *'?'|*'? '|*yes/no*) ;; *) stty -echo </dev/tty ;; esac\n" +
		"\tIFS= read -r a </dev/tty\n" +
		"\tstty echo </dev/tty\n" +
		"\tcase \"$1\" in *'?'|*'? '|*yes/no*) ;; *) printf '\\n' >/dev/tty ;; esac\n" +
		"\tprintf '%s\\n' \"$a\" ;;\n" +
		"esac\n"
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

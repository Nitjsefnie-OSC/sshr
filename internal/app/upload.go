package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// UploadTTL is how long an uploaded image stays on the server. It is meant
// to be read once (by a person or a remote tool), not stored there.
const UploadTTL = 15 * time.Minute

// SSHArgs returns the ssh options for a host: key, password prefs, and port.
func SSHArgs(h *Host) []string {
	args := []string{}
	if h.Key != "" {
		args = append(args, "-i", h.Key)
	} else if h.Password != "" {
		args = append(args,
			"-o", "PreferredAuthentications=password",
			"-o", "PubkeyAuthentication=no",
		)
	}
	return append(args, "-p", strconv.Itoa(h.Port))
}

// Dest is the user@host argument for ssh.
func (h *Host) Dest() string {
	if h.User == "" {
		return h.Host
	}
	return h.User + "@" + h.Host
}

// UploadImage copies data to ~/.cache/sshr/pasted/ on the host over a
// separate ssh and returns the path there. The file is removed after
// UploadTTL by a detached sleep; each upload also sweeps older files in case
// that sleep was killed by a logout.
//
// batch disables password prompts (ssh -o BatchMode=yes): set it when there
// is no terminal to ask in, as in the GUI, the upload goes through the
// host's jump hosts like a normal connection
func (s *Store) UploadImage(h *Host, data []byte, ext string, batch bool) (string, error) {
	if _, err := exec.LookPath("ssh"); err != nil {
		return "", errors.New("ssh not found in PATH")
	}
	name := fmt.Sprintf("paste-%d.%s", time.Now().UnixMilli(), ext)
	mins := int(UploadTTL / time.Minute)
	script := strings.Join([]string{
		`d="$HOME/.cache/sshr/pasted"`,
		`mkdir -p "$d" || exit 1`,
		`f="$d/` + name + `"`,
		`cat > "$f" || exit 1`,
		fmt.Sprintf(`find "$d" -maxdepth 1 -name 'paste-*' -mmin +%d -exec rm -f {} + 2>/dev/null`, mins),
		fmt.Sprintf(`nohup sh -c 'sleep %d; rm -f "$1"' _ "$f" </dev/null >/dev/null 2>&1 &`, mins*60),
		`printf "%s" "$f"`,
	}, "\n")

	args, err := s.ConnectArgs(h)
	if err != nil {
		return "", err
	}
	if batch {
		args = append(args, "-o", "BatchMode=yes")
	}
	// The login shell on the server may be fish, so the script runs under sh.
	args = append(args, "-o", "ConnectTimeout=10", "-T", "--", h.Dest(), "sh -c "+shQuote(script))

	cmd := exec.Command("ssh", args...)
	if batch {
		// Jump hops run as separate ssh processes, they read this too
		cmd.Env = append(os.Environ(), "SSHR_BATCH=1")
	}
	cmd.Stdin = bytes.NewReader(data)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cleanup := AttachPasswordAuth(cmd, h)
	defer cleanup()
	err = cmd.Run()
	remote := strings.TrimSpace(stdout.String())
	if err == nil && strings.HasPrefix(remote, "/") {
		return remote, nil
	}
	msg := strings.TrimSpace(stderr.String())
	if msg != "" {
		lines := strings.Split(msg, "\n")
		msg = strings.TrimSpace(lines[len(lines)-1])
	} else if err != nil {
		msg = err.Error()
	} else {
		msg = "server returned no path (is sh available there?)"
	}
	return "", fmt.Errorf("upload to %s failed: %s", h.Name, msg)
}

// shQuote wraps s in single quotes for a POSIX shell.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

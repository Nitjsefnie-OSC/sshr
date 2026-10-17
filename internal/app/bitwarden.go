package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Modes for hosts whose secrets live in Bitwarden, read with the bw CLI on
// every connect, the reference is the item ID (or a name that finds one
// item)
const (
	PasswordBitwarden = "bitwarden"
	KeyBitwarden      = "bitwarden"
)

// ErrNoBW means the Bitwarden CLI (bw) is not installed
var ErrNoBW = errors.New("no_bw")

// bwTimeout bounds one bw call, bw starts Node and syncs on the first call
const bwTimeout = time.Minute

// bwBin is the bw executable, tests point it at a fake
var bwBin = "bw"

// bwRun runs bw without prompts, it reads the unlocked session from
// BW_SESSION in the environment, an error carries the last line bw printed
func bwRun(args ...string) ([]byte, error) {
	bin, err := exec.LookPath(bwBin)
	if err != nil {
		return nil, fmt.Errorf("%w: the Bitwarden CLI (bw) is not installed, see https://bitwarden.com/help/cli/", ErrNoBW)
	}
	ctx, cancel := context.WithTimeout(context.Background(), bwTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, append(args, "--nointeraction")...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, errors.New("Bitwarden did not answer in time")
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if i := strings.LastIndexByte(msg, '\n'); i >= 0 {
			msg = msg[i+1:]
		}
		if msg == "" {
			msg = err.Error()
		}
		if strings.Contains(msg, "locked") || strings.Contains(msg, "not logged in") {
			msg += ", run bw unlock and export BW_SESSION"
		}
		return nil, fmt.Errorf("Bitwarden: %s", msg)
	}
	return stdout.Bytes(), nil
}

// checkBWRef refuses what bw would read as an option or cannot be an item
func checkBWRef(ref string) error {
	if ref == "" || strings.HasPrefix(ref, "-") || strings.ContainsAny(ref, "\r\n") {
		return fmt.Errorf("%q is not a Bitwarden item, give its ID (bw list items) or a name that finds one item", ref)
	}
	return nil
}

// bwPassword reads the password of a Login item
func bwPassword(ref string) ([]byte, error) {
	out, err := bwRun("get", "password", ref)
	if err != nil {
		return nil, err
	}
	return bytes.TrimRight(out, "\r\n"), nil
}

// bwKey reads the private and public key of an SSH Key item, bw has no
// "get" for them, the item JSON carries them in sshKey
func bwKey(ref string) (private []byte, public string, err error) {
	out, err := bwRun("get", "item", ref)
	if err != nil {
		return nil, "", err
	}
	var item struct {
		Name   string `json:"name"`
		SSHKey *struct {
			PrivateKey string `json:"privateKey"`
			PublicKey  string `json:"publicKey"`
		} `json:"sshKey"`
	}
	if err := json.Unmarshal(out, &item); err != nil {
		return nil, "", fmt.Errorf("Bitwarden: unexpected answer for item %s: %v", ref, err)
	}
	if item.SSHKey == nil || item.SSHKey.PrivateKey == "" {
		return nil, "", fmt.Errorf("Bitwarden item %q is not an SSH Key item", item.Name)
	}
	return []byte(item.SSHKey.PrivateKey), strings.TrimSpace(item.SSHKey.PublicKey), nil
}

// bwKeyPub reads the key of an SSH Key item once and returns its public
// half, from the private key, so it always matches what ssh will be given
func bwKeyPub(ref string) (string, error) {
	plain, _, err := bwKey(ref)
	if err != nil {
		return "", err
	}
	raw, err := ssh.ParseRawPrivateKey(plain)
	if err != nil {
		return "", fmt.Errorf("Bitwarden item %s holds no usable private key: %v", ref, err)
	}
	signer, err := ssh.NewSignerFromKey(raw)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))), nil
}

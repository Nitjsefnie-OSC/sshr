package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"sshr.dev/internal/app"
)

// cmdCopy is sshr send and sshr get, scp with the host's own login
func cmdCopy(store *app.Store, send bool, args []string) {
	name := "get"
	usage := "Usage: sshr get <name or id> <remote path>... [--to <local folder>]"
	dest := "."
	if send {
		name = "send"
		usage = "Usage: sshr send <name or id> <local path>... [--to <remote folder>]"
		dest = ""
	}
	var rest []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--to" && i+1 < len(args):
			i++
			dest = args[i]
		case strings.HasPrefix(a, "--to="):
			dest = strings.TrimPrefix(a, "--to=")
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}
	h := findHost(store, name, rest[0])
	sources := rest[1:]
	if send {
		for _, p := range sources {
			if _, err := os.Stat(p); err != nil {
				fatal(err)
			}
		}
	}
	scp, err := exec.LookPath("scp")
	if err != nil {
		fatal(fmt.Errorf("scp not found in PATH, it comes with OpenSSH"))
	}
	scpArgs, err := store.CopyArgs(h, send, sources, dest)
	if err != nil {
		fatal(err)
	}
	os.Exit(run(h, scp, scpArgs...))
}

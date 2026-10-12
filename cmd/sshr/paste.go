package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sshr.dev/internal/app"
	"sshr.dev/internal/clip"
)

// cmdPaste uploads the clipboard image (or a given file) to a host and prints
// the path on the server, so it can be pasted into a remote session instead
// of a local path the server cannot read.
func cmdPaste(store *app.Store, args []string) {
	copyPath := false
	file := ""
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-c" || a == "--copy":
			copyPath = true
		case (a == "-f" || a == "--file") && i+1 < len(args):
			i++
			file = args[i]
		case strings.HasPrefix(a, "--file="):
			file = strings.TrimPrefix(a, "--file=")
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: sshr paste [-c] [-f <file>] <name or id>")
		os.Exit(1)
	}
	query := strings.Join(rest, " ")
	h, err := store.Find(query)
	if err != nil {
		findFailed("paste", query, err)
	}

	var img *clip.Image
	if file != "" {
		data, err := os.ReadFile(expandHome(file))
		if err != nil {
			fatal(err)
		}
		ext := strings.TrimPrefix(filepath.Ext(file), ".")
		if ext == "" {
			ext = "png"
		}
		img = &clip.Image{Data: data, Ext: ext}
	} else {
		img, err = clip.ReadImage()
		if errors.Is(err, clip.ErrNoImage) {
			fmt.Fprintln(os.Stderr, "sshr paste: no image in clipboard")
			os.Exit(1)
		}
		if err != nil {
			fatal(err)
		}
	}

	// ssh asks for a password on the terminal, not on stdin, so an
	// interactive run does not need BatchMode.
	remote, err := app.UploadImage(h, img.Data, img.Ext, false)
	if err != nil {
		fatal(err)
	}
	fmt.Println(remote)
	if copyPath {
		if err := clip.WriteText(remote); err != nil {
			fmt.Fprintf(os.Stderr, "sshr paste: could not copy path: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "path copied to clipboard")
	}
}

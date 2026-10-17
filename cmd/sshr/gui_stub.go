//go:build nocgo

package main

import (
	"fmt"
	"os"
)

// guiBuilt tells whether this build has the GUI
const guiBuilt = false

func runGUI() {
	fmt.Fprintln(os.Stderr, "sshr: GUI not available in this build. Use `sshr help` for CLI commands.")
	os.Exit(1)
}

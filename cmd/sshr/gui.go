//go:build !nocgo

package main

import "sshr.dev/internal/ui"

// guiBuilt tells whether this build has the GUI
const guiBuilt = true

func runGUI() {
	ui.Run()
}

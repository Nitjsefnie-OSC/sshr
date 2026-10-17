// The Windows icon and version info are resources in rsrc_windows_*.syso,
// go build links them in, regenerate them after the logo changes:
//
//go:generate go run github.com/tc-hib/go-winres@v0.3.3 simply --arch amd64,386,arm64 --manifest none --icon ../../assets/icons/sshr-512.png --product-name sshr --file-description "SSH host manager" --original-filename sshr.exe --out rsrc

package main

import (
	"os"

	"sshr.dev/internal/app"
)

func main() {
	// ssh runs sshr as SSH_ASKPASS for hosts with a saved password
	if app.IsAskpassCall(os.Args[1:]) {
		os.Exit(app.RunAskpass(os.Args[1]))
	}
	if len(os.Args) > 1 {
		runCLI(os.Args[1:])
		return
	}
	runGUI()
}

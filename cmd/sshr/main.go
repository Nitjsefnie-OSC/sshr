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

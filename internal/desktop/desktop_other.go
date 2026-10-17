//go:build !linux && !darwin

package desktop

import (
	"errors"
	"runtime"
)

var errUnsupported = errors.New("there is no application menu entry to make on " + runtime.GOOS + ", the icon is part of the program")

// Install is not needed here, on Windows the .exe carries its icon
func Install() (string, error) { return "", errUnsupported }

// Remove has nothing to remove here
func Remove() error { return errUnsupported }

//go:build !windows

// Package assets holds the logo, sshr installs it as its icon on Linux
// and macOS, on Windows the icon is a resource of the .exe (cmd/sshr/*.syso)
package assets

import "embed"

// Icons are icons/sshr-<size>.png in IconSizes
//
//go:embed icons/*.png
var Icons embed.FS

// IconSizes are the sizes of Icons in pixels
var IconSizes = []int{16, 32, 48, 64, 128, 256, 512}

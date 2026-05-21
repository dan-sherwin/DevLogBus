// Package ui embeds the built DevLogBus browser UI for devlogbusd.
package ui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// DistFS returns the built UI bundle rooted at dist/.
func DistFS() (fs.FS, error) {
	return fs.Sub(dist, "dist")
}

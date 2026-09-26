// Package ui embeds the built frontend (see /frontend) into the Go binary.
// Run `npm run build` in /frontend before `go build` to populate dist/.
package ui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS returns the built frontend with dist/ as its root.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

// Package web serves the compiled single-page application from assets
// embedded in the binary, so a deployment is one artifact rather than a
// binary plus a directory of files that has to be shipped and mounted
// alongside it.
package web

import "embed"

// all: is required, not cosmetic: dist/.gitkeep is the committed placeholder
// that keeps this directive compiling before anyone has run a frontend
// build, and a bare //go:embed dist would skip it as a dotfile and fail the
// build once the real assets (which include no dotfiles) replace it.
//
//go:embed all:dist
var dist embed.FS

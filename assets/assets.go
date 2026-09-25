// Package assets exposes the web UI static files, embedded at compile time.
package assets

import (
	"embed"
	"strings"
)

//go:embed css fonts images js templates
var content embed.FS

// Asset returns the contents of an embedded file. Names use the historical
// go-bindata scheme with a leading "assets/" prefix, e.g. "assets/js/app.js".
func Asset(name string) ([]byte, error) {
	return content.ReadFile(strings.TrimPrefix(name, "assets/"))
}

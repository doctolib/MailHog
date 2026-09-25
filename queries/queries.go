// Package queries exposes the SQL schema files, embedded at compile time.
package queries

import (
	"embed"
	"strings"
)

//go:embed *.sql
var content embed.FS

// Asset returns the contents of an embedded file. Names use the historical
// go-bindata scheme with a leading "queries/" prefix,
// e.g. "queries/postgresql-schema.sql".
func Asset(name string) ([]byte, error) {
	return content.ReadFile(strings.TrimPrefix(name, "queries/"))
}

// Package dbmig membawa file migrasi SQL ke dalam binary.
package dbmig

import "embed"

//go:embed migrations/*.sql
var FS embed.FS

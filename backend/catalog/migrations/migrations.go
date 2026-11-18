// Package migrations embeds the catalog schema. The service applies it at startup.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

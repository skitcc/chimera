// Package migrations embeds the identity schema. The service applies it at startup.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

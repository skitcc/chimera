// Package migrations embeds the social schema. The service applies it at startup.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

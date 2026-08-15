package migrations

import "embed"

// FS contains migrations shipped with Balemoh.
//go:embed *.sql
var FS embed.FS

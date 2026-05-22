package static

import "embed"

// FS is the embedded filesystem containing all static web assets
//go:embed *
var FS embed.FS

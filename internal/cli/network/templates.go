package network

import (
	"embed"
	"io/fs"
)

//go:embed templates
var embeddedTemplates embed.FS

// networkTemplates is the root FS for network domain templates (no "templates/" prefix).
var networkTemplates, _ = fs.Sub(embeddedTemplates, "templates")

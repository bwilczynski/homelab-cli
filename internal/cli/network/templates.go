package network

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed templates
var embeddedTemplates embed.FS

var networkTemplates = mustSubFS(embeddedTemplates, "templates")

func mustSubFS(f fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		// Structurally impossible: //go:embed guarantees the directory exists at compile time.
		panic(fmt.Sprintf("failed to create network templates FS: %v", err))
	}
	return sub
}

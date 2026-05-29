package network

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed templates
var embeddedTemplates embed.FS

var networkTemplates fs.FS

func init() {
	var err error
	networkTemplates, err = fs.Sub(embeddedTemplates, "templates")
	if err != nil {
		// Structurally impossible: //go:embed guarantees the directory exists at compile time.
		panic(fmt.Sprintf("failed to create network templates FS: %v", err))
	}
}

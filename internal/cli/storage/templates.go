package storage

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed templates
var embeddedTemplates embed.FS

var storageTemplates fs.FS

func init() {
	var err error
	storageTemplates, err = fs.Sub(embeddedTemplates, "templates")
	if err != nil {
		panic(fmt.Sprintf("failed to create storage templates FS: %v", err))
	}
}

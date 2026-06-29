package version

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed templates
var embeddedTemplates embed.FS

var versionTemplates = mustSubFS(embeddedTemplates, "templates")

func mustSubFS(f fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(fmt.Sprintf("failed to create version templates FS: %v", err))
	}
	return sub
}

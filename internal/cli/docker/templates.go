package docker

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed templates
var embeddedTemplates embed.FS

var dockerTemplates fs.FS

func init() {
	var err error
	dockerTemplates, err = fs.Sub(embeddedTemplates, "templates")
	if err != nil {
		panic(fmt.Sprintf("failed to create docker templates FS: %v", err))
	}
}

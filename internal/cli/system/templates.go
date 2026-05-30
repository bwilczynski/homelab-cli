package system

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed templates
var embeddedTemplates embed.FS

var systemTemplates fs.FS

func init() {
	var err error
	systemTemplates, err = fs.Sub(embeddedTemplates, "templates")
	if err != nil {
		panic(fmt.Sprintf("failed to create system templates FS: %v", err))
	}
}

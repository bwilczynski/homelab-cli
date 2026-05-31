package cmdutil

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/output"
)

// View binds a template filesystem, a template name, and the expected success
// status code. Domains declare one View per renderable response and reuse it
// across the command(s) that produce it.
//
// Status is optional and defaults to http.StatusOK. Set it explicitly for
// endpoints that return 201, 202, etc. — the value pairs with the JSON field
// the caller passes (a 201 endpoint populates resp.JSON201, not resp.JSON200).
type View struct {
	Templates fs.FS
	Name      string
	Status    int
}

// Render handles the standard response→output flow:
//   - status != v.Status (or 200 if unset) → apiclient.ParseError
//   - --output=json → write raw body
//   - otherwise → render the bound template against data
func (v View) Render(w io.Writer, statusCode int, body []byte, data any) error {
	expected := v.Status
	if expected == 0 {
		expected = http.StatusOK
	}
	if statusCode != expected {
		return apiclient.ParseError(statusCode, body)
	}
	if flags.GetOutputFormat() == output.FormatJSON {
		fmt.Fprint(w, string(body))
		return nil
	}
	return output.RenderTemplate(w, v.Templates, v.Name, data)
}

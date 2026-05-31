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

// renderHead handles the status check and JSON shortcut shared by every
// render path. Returns handled=true when the JSON body has been written and
// the caller should return nil; returns a non-nil error on status mismatch.
func renderHead(w io.Writer, expectedStatus, statusCode int, body []byte) (handled bool, err error) {
	expected := expectedStatus
	if expected == 0 {
		expected = http.StatusOK
	}
	if statusCode != expected {
		return false, apiclient.ParseError(statusCode, body)
	}
	if flags.GetOutputFormat() == output.FormatJSON {
		fmt.Fprint(w, string(body))
		return true, nil
	}
	return false, nil
}

// Render handles the standard response→output flow:
//   - status != v.Status (or 200 if unset) → apiclient.ParseError
//   - --output=json → write raw body
//   - otherwise → render the bound template against data
func (v View) Render(w io.Writer, statusCode int, body []byte, data any) error {
	handled, err := renderHead(w, v.Status, statusCode, body)
	if handled || err != nil {
		return err
	}
	return output.RenderTemplate(w, v.Templates, v.Name, data)
}

// RenderWith mirrors Render but defers data construction. fn is invoked only
// in table mode — JSON mode dumps the raw body without running fn. Use this
// when the template data needs to be derived from the response body and the
// derivation work would be wasted in JSON mode.
func (v View) RenderWith(w io.Writer, statusCode int, body []byte, fn func() (any, error)) error {
	handled, err := renderHead(w, v.Status, statusCode, body)
	if handled || err != nil {
		return err
	}
	data, err := fn()
	if err != nil {
		return err
	}
	return output.RenderTemplate(w, v.Templates, v.Name, data)
}

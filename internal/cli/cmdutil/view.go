package cmdutil

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/api"
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
		return false, api.ParseError(statusCode, body)
	}
	if flags.GetOutputFormat() == output.FormatJSON {
		fmt.Fprint(w, string(body))
		return true, nil
	}
	return false, nil
}

// Render handles the standard response→output flow:
//   - status != v.Status (or 200 if unset) → api.ParseError
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

// Discriminator constrains polymorphic response bodies. Oapi-codegen union
// types satisfy this automatically — each generated *Detail struct has a
// Discriminator() (string, error) method.
type Discriminator interface {
	Discriminator() (string, error)
}

// Variant binds one discriminator branch to its template and a resolver that
// extracts the typed variant (and optionally transforms it) from the union.
type Variant[T Discriminator] struct {
	Template string
	Resolve  func(T) (any, error)
}

// PolymorphicView is the View equivalent for discriminated-union responses.
// Variants is keyed by the discriminator string returned by T.Discriminator().
// Status defaults to http.StatusOK.
type PolymorphicView[T Discriminator] struct {
	Templates fs.FS
	Status    int
	Variants  map[string]Variant[T]
}

// Render handles the status check + JSON shortcut, then dispatches on
// detail.Discriminator() to look up the variant template and resolved data.
func (v PolymorphicView[T]) Render(w io.Writer, statusCode int, body []byte, detail *T) error {
	handled, err := renderHead(w, v.Status, statusCode, body)
	if handled || err != nil {
		return err
	}
	if detail == nil {
		var zero T
		return fmt.Errorf("nil %T body", zero)
	}
	disc, err := (*detail).Discriminator()
	if err != nil {
		return err
	}
	variant, ok := v.Variants[disc]
	if !ok {
		return fmt.Errorf("unknown %T discriminator: %q", *detail, disc)
	}
	data, err := variant.Resolve(*detail)
	if err != nil {
		return err
	}
	return output.RenderTemplate(w, v.Templates, variant.Template, data)
}

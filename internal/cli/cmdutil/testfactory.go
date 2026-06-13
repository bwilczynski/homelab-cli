package cmdutil

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/config"
	"github.com/bwilczynski/hlctl/internal/output"
)

// TestFactory builds a Factory suitable for cobra leaf tests. The Config and
// HTTPClient closures return errors so any test that accidentally triggers
// real I/O fails loudly — tests that drive a stub via SetClient[C] never
// reach these closures.
//
// Tests that need JSON output mode override the Output field directly:
//
//	f := cmdutil.TestFactory(t)
//	f.Output = func() output.Format { return output.FormatJSON }
func TestFactory(t *testing.T) *Factory {
	t.Helper()
	return &Factory{
		Version:   "test",
		IOStreams: &IOStreams{In: strings.NewReader(""), Out: io.Discard, ErrOut: io.Discard},
		Config: func() (*config.Config, error) {
			return nil, errors.New("TestFactory: Config not configured")
		},
		APIURL: func() (string, error) {
			return "", errors.New("TestFactory: APIURL not configured")
		},
		HTTPClient: func() (*http.Client, string, error) {
			return nil, "", errors.New("TestFactory: HTTPClient not configured")
		},
		Output: func() output.Format { return output.FormatTable },
	}
}

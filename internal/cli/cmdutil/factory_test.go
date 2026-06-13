package cmdutil_test

import (
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
)

func TestNewFactory_outputFlagDefersToLatestValue(t *testing.T) {
	apiURL := ""
	outputFmt := "table"
	f := cmdutil.NewFactory("test", &apiURL, &outputFmt)

	if got := f.Output(); got != output.FormatTable {
		t.Errorf("expected table, got %q", got)
	}

	outputFmt = "json"
	if got := f.Output(); got != output.FormatJSON {
		t.Errorf("expected json after flag change, got %q", got)
	}
}

func TestNewFactory_apiURLFlagOverridesConfig(t *testing.T) {
	apiURL := "https://override.test"
	outputFmt := "table"
	f := cmdutil.NewFactory("test", &apiURL, &outputFmt)

	got, err := f.APIURL()
	if err != nil {
		t.Fatalf("APIURL: %v", err)
	}
	if got != "https://override.test" {
		t.Errorf("expected override URL, got %q", got)
	}
}

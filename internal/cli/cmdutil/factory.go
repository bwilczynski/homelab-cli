package cmdutil

import (
	"net/http"
	"sync"

	"github.com/bwilczynski/hlctl/internal/auth"
	"github.com/bwilczynski/hlctl/internal/config"
	"github.com/bwilczynski/hlctl/internal/output"
)

// Factory bundles the building blocks every command needs. Constructed once in
// main, threaded through every NewCmd. Function-valued fields defer expensive
// work (config load, token read, URL resolution) until a command actually runs.
type Factory struct {
	Version string

	IOStreams *IOStreams

	Config     func() (*config.Config, error)
	APIURL     func() (string, error)
	HTTPClient func() (*http.Client, string, error)
	Output     func() output.Format
}

// NewFactory builds the default Factory wired to real config/auth/http. The
// caller passes *string pointers to flag-backed storage (declared on the root
// command's PersistentFlags); the returned closures read the *latest* flag
// values each invocation, so resolution sees flag-parsing outcomes correctly.
func NewFactory(version string, apiURLFlag, outputFlag *string) *Factory {
	var (
		cfg     *config.Config
		cfgErr  error
		cfgOnce sync.Once
	)
	loadConfig := func() (*config.Config, error) {
		cfgOnce.Do(func() { cfg, cfgErr = config.Load() })
		return cfg, cfgErr
	}
	apiURLFn := func() (string, error) {
		if *apiURLFlag != "" {
			return *apiURLFlag, nil
		}
		c, err := loadConfig()
		if err != nil {
			return "", err
		}
		return c.ResolveAPIURL()
	}
	return &Factory{
		Version:   version,
		IOStreams: SystemIOStreams(),
		Config:    loadConfig,
		APIURL:    apiURLFn,
		HTTPClient: func() (*http.Client, string, error) {
			apiURL, err := apiURLFn()
			if err != nil {
				return nil, "", err
			}
			return &http.Client{Transport: auth.NewAuthenticatedTransport(nil)}, apiURL, nil
		},
		Output: func() output.Format { return output.Format(*outputFlag) },
	}
}

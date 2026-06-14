package system

import (
	"context"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var healthView = cmdutil.View{Templates: systemTemplates, Name: "health.tmpl"}

type getHealthOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
}

func newHealthCmd(f *cmdutil.Factory, runF func(*getHealthOptions) error) *cobra.Command {
	opts := &getHealthOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "health",
		Short: "Show aggregate system health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(opts)
			}
			return getHealthRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getHealthRun(ctx context.Context, w io.Writer, opts *getHealthOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewSystemClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetSystemHealthWithResponse(ctx)
	if err != nil {
		return err
	}
	return healthView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

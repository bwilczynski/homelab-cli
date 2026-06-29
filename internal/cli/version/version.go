package version

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var versionView = cmdutil.View{Templates: versionTemplates, Name: "version.tmpl"}

type versionOptions struct {
	ClientVersion string
	ClientSpec    string
	ClientOnly    bool
	HTTPClient    func() (*http.Client, string, error)
	IO            *cmdutil.IOStreams
	Output        func() output.Format
}

type versionOutput struct {
	ClientVersion string  `json:"clientVersion"`
	ClientSpec    string  `json:"clientSpec"`
	ServerVersion *string `json:"serverVersion,omitempty"`
	ServerSpec    *string `json:"serverSpec,omitempty"`
	ClientOnly    bool    `json:"-"`
}

// NewCmd returns the `hlctl version` command.
func NewCmd(f *cmdutil.Factory, runF func(*versionOptions) error) *cobra.Command {
	opts := &versionOptions{
		ClientVersion: f.Version,
		ClientSpec:    f.SpecVersion,
		HTTPClient:    f.HTTPClient,
		IO:            f.IOStreams,
		Output:        f.Output,
	}
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show client and server version information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(opts)
			}
			return getVersionRun(cmd.Context(), cmd.OutOrStdout(), opts)
		},
	}
	cmd.Flags().BoolVar(&opts.ClientOnly, "client", false, "Show client version only (no server request)")
	return cmd
}

func getVersionRun(ctx context.Context, w io.Writer, opts *versionOptions) error {
	out := versionOutput{
		ClientVersion: opts.ClientVersion,
		ClientSpec:    opts.ClientSpec,
		ClientOnly:    opts.ClientOnly,
	}

	if !opts.ClientOnly {
		if httpClient, apiURL, err := opts.HTTPClient(); err != nil {
			fmt.Fprintf(opts.IO.ErrOut, "warning: could not reach server: %v\n", err)
		} else if c, err := NewMetaClient(httpClient, apiURL); err != nil {
			fmt.Fprintf(opts.IO.ErrOut, "warning: could not reach server: %v\n", err)
		} else if resp, err := c.GetMetaVersionWithResponse(ctx); err != nil {
			fmt.Fprintf(opts.IO.ErrOut, "warning: could not reach server: %v\n", err)
		} else if resp.JSON200 != nil {
			out.ServerVersion = &resp.JSON200.ServerVersion
			out.ServerSpec = &resp.JSON200.ApiVersion
		}
	}

	return versionView.RenderObject(w, opts.Output(), out)
}

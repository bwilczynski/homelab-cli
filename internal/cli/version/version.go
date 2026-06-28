package version

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"text/tabwriter"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

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
	var serverVersion, serverSpec string

	if !opts.ClientOnly {
		if httpClient, apiURL, err := opts.HTTPClient(); err != nil {
			fmt.Fprintf(opts.IO.ErrOut, "warning: could not reach server: %v\n", err)
		} else if c, err := NewMetaClient(httpClient, apiURL); err != nil {
			fmt.Fprintf(opts.IO.ErrOut, "warning: could not reach server: %v\n", err)
		} else if resp, err := c.GetMetaVersionWithResponse(ctx); err != nil {
			fmt.Fprintf(opts.IO.ErrOut, "warning: could not reach server: %v\n", err)
		} else if resp.JSON200 != nil {
			serverVersion = resp.JSON200.ServerVersion
			serverSpec = resp.JSON200.ApiVersion
		}
	}

	if opts.Output() == output.FormatJSON {
		out := versionOutput{
			ClientVersion: opts.ClientVersion,
			ClientSpec:    opts.ClientSpec,
		}
		if serverVersion != "" {
			out.ServerVersion = &serverVersion
			out.ServerSpec = &serverSpec
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "Client version:\t%s\n", opts.ClientVersion)
	fmt.Fprintf(tw, "Client spec:\t%s\n", opts.ClientSpec)
	if !opts.ClientOnly {
		if serverVersion != "" {
			fmt.Fprintf(tw, "Server version:\t%s\n", serverVersion)
			fmt.Fprintf(tw, "Server spec:\t%s\n", serverSpec)
		} else {
			fmt.Fprintf(tw, "Server version:\t(unavailable)\n")
			fmt.Fprintf(tw, "Server spec:\t(unavailable)\n")
		}
	}
	return tw.Flush()
}

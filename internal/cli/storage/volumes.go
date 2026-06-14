package storage

import (
	"context"
	"io"
	"net/http"

	storageapi "github.com/bwilczynski/hlctl/internal/api/storage"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	volumesListView = cmdutil.View{Templates: storageTemplates, Name: "volumes_list.tmpl"}
	volumesGetView  = cmdutil.View{Templates: storageTemplates, Name: "volumes_get.tmpl"}
)

// --- list ---

type listVolumesOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	Device     string
}

func newListVolumesCmd(f *cmdutil.Factory, runF func(*listVolumesOptions) error) *cobra.Command {
	opts := &listVolumesOptions{
		HTTPClient: f.HTTPClient,
		IO:         f.IOStreams,
		Output:     f.Output,
	}
	cmd := &cobra.Command{Use: "list", Short: "List storage volumes"}
	cmd.Flags().StringVar(&opts.Device, "device", "", "Filter by device ID")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if runF != nil {
			return runF(opts)
		}
		return listVolumesRun(cmd.Context(), opts.IO.Out, opts)
	}
	return cmd
}

func listVolumesRun(ctx context.Context, w io.Writer, opts *listVolumesOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewStorageClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &storageapi.ListStorageVolumesParams{}
	if opts.Device != "" {
		params.Device = &opts.Device
	}
	resp, err := c.ListStorageVolumesWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return volumesListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

// --- get ---

type getVolumeOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
}

func newGetVolumeCmd(f *cmdutil.Factory, runF func(*getVolumeOptions) error) *cobra.Command {
	opts := &getVolumeOptions{
		HTTPClient: f.HTTPClient,
		IO:         f.IOStreams,
		Output:     f.Output,
	}
	return &cobra.Command{
		Use:   "get <volume-id>",
		Short: "Show volume details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getVolumeRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getVolumeRun(ctx context.Context, w io.Writer, opts *getVolumeOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewStorageClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetStorageVolumeWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return volumesGetView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

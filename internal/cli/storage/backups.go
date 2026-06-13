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
	backupsListView = cmdutil.View{Templates: storageTemplates, Name: "backups_list.tmpl"}
	backupsGetView  = cmdutil.View{Templates: storageTemplates, Name: "backups_get.tmpl"}
)

// --- list ---

type listBackupsOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	Device     string
}

func newListBackupsCmd(f *cmdutil.Factory, runF func(*listBackupsOptions) error) *cobra.Command {
	opts := &listBackupsOptions{
		HTTPClient: f.HTTPClient,
		IO:         f.IOStreams,
		Output:     f.Output,
	}
	cmd := &cobra.Command{Use: "list", Short: "List backups"}
	cmd.Flags().StringVar(&opts.Device, "device", "", "Filter by device ID")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if runF != nil {
			return runF(opts)
		}
		return listBackupsRun(cmd.Context(), opts.IO.Out, opts)
	}
	return cmd
}

func listBackupsRun(ctx context.Context, w io.Writer, opts *listBackupsOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewStorageClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &storageapi.ListBackupsParams{}
	if opts.Device != "" {
		params.Device = &opts.Device
	}
	resp, err := c.ListBackupsWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return backupsListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

// --- get ---

type getBackupOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
}

func newGetBackupCmd(f *cmdutil.Factory, runF func(*getBackupOptions) error) *cobra.Command {
	opts := &getBackupOptions{
		HTTPClient: f.HTTPClient,
		IO:         f.IOStreams,
		Output:     f.Output,
	}
	return &cobra.Command{
		Use:   "get <backup-id>",
		Short: "Show backup details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getBackupRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getBackupRun(ctx context.Context, w io.Writer, opts *getBackupOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewStorageClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetBackupWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return backupsGetView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

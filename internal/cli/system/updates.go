package system

import (
	"context"
	"io"
	"net/http"

	systemapi "github.com/bwilczynski/hlctl/internal/api/system"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	updatesListView = cmdutil.View{Templates: systemTemplates, Name: "updates_list.tmpl"}
	updateGetView   = cmdutil.PolymorphicView[systemapi.SystemUpdateDetail]{
		Templates: systemTemplates,
		Variants: map[string]cmdutil.Variant[systemapi.SystemUpdateDetail]{
			"container": {
				Template: "updates_get_container.tmpl",
				Resolve:  func(d systemapi.SystemUpdateDetail) (any, error) { return d.AsContainerSystemUpdateDetail() },
			},
		},
	}
)

type listUpdatesOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	Status     string
	UpdateType string
}

type getUpdateOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
}

type checkUpdatesOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
}

func newUpdatesCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "updates", Short: "Software update tracking"}
	cmd.AddCommand(newListUpdatesCmd(f, nil), newGetUpdateCmd(f, nil), newCheckUpdatesCmd(f, nil))
	return cmd
}

func newListUpdatesCmd(f *cmdutil.Factory, runF func(*listUpdatesOptions) error) *cobra.Command {
	opts := &listUpdatesOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tracked software updates",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(opts)
			}
			return listUpdatesRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
	cmd.Flags().StringVar(&opts.Status, "status", "", "Filter by update status (unknown, upToDate, updateAvailable)")
	cmd.Flags().StringVar(&opts.UpdateType, "type", "", "Filter by component type (container)")
	return cmd
}

func listUpdatesRun(ctx context.Context, w io.Writer, opts *listUpdatesOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewSystemClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &systemapi.ListSystemUpdatesParams{}
	if opts.Status != "" {
		s := systemapi.UpdateStatusFilter(opts.Status)
		params.Status = &s
	}
	if opts.UpdateType != "" {
		ut := systemapi.UpdateTypeFilter(opts.UpdateType)
		params.Type = &ut
	}
	resp, err := c.ListSystemUpdatesWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return updatesListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

func newGetUpdateCmd(f *cmdutil.Factory, runF func(*getUpdateOptions) error) *cobra.Command {
	opts := &getUpdateOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "get <update-id>",
		Short: "Show update details for a tracked component",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getUpdateRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getUpdateRun(ctx context.Context, w io.Writer, opts *getUpdateOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewSystemClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetSystemUpdateWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return updateGetView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

func newCheckUpdatesCmd(f *cmdutil.Factory, runF func(*checkUpdatesOptions) error) *cobra.Command {
	opts := &checkUpdatesOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "check",
		Short: "Force check for upstream updates",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(opts)
			}
			return checkUpdatesRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func checkUpdatesRun(ctx context.Context, w io.Writer, opts *checkUpdatesOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewSystemClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.CheckSystemUpdatesWithResponse(ctx, &systemapi.CheckSystemUpdatesParams{})
	if err != nil {
		return err
	}
	return updatesListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

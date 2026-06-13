package docker

import (
	"context"
	"io"
	"net/http"

	dockerapi "github.com/bwilczynski/hlctl/internal/api/docker"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	imagesListView = cmdutil.View{Templates: dockerTemplates, Name: "images_list.tmpl"}
	imagesGetView  = cmdutil.View{Templates: dockerTemplates, Name: "images_get.tmpl"}
)

type listImagesOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	Device     string
}

func newImagesCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "images", Short: "Docker images"}
	cmd.AddCommand(newListImagesCmd(f, nil), newGetImageCmd(f, nil))
	return cmd
}

func newListImagesCmd(f *cmdutil.Factory, runF func(*listImagesOptions) error) *cobra.Command {
	opts := &listImagesOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Docker images",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(opts)
			}
			return listImagesRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
	cmd.Flags().StringVar(&opts.Device, "device", "", "Filter by device ID")
	return cmd
}

func listImagesRun(ctx context.Context, w io.Writer, opts *listImagesOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewDockerClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &dockerapi.ListDockerImagesParams{}
	if opts.Device != "" {
		params.Device = &opts.Device
	}
	resp, err := c.ListDockerImagesWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return imagesListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

type getImageOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
}

func newGetImageCmd(f *cmdutil.Factory, runF func(*getImageOptions) error) *cobra.Command {
	opts := &getImageOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "get <image-id>",
		Short: "Show image details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getImageRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getImageRun(ctx context.Context, w io.Writer, opts *getImageOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewDockerClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetDockerImageWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return imagesGetView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

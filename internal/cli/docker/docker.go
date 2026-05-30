package docker

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
	gen "github.com/bwilczynski/hlctl/internal/docker"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker",
		Short: "Docker resources",
	}
	cmd.AddCommand(newContainersCmd())
	cmd.AddCommand(newNetworksCmd())
	cmd.AddCommand(newImagesCmd())
	return cmd
}

func buildClient() (DockerClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewDockerClient(httpClient, apiURL)
}

func newContainersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "containers",
		Short: "Manage Docker containers",
	}
	cmd.AddCommand(newListCmd(nil))
	cmd.AddCommand(newGetCmd(nil))
	cmd.AddCommand(newStartCmd(nil))
	cmd.AddCommand(newStopCmd(nil))
	cmd.AddCommand(newRestartCmd(nil))
	return cmd
}

func newListCmd(client DockerClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List containers",
	}
	cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
		c := client
		if c == nil {
			var err error
			c, err = buildClient()
			if err != nil {
				return err
			}
		}

		params := &gen.ListContainersParams{}
		if device != "" {
			params.Device = &device
		}

		resp, err := c.ListContainersWithResponse(ctx, params)
		if err != nil {
			return err
		}
		if resp.StatusCode() != http.StatusOK {
			return apiclient.ParseError(resp.StatusCode(), resp.Body)
		}

		if flags.GetOutputFormat() == output.FormatJSON {
			fmt.Fprint(w, string(resp.Body))
			return nil
		}

		return output.RenderTemplate(w, dockerTemplates, "containers_list.tmpl", *resp.JSON200)
	})

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	watch.RegisterFlags(cmd)
	return cmd
}

func newGetCmd(client DockerClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <container-id>",
		Short: "Show container details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.GetContainerWithResponse(context.Background(), args[0])
			if err != nil {
				return err
			}
			if resp.StatusCode() != http.StatusOK {
				return apiclient.ParseError(resp.StatusCode(), resp.Body)
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
				return nil
			}

			return output.RenderTemplate(cmd.OutOrStdout(), dockerTemplates, "containers_get.tmpl", *resp.JSON200)
		},
	}
}

func newStartCmd(client DockerClient) *cobra.Command {
	return &cobra.Command{
		Use:   "start <container-id>",
		Short: "Start a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}
			resp, err := c.StartContainerWithResponse(context.Background(), args[0], &gen.StartContainerParams{})
			if err != nil {
				return err
			}
			if resp.StatusCode() != http.StatusNoContent {
				return apiclient.ParseError(resp.StatusCode(), resp.Body)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Container %s started\n", args[0])
			return nil
		},
	}
}

func newStopCmd(client DockerClient) *cobra.Command {
	return &cobra.Command{
		Use:   "stop <container-id>",
		Short: "Stop a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}
			resp, err := c.StopContainerWithResponse(context.Background(), args[0], &gen.StopContainerParams{})
			if err != nil {
				return err
			}
			if resp.StatusCode() != http.StatusNoContent {
				return apiclient.ParseError(resp.StatusCode(), resp.Body)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Container %s stopped\n", args[0])
			return nil
		},
	}
}

func newRestartCmd(client DockerClient) *cobra.Command {
	return &cobra.Command{
		Use:   "restart <container-id>",
		Short: "Restart a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}
			resp, err := c.RestartContainerWithResponse(context.Background(), args[0], &gen.RestartContainerParams{})
			if err != nil {
				return err
			}
			if resp.StatusCode() != http.StatusNoContent {
				return apiclient.ParseError(resp.StatusCode(), resp.Body)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Container %s restarted\n", args[0])
			return nil
		},
	}
}

func newNetworksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "networks",
		Short: "Docker networks",
	}
	cmd.AddCommand(newListNetworksCmd(nil))
	cmd.AddCommand(newGetNetworkCmd(nil))
	return cmd
}

func newListNetworksCmd(client DockerClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Docker networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListDockerNetworksParams{}
			if device != "" {
				params.Device = &device
			}

			resp, err := c.ListDockerNetworksWithResponse(context.Background(), params)
			if err != nil {
				return err
			}
			if resp.StatusCode() != http.StatusOK {
				return apiclient.ParseError(resp.StatusCode(), resp.Body)
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
				return nil
			}

			return output.RenderTemplate(cmd.OutOrStdout(), dockerTemplates, "networks_list.tmpl", *resp.JSON200)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newGetNetworkCmd(client DockerClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <network-id>",
		Short: "Show network details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.GetDockerNetworkWithResponse(context.Background(), args[0])
			if err != nil {
				return err
			}
			if resp.StatusCode() != http.StatusOK {
				return apiclient.ParseError(resp.StatusCode(), resp.Body)
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
				return nil
			}

			return output.RenderTemplate(cmd.OutOrStdout(), dockerTemplates, "networks_get.tmpl", *resp.JSON200)
		},
	}
}

func newImagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "images",
		Short: "Docker images",
	}
	cmd.AddCommand(newListImagesCmd(nil))
	cmd.AddCommand(newGetImageCmd(nil))
	return cmd
}

func newListImagesCmd(client DockerClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Docker images",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListDockerImagesParams{}
			if device != "" {
				params.Device = &device
			}

			resp, err := c.ListDockerImagesWithResponse(context.Background(), params)
			if err != nil {
				return err
			}
			if resp.StatusCode() != http.StatusOK {
				return apiclient.ParseError(resp.StatusCode(), resp.Body)
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
				return nil
			}

			return output.RenderTemplate(cmd.OutOrStdout(), dockerTemplates, "images_list.tmpl", *resp.JSON200)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newGetImageCmd(client DockerClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <image-id>",
		Short: "Show image details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.GetDockerImageWithResponse(context.Background(), args[0])
			if err != nil {
				return err
			}
			if resp.StatusCode() != http.StatusOK {
				return apiclient.ParseError(resp.StatusCode(), resp.Body)
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
				return nil
			}

			return output.RenderTemplate(cmd.OutOrStdout(), dockerTemplates, "images_get.tmpl", *resp.JSON200)
		},
	}
}


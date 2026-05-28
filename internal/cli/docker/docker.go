package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

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

		resp, err := c.ListContainers(ctx, params)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return apiclient.ParseError(resp)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		var list gen.ContainerList
		if err := json.Unmarshal(body, &list); err != nil {
			return err
		}

		if flags.GetOutputFormat() == output.FormatJSON {
			fmt.Fprint(w, string(body))
			return nil
		}

		headers := []string{"ID", "IMAGE", "STATUS", "CPU", "MEMORY"}
		var rows [][]string
		for _, c := range list.Items {
			rows = append(rows, []string{
				c.Id,
				c.Image,
				string(c.Status),
				fmt.Sprintf("%.1f%%", c.Resources.CpuPercent),
				output.FormatBytes(c.Resources.MemoryBytes),
			})
		}
		return output.Print(w, flags.GetOutputFormat(), list, headers, rows)
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

			resp, err := c.GetContainer(context.Background(), args[0])
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return apiclient.ParseError(resp)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			var detail gen.ContainerDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			return printContainerDetail(cmd, detail)
		},
	}
}

func printContainerDetail(cmd *cobra.Command, d gen.ContainerDetail) error {
	w := cmd.OutOrStdout()

	memoryLimit := "unlimited"
	if d.MemoryLimit > 0 {
		memoryLimit = output.FormatBytes(d.MemoryLimit)
	}

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"ID", d.Id},
		{"NAME", d.Name},
		{"DEVICE", d.Device},
		{"STATUS", string(d.Status)},
		{"IMAGE", d.Image},
		{"RESTART COUNT", fmt.Sprintf("%d", d.RestartCount)},
		{"CPU", fmt.Sprintf("%.1f%%", d.Resources.CpuPercent)},
		{"MEMORY", fmt.Sprintf("%s (%.1f%%)", output.FormatBytes(d.Resources.MemoryBytes), d.Resources.MemoryPercent)},
		{"STARTED AT", output.FormatTime(d.StartedAt)},
		{"EXIT CODE", fmt.Sprintf("%d", d.ExitCode)},
		{"OOM KILLED", fmt.Sprintf("%v", d.OomKilled)},
		{"RESTART POLICY", string(d.RestartPolicy)},
		{"PRIVILEGED", fmt.Sprintf("%v", d.Privileged)},
		{"MEMORY LIMIT", memoryLimit},
	}
	if err := output.Print(w, output.FormatTable, nil, headers, rows); err != nil {
		return err
	}

	if len(d.PortBindings) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "PORT BINDINGS")
		var pbRows [][]string
		for _, pb := range d.PortBindings {
			pbRows = append(pbRows, []string{
				fmt.Sprintf("%d", pb.ContainerPort),
				fmt.Sprintf("%d", pb.HostPort),
				string(pb.Protocol),
			})
		}
		if err := output.Print(w, output.FormatTable, nil, []string{"CONTAINER PORT", "HOST PORT", "PROTOCOL"}, pbRows); err != nil {
			return err
		}
	}

	if len(d.Networks) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "NETWORKS")
		var netRows [][]string
		for _, n := range d.Networks {
			netRows = append(netRows, []string{n.Name, n.Driver})
		}
		if err := output.Print(w, output.FormatTable, nil, []string{"NAME", "DRIVER"}, netRows); err != nil {
			return err
		}
	}

	if len(d.VolumeBindings) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "VOLUME BINDINGS")
		var volRows [][]string
		for _, v := range d.VolumeBindings {
			volRows = append(volRows, []string{v.Source, v.Destination, string(v.Mode)})
		}
		if err := output.Print(w, output.FormatTable, nil, []string{"SOURCE", "DESTINATION", "MODE"}, volRows); err != nil {
			return err
		}
	}

	if len(d.EnvVariables) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "ENVIRONMENT VARIABLES")
		var envRows [][]string
		for _, e := range d.EnvVariables {
			envRows = append(envRows, []string{e.Key, e.Value})
		}
		if err := output.Print(w, output.FormatTable, nil, []string{"KEY", "VALUE"}, envRows); err != nil {
			return err
		}
	}

	if len(d.Entrypoint) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "ENTRYPOINT")
		for _, e := range d.Entrypoint {
			fmt.Fprintln(w, " ", e)
		}
	}

	if len(d.Cmd) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "COMMAND")
		for _, c := range d.Cmd {
			fmt.Fprintln(w, " ", c)
		}
	}

	if d.Labels != nil && len(*d.Labels) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "LABELS")
		var labelRows [][]string
		for k, v := range *d.Labels {
			labelRows = append(labelRows, []string{k, v})
		}
		sort.Slice(labelRows, func(i, j int) bool {
			return labelRows[i][0] < labelRows[j][0]
		})
		if err := output.Print(w, output.FormatTable, nil, []string{"KEY", "VALUE"}, labelRows); err != nil {
			return err
		}
	}

	return nil
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
			resp, err := c.StartContainer(context.Background(), args[0], &gen.StartContainerParams{})
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusNoContent {
				return apiclient.ParseError(resp)
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
			resp, err := c.StopContainer(context.Background(), args[0], &gen.StopContainerParams{})
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusNoContent {
				return apiclient.ParseError(resp)
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
			resp, err := c.RestartContainer(context.Background(), args[0], &gen.RestartContainerParams{})
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusNoContent {
				return apiclient.ParseError(resp)
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

			resp, err := c.ListDockerNetworks(context.Background(), params)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return apiclient.ParseError(resp)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			var list gen.DockerNetworkList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"ID", "NAME", "DEVICE", "CONTAINERS"}
			var rows [][]string
			for _, n := range list.Items {
				rows = append(rows, []string{
					n.Id, n.Name, n.Device,
					fmt.Sprintf("%d", n.ConnectedContainers),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
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

			resp, err := c.GetDockerNetwork(context.Background(), args[0])
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return apiclient.ParseError(resp)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			var detail gen.DockerNetworkDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			return printNetworkDetail(cmd, detail)
		},
	}
}

func printNetworkDetail(cmd *cobra.Command, d gen.DockerNetworkDetail) error {
	w := cmd.OutOrStdout()

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"ID", d.Id},
		{"NAME", d.Name},
		{"DEVICE", d.Device},
		{"DRIVER", d.Driver},
		{"CONTAINERS", fmt.Sprintf("%d", d.ConnectedContainers)},
	}
	// subnet and gateway are optional (no IPAM on host/macvlan networks)
	if d.Subnet != nil {
		rows = append(rows, []string{"SUBNET", *d.Subnet})
	}
	if d.Gateway != nil {
		rows = append(rows, []string{"GATEWAY", *d.Gateway})
	}
	if err := output.Print(w, output.FormatTable, nil, headers, rows); err != nil {
		return err
	}

	if len(d.Containers) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "CONNECTED CONTAINERS")
		var rows [][]string
		for _, name := range d.Containers {
			rows = append(rows, []string{name})
		}
		if err := output.Print(w, output.FormatTable, nil, []string{"NAME"}, rows); err != nil {
			return err
		}
	}

	return nil
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

			resp, err := c.ListDockerImages(context.Background(), params)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return apiclient.ParseError(resp)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			var list gen.DockerImageList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"ID", "DEVICE", "REPOSITORY", "TAGS", "SIZE"}
			var rows [][]string
			for _, img := range list.Items {
				rows = append(rows, []string{
					img.Id,
					img.Device,
					img.Repository,
					strings.Join(img.Tags, ", "),
					output.FormatBytes(img.Size),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
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

			resp, err := c.GetDockerImage(context.Background(), args[0])
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return apiclient.ParseError(resp)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			var detail gen.DockerImageDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			return printImageDetail(cmd, detail)
		},
	}
}

func printImageDetail(cmd *cobra.Command, d gen.DockerImageDetail) error {
	w := cmd.OutOrStdout()

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"ID", d.Id},
		{"DEVICE", d.Device},
		{"REPOSITORY", d.Repository},
		{"TAGS", strings.Join(d.Tags, ", ")},
		{"SIZE", output.FormatBytes(d.Size)},
		{"VIRTUAL SIZE", output.FormatBytes(d.VirtualSize)},
	}
	if !d.Created.IsZero() {
		rows = append(rows, []string{"CREATED", output.FormatTime(d.Created)})
	}
	return output.Print(w, output.FormatTable, nil, headers, rows)
}

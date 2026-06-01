package docker

import (
	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "docker", Short: "Docker resources"}
	cmd.AddCommand(newContainersCmd(), newNetworksCmd(), newImagesCmd())
	return cmd
}

func buildClient() (DockerClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewDockerClient(httpClient, apiURL)
}

package storage

import (
	"github.com/bwilczynski/hlctl/internal/api"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "storage", Short: "NAS storage resources"}
	cmd.AddCommand(newVolumesCmd(), newBackupsCmd())
	return cmd
}

func buildClient() (StorageClient, error) {
	httpClient, apiURL, err := api.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewStorageClient(httpClient, apiURL)
}

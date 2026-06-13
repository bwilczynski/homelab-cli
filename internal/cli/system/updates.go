package system

import (
	systemapi "github.com/bwilczynski/hlctl/internal/api/system"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
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

func newUpdatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "updates",
		Short: "Software update tracking",
	}
	cmd.AddCommand(newListUpdatesCmd(), newGetUpdateCmd(), newCheckUpdatesCmd())
	return cmd
}

func newListUpdatesCmd() *cobra.Command {
	var status, updateType string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tracked software updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := &systemapi.ListSystemUpdatesParams{}
			if status != "" {
				s := systemapi.UpdateStatusFilter(status)
				params.Status = &s
			}
			if updateType != "" {
				ut := systemapi.UpdateTypeFilter(updateType)
				params.Type = &ut
			}

			resp, err := cmdutil.Client[SystemClient](cmd).ListSystemUpdatesWithResponse(cmd.Context(), params)
			if err != nil {
				return err
			}
			return updatesListView.Render(cmd.OutOrStdout(), flags.GetOutputFormat(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by update status (unknown, upToDate, updateAvailable)")
	cmd.Flags().StringVar(&updateType, "type", "", "Filter by component type (container)")
	return cmd
}

func newGetUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <update-id>",
		Short: "Show update details for a tracked component",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[SystemClient](cmd).GetSystemUpdateWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return updateGetView.Render(cmd.OutOrStdout(), flags.GetOutputFormat(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
}

func newCheckUpdatesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Force check for upstream updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[SystemClient](cmd).CheckSystemUpdatesWithResponse(cmd.Context(), &systemapi.CheckSystemUpdatesParams{})
			if err != nil {
				return err
			}
			return updatesListView.Render(cmd.OutOrStdout(), flags.GetOutputFormat(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
}

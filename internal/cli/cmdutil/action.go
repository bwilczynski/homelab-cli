package cmdutil

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/spf13/cobra"
)

// ActionCmd builds a Cobra command of the form `<verb> <id>` that calls exec
// with the resolved client and the positional id, asserts a 204 No Content
// response, and prints "<id> <pastTense>" on success.
func ActionCmd[C any](use, short, pastTense string, exec func(c C, ctx context.Context, id string) (int, []byte, error)) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			code, body, err := exec(Client[C](cmd), cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if code != http.StatusNoContent {
				return apiclient.ParseError(code, body)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", args[0], pastTense)
			return nil
		},
	}
}

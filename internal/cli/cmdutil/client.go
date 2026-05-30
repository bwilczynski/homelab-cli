package cmdutil

import (
	"context"

	"github.com/spf13/cobra"
)

type clientKey[C any] struct{}

// InjectClient registers a PersistentPreRunE on cmd that builds a client and
// stores it on the executing command's context. Leaf commands retrieve it via
// Client[C]. The PreRunE fires after flags are parsed and only when a real
// subcommand runs (not on --help/--version), so flag-dependent construction
// and disk I/O stay deferred until actually needed.
func InjectClient[C any](cmd *cobra.Command, build func() (C, error)) {
	cmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		c, err := build()
		if err != nil {
			return err
		}
		cmd.SetContext(context.WithValue(cmd.Context(), clientKey[C]{}, c))
		return nil
	}
}

// Client returns the client previously injected for type C. Panics if no
// client is set — callers should always run under an InjectClient ancestor
// or after a SetClient call.
func Client[C any](cmd *cobra.Command) C {
	return cmd.Context().Value(clientKey[C]{}).(C)
}

// SetClient layers a client value onto cmd's existing context. Intended for
// tests that exercise a leaf command directly (without its real parent's
// PersistentPreRunE chain). Preserves any pre-existing context values.
func SetClient[C any](cmd *cobra.Command, c C) {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	cmd.SetContext(context.WithValue(ctx, clientKey[C]{}, c))
}

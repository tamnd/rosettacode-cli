package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) searchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search Rosetta Code tasks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n := a.effectiveLimit(10)
			a.progressf("searching for %q (limit %d)...", args[0], n)
			results, err := a.client.Search(cmd.Context(), args[0], n)
			if err != nil {
				return codeError(exitError, err)
			}
			return a.renderOrEmpty(results, len(results))
		},
	}
}

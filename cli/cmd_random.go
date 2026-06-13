package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) randomCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "random",
		Short: "Show random Rosetta Code task pages",
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(5)
			a.progressf("fetching %d random pages...", n)
			tasks, err := a.client.Random(cmd.Context(), n)
			if err != nil {
				return codeError(exitError, err)
			}
			return a.renderOrEmpty(tasks, len(tasks))
		},
	}
}

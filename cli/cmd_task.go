package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) taskCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "task <title>",
		Short: "Show a Rosetta Code task (stripped wikitext)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := args[0]
			a.progressf("fetching task %q...", title)
			task, err := a.client.Task(cmd.Context(), title)
			if err != nil {
				return codeError(exitError, err)
			}
			return a.render(task)
		},
	}
}

package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) tasksCmd() *cobra.Command {
	var lang string
	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "List programming tasks on Rosetta Code",
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(30)
			if lang != "" {
				a.progressf("fetching tasks with %s solutions (limit %d)...", lang, n)
			} else {
				a.progressf("fetching programming tasks (limit %d)...", n)
			}
			tasks, err := a.client.Tasks(cmd.Context(), n, lang)
			if err != nil {
				return codeError(exitError, err)
			}
			return a.renderOrEmpty(tasks, len(tasks))
		},
	}
	cmd.Flags().StringVar(&lang, "lang", "", "filter by language category (e.g. Go, Python)")
	return cmd
}

package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) langsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "langs",
		Short: "List programming languages available on Rosetta Code",
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(30)
			a.progressf("fetching languages (limit %d)...", n)
			langs, err := a.client.Langs(cmd.Context(), n)
			if err != nil {
				return codeError(exitError, err)
			}
			return a.renderOrEmpty(langs, len(langs))
		},
	}
}

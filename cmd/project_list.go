package cmd

import (
	"github.com/spf13/cobra"
	"qovery.go/api"
	"qovery.go/util"
)

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects",
	Long: `LIST show all available projects. For example:

	qovery project list`,

	Run: func(cmd *cobra.Command, args []string) {
		table := util.GetTable()
		table.SetHeader([]string{"name"})

		projects := api.ListProjects()

		if len(projects.Results) == 0 {
			table.Append([]string{""})
		} else {
			for _, p := range projects.Results {
				table.Append([]string{p.Name})
			}
		}

		table.Render()
	},
}

func init() {
	projectCmd.AddCommand(projectListCmd)
}

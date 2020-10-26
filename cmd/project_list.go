package cmd

import (
	"github.com/spf13/cobra"
	"qovery.go/io"
)

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects",
	Long: `LIST show all available projects. For example:

	qovery project list`,

	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, false, false, false)

		table := io.GetTable()
		table.SetHeader([]string{"name", "organization"})

		projects := io.ListProjects(OrganizationName)

		if len(projects.Results) == 0 {
			table.Append([]string{"", ""})
		} else {
			for _, p := range projects.Results {
				table.Append([]string{p.Name, p.Organization.DisplayName})
			}
		}

		table.Render()
	},
}

func init() {
	projectListCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	projectCmd.AddCommand(projectListCmd)
}

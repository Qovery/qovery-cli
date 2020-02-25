package cmd

import (
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
)

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects",
	Long: `LIST show all available projects. For example:

	qovery project list`,

	Run: func(cmd *cobra.Command, args []string) {
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"name", "region"})
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetHeaderColor(
			tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
			tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor})

		projects := api.ListProjects()

		if len(projects.Results) == 0 {
			table.Append([]string{"", ""})
		} else {
			for _, p := range projects.Results {
				table.Append([]string{p.Name, p.CloudProviderRegion.FullName})
			}
		}

		table.Render()
	},
}

func init() {
	projectCmd.AddCommand(projectListCmd)
}

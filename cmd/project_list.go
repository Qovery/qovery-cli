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
		table.SetAutoWrapText(false)
		table.SetAutoFormatHeaders(true)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetCenterSeparator("")
		table.SetColumnSeparator("")
		table.SetRowSeparator("")
		table.SetHeaderLine(false)
		table.SetBorder(false)
		table.SetTablePadding("\t")
		table.SetNoWhiteSpace(true)

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

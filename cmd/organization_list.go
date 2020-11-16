package cmd

import (
	"github.com/spf13/cobra"
	"qovery.go/io"
)

var organizationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organizations",
	Long: `LIST show all available organizations. For example:

	qovery organization list`,

	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, false, false, false, false)

		table := io.GetTable()
		table.SetHeader([]string{"name"})

		orgs := io.ListOrganizations()

		if len(orgs.Results) == 0 {
			println("User is not part of any organization. ")
			return
		} else {
			for _, p := range orgs.Results {
				table.Append([]string{p.DisplayName})
			}
		}

		table.Render()
	},
}

func init() {
	organizationCmd.AddCommand(organizationListCmd)
}

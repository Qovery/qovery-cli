package cmd

import (
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
	"strings"
)

var environmentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environments",
	Long: `LIST show all available environments. For example:

	qovery environment list`,

	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			ProjectName = util.CurrentQoveryYML().Application.Project

			if ProjectName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(1)
			}
		}
		aggEnvs := api.ListBranches(api.GetProjectByName(ProjectName).Id)

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"branches", "status", "endpoints", "applications", "databases", "brokers", "storage"})
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetHeaderColor(
			tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
			tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
			tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
			tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
			tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
			tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
			tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor})

		if aggEnvs.Results == nil || len(aggEnvs.Results) == 0 {
			table.Append([]string{"", "", "", "", "", "", ""})
		} else {
			for _, a := range aggEnvs.Results {
				//output = append(output,
				table.Append([]string{
						a.BranchId,
						a.Status.CodeMessage,
						strings.Join(a.ConnectionURIs, ", "),
						intPointerValue(a.TotalApplications),
						intPointerValue(a.TotalDatabases),
						intPointerValue(a.TotalBrokers),
						intPointerValue(a.TotalStorage),
				})
			}
		}
		table.Render()
	},
}

func init() {
	environmentListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	environmentCmd.AddCommand(environmentListCmd)
}

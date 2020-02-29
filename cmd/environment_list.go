package cmd

import (
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
			qoveryYML, err := util.CurrentQoveryYML()
			if err != nil {
				util.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
		}
		aggEnvs := api.ListBranches(api.GetProjectByName(ProjectName).Id)

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"branches", "status", "endpoints", "applications", "databases", "brokers", "storage"})
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

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

var environmentStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Environment status",
	Long: `STATUS show an environment status. For example:

	qovery environment status`,

	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = util.CurrentBranchName()
			ProjectName = util.CurrentQoveryYML().Application.Project

			if BranchName == "" || ProjectName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(1)
			}
		}

		ShowEnvironmentStatus(ProjectName, BranchName)
	},
}

func init() {
	environmentStatusCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	environmentStatusCmd.PersistentFlags().StringVarP(&BranchName, "environment", "e", "", "Your environment name")

	environmentCmd.AddCommand(environmentStatusCmd)
}

func ShowEnvironmentStatus(projectName string, branchName string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"branches name", "status", "endpoints", "applications", "databases", "brokers", "storage"})
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetHeaderColor(
		tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
		tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
		tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
		tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
		tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
		tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
		tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor})

	a := api.GetBranchByName(api.GetProjectByName(projectName).Id, branchName)
	if a.BranchId == "" {
		table.Append([]string{"", "", "", "", "", "", ""})
	} else {
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

	table.Render()
	fmt.Printf("\n")
}

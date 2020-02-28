package cmd

import (
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
)

var databaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "List databases",
	Long: `LIST show all available databases within a project and environment. For example:

	qovery database list`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = util.CurrentBranchName()
			qoveryYML, err := util.CurrentQoveryYML()
			if err != nil {
				util.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
		}

		ShowDatabaseList(ProjectName, BranchName)
	},
}

func init() {
	databaseListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	databaseListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	databaseCmd.AddCommand(databaseListCmd)
}

func ShowDatabaseList(projectName string, branchName string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"databases name", "status", "types", "versions", "endpoints", "ports", "username", "passwords", "applications"})
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorders(tablewriter.Border{Left: false, Top: true, Right: false, Bottom: true})
	table.SetHeaderColor(
		tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
		tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
		tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
		tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
		tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
		tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
		tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
		tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor},
		tablewriter.Colors{tablewriter.BgMagentaColor, tablewriter.FgWhiteColor})

	services := api.ListDatabases(api.GetProjectByName(projectName).Id, branchName)
	if services.Results == nil || len(services.Results) == 0 {
		table.Append([]string{"", "", "", "", "", "", "", "", ""})
	} else {
		for _, a := range services.Results {
			applicationName := "none"
			if a.Application != nil {
				applicationName = a.Application.Name
			}

			table.Append([]string{
				a.Name,
				a.Status.CodeMessage,
				a.Type,
				a.Version,
				a.FQDN,
				intPointerValue(a.Port),
				a.Username,
				a.Password,
				applicationName,
			})
		}
	}
	table.Render()
	fmt.Printf("\n")
}

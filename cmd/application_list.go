package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
)

var applicationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List applications",
	Long: `LIST show all available applications within a project and environment. For example:

	qovery application list`,
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

		ShowApplicationList(ProjectName, BranchName)
	},
}

func init() {
	applicationListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	applicationListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	applicationCmd.AddCommand(applicationListCmd)
}

func ShowApplicationList(projectName string, branchName string) {
	table := GetTable()
	table.SetHeader([]string{"application name", "status", "endpoint", "databases", "brokers", "storage"})

	applications := api.ListApplications(api.GetProjectByName(projectName).Id, branchName)
	if applications.Results == nil || len(applications.Results) == 0 {
		table.Append([]string{"", "", "", "", "", ""})
	} else {
		for _, a := range applications.Results {
			table.Append([]string{
				a.Name,
				a.Status.GetColoredCodeMessage(),
				a.ConnectionURI,
				intPointerValue(a.TotalDatabases),
				intPointerValue(a.TotalBrokers),
				intPointerValue(a.TotalStorage),
			})
		}
	}

	table.Render()
	fmt.Printf("\n")
}

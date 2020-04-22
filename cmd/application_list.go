package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
	"strings"
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
	table := util.GetTable()
	table.SetHeader([]string{"application name", "status", "databases"})

	projectId := api.GetProjectByName(projectName).Id
	environment := api.GetEnvironmentByName(projectId, branchName)

	applications := api.ListApplications(projectId, environment.Id)
	if applications.Results == nil || len(applications.Results) == 0 {
		table.Append([]string{"", "", ""})
	} else {
		for _, a := range applications.Results {
			databaseName := "none"
			if a.Databases != nil {
				databaseName = strings.Join(a.GetDatabaseNames(), ", ")
			}

			table.Append([]string{
				a.Name,
				a.Status.GetColoredCodeMessage(),
				databaseName,
			})
		}
	}

	table.Render()
	fmt.Printf("\n")
}

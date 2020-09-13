package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/io"
	"strings"
)

var applicationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List applications",
	Long: `LIST show all available applications within a project and environment. For example:

	qovery application list`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = io.CurrentBranchName()
			qoveryYML, err := io.CurrentQoveryYML()
			if err != nil {
				io.PrintError("No qovery configuration file found")
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
	table := io.GetTable()
	table.SetHeader([]string{"application name", "status", "databases"})

	projectId := io.GetProjectByName(projectName).Id
	environment := io.GetEnvironmentByName(projectId, branchName)

	applications := io.ListApplications(projectId, environment.Id)
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
				a.Status.GetColoredStatus(),
				databaseName,
			})
		}
	}

	table.Render()
	fmt.Printf("\n")
}

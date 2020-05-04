package cmd

import (
	"fmt"
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
			qoveryYML, err := util.CurrentQoveryYML()
			if err != nil {
				util.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
		}

		ShowEnvironmentStatus(ProjectName, BranchName)
	},
}

func init() {
	environmentStatusCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	environmentStatusCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	environmentCmd.AddCommand(environmentStatusCmd)
}

func ShowEnvironmentStatus(projectName string, branchName string) bool {
	table := util.GetTable()
	table.SetHeader([]string{"branch name", "status", "endpoints", "applications", "databases"})

	result := false
	a := api.GetEnvironmentByName(api.GetProjectByName(projectName).Id, branchName)

	if a.Name == "" {
		table.Append([]string{"", "", "", "", "", ""})
	} else {
		applicationName := "none"
		if a.Applications != nil {
			applicationName = strings.Join(a.GetApplicationNames(), ", ")
		}

		databaseName := "none"
		if a.Databases != nil {
			databaseName = strings.Join(a.GetDatabaseNames(), ", ")
		}

		table.Append([]string{
			a.Name,
			a.Status.GetColoredCodeMessage(),
			strings.Join(a.GetConnectionURIs(), "\n"),
			applicationName,
			databaseName,
		})

		result = true
	}

	table.Render()
	fmt.Printf("\n")

	return result
}

package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/io"
	"strings"
)

var environmentStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Environment status",
	Long: `STATUS show an environment status. For example:

	qovery environment status`,

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

		ShowEnvironmentStatusWithProjectAndBranchNames(ProjectName, BranchName)
	},
}

func init() {
	environmentStatusCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	environmentStatusCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	environmentCmd.AddCommand(environmentStatusCmd)
}

func ShowEnvironmentStatusWithProjectAndBranchNames(projectName string, branchName string) bool {
	environment := io.GetEnvironmentByName(io.GetProjectByName(projectName).Id, branchName)
	return ShowEnvironmentStatus(environment)
}

func ShowEnvironmentStatus(environment io.Environment) bool {
	table := io.GetTable()
	table.SetHeader([]string{"branch name", "status", "endpoints", "applications", "databases"})

	result := false

	if environment.Name == "" {
		table.Append([]string{"", "", "", "", "", ""})
	} else {
		applicationName := "none"
		if environment.Applications != nil {
			applicationName = strings.Join(environment.GetApplicationNames(), ", ")
		}

		databaseName := "none"
		if environment.Databases != nil {
			databaseName = strings.Join(environment.GetDatabaseNames(), ", ")
		}

		endpoints := strings.Join(environment.GetConnectionURIs(), "\n")
		if endpoints == "" {
			endpoints = "none"
		}

		table.Append([]string{
			environment.Name,
			environment.Status.GetColoredStatus(),
			endpoints,
			applicationName,
			databaseName,
		})

		result = true
	}

	table.Render()
	fmt.Printf("\n")

	return result
}

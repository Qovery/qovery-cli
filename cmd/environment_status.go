package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"qovery-cli/io"
	"strings"
)

var environmentStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Environment status",
	Long: `STATUS show an environment status. For example:

	qovery environment status`,

	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, true, false)
		ShowEnvironmentStatusWithProjectAndBranchNames(OrganizationName, ProjectName, BranchName)
	},
}

func init() {
	environmentStatusCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	environmentStatusCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	environmentStatusCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	environmentCmd.AddCommand(environmentStatusCmd)
}

func ShowEnvironmentStatusWithProjectAndBranchNames(organizationName string, projectName string, branchName string) bool {
	environment := io.GetEnvironmentByName(io.GetProjectByName(projectName, organizationName).Id, branchName)
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

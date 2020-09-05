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

		ShowEnvironmentStatus(ProjectName, BranchName)
	},
}

func init() {
	environmentStatusCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	environmentStatusCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	environmentCmd.AddCommand(environmentStatusCmd)
}

func ShowEnvironmentStatus(projectName string, branchName string) bool {
	table := io.GetTable()
	table.SetHeader([]string{"branch name", "status", "endpoints", "applications", "databases"})

	result := false
	e := io.GetEnvironmentByName(io.GetProjectByName(projectName).Id, branchName)

	if e.Name == "" {
		table.Append([]string{"", "", "", "", "", ""})
	} else {
		applicationName := "none"
		if e.Applications != nil {
			applicationName = strings.Join(e.GetApplicationNames(), ", ")
		}

		databaseName := "none"
		if e.Databases != nil {
			databaseName = strings.Join(e.GetDatabaseNames(), ", ")
		}

		endpoints := strings.Join(e.GetConnectionURIs(), "\n")
		if endpoints == "" {
			endpoints = "none"
		}

		table.Append([]string{
			e.Name,
			e.Status.GetColoredCodeMessage(),
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

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

func ShowEnvironmentStatus(projectName string, branchName string) {
	table := util.GetTable()
	table.SetHeader([]string{"branch name", "status", "endpoints", "applications", "databases"})

	a := api.GetEnvironmentByName(api.GetProjectByName(projectName).Id, branchName)
	if a.Name == "" {
		table.Append([]string{"", "", "", "", "", ""})
	} else {
		table.Append([]string{
			a.Name,
			a.Status.GetColoredCodeMessage(),
			strings.Join(a.GetConnectionURIs(), ", "),
			intPointerValue(a.TotalApplications),
			intPointerValue(a.TotalDatabases),
		})
	}

	table.Render()
	fmt.Printf("\n")
}

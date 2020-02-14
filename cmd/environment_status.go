package cmd

import (
	"fmt"
	"github.com/ryanuber/columnize"
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
	output := []string{
		"branch | status | endpoints | applications | databases | brokers | storage",
	}

	a := api.GetBranchByName(api.GetProjectByName(projectName).Id, branchName)
	if a.BranchId == "" {
		fmt.Println(columnize.SimpleFormat(output))
		return
	}
	output = append(output, strings.Join([]string{
		a.BranchId,
		a.Status.CodeMessage,
		strings.Join(a.ConnectionURIs, ", "),
		intPointerValue(a.TotalApplications),
		intPointerValue(a.TotalDatabases),
		intPointerValue(a.TotalBrokers),
		intPointerValue(a.TotalStorage),
	}, " | "))

	fmt.Println(columnize.SimpleFormat(output))
}

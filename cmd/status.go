package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/util"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status from current project and environment",
	Long: `STATUS show status from current project and environment. For example:

	qovery status`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = util.CurrentBranchName()
			ProjectName = util.CurrentQoveryYML().Application.Project

			if BranchName == "" || ProjectName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(1)
			}
		}

		fmt.Println("Environment")
		ShowEnvironmentStatus(ProjectName, BranchName)
		fmt.Println("\nApplications")
		ShowApplicationList(ProjectName, BranchName)
		fmt.Println("\nDatabases")
		ShowDatabaseList(ProjectName, BranchName)
		fmt.Println("\nBrokers")
		ShowBrokerList(ProjectName, BranchName)
		fmt.Println("\nStorage")
		ShowStorageList(ProjectName, BranchName)
	},
}

func init() {
	statusCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	statusCmd.PersistentFlags().StringVarP(&BranchName, "environment", "e", "", "Your environment name")

	RootCmd.AddCommand(statusCmd)
}

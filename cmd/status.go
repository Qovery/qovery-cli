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
			EnvironmentName = util.CurrentBranchName()
			ProjectName = util.CurrentQoveryYML().Application.Project

			if EnvironmentName == "" || ProjectName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(0)
			}
		}

		// TODO API call

	},
}

func init() {
	statusCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	statusCmd.PersistentFlags().StringVarP(&EnvironmentName, "environment", "e", "", "Your environment name")

	RootCmd.AddCommand(statusCmd)
}

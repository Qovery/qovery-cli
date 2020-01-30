package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/util"
)

var applicationEnvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environment variables of an application",
	Long: `LIST show all environment variables from an application. For example:

	qovery application env list`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			qoveryYML := util.CurrentQoveryYML()
			BranchName = util.CurrentBranchName()
			ProjectName = qoveryYML.Application.Project
			ApplicationName = qoveryYML.Application.Name

			if BranchName == "" || ProjectName == "" || ApplicationName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(1)
			}
		}

		ShowEnvironmentVariablesByApplicationName(ProjectName, BranchName, ApplicationName)
	},
}

func init() {
	applicationEnvListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	applicationEnvListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	applicationEnvListCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "a", "", "Your application name")

	applicationEnvCmd.AddCommand(applicationEnvListCmd)
}

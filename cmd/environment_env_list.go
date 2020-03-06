package cmd

import (
	"github.com/spf13/cobra"
	"os"
	"qovery.go/util"
)

var environmentEnvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environment variables",
	Long: `LIST show all environment variables from an environment. For example:

	qovery environment env list`,
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

		ShowEnvironmentVariablesByBranchName(ProjectName, BranchName, ShowCredentials)
	},
}

func init() {
	environmentEnvListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	environmentEnvListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	environmentEnvListCmd.PersistentFlags().BoolVarP(&ShowCredentials, "credentials", "c", false, "Show credentials")

	environmentEnvCmd.AddCommand(environmentEnvListCmd)
}

package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/util"
)

var applicationEnvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environment variables",
	Long: `LIST show all environment variables from an application. For example:

	qovery application env list`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			qoveryYML, err := util.CurrentQoveryYML()
			if err != nil {
				util.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			BranchName = util.CurrentBranchName()
			ApplicationName = qoveryYML.Application.GetSanitizeName()
			ProjectName = qoveryYML.Application.Project

			if BranchName == "" || ProjectName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(1)
			}
		}

		ShowEnvironmentVariablesByApplicationName(ProjectName, BranchName, ApplicationName, ShowCredentials)
	},
}

func init() {
	applicationEnvListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	applicationEnvListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	applicationEnvListCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "a", "", "Your application name")
	applicationEnvListCmd.PersistentFlags().BoolVarP(&ShowCredentials, "credentials", "c", false, "Show credentials")
	// TODO select application

	applicationEnvCmd.AddCommand(applicationEnvListCmd)
}

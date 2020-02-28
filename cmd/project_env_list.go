package cmd

import (
	"github.com/spf13/cobra"
	"os"
	"qovery.go/util"
)

var projectEnvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environment variables of a project",
	Long: `LIST show all environment variables from a project. For example:

	qovery project env list`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			qoveryYML, err := util.CurrentQoveryYML()
			if err != nil {
				util.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
		}

		ShowEnvironmentVariablesByProjectName(ProjectName)
	},
}

func init() {
	projectEnvListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")

	projectEnvCmd.AddCommand(projectEnvListCmd)
}

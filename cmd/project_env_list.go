package cmd

import (
	"github.com/spf13/cobra"
	"os"
	"qovery.go/io"
)

var projectEnvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environment variables of a project",
	Long: `LIST show all environment variables from a project. For example:

	qovery project env list`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			qoveryYML, err := io.CurrentQoveryYML()
			if err != nil {
				io.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
		}

		ShowEnvironmentVariablesByProjectName(ProjectName, ShowCredentials, OutputEnvironmentVariables)
	},
}

func init() {
	projectEnvListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	projectEnvListCmd.PersistentFlags().BoolVarP(&ShowCredentials, "credentials", "c", false, "Show credentials")
	projectEnvListCmd.PersistentFlags().BoolVar(&OutputEnvironmentVariables, "dotenv", false, "Message environment variables KEY=VALUE")

	projectEnvCmd.AddCommand(projectEnvListCmd)
}

package cmd

import (
	"github.com/spf13/cobra"
)

var applicationEnvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environment variables",
	Long: `LIST show all environment variables from an application. For example:

	qovery application env list`,
	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, true, true)
		ShowEnvironmentVariablesByApplicationName(OrganizationName, ProjectName, BranchName, ApplicationName, ShowCredentials, OutputEnvironmentVariables)
	},
}

func init() {
	applicationEnvListCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	applicationEnvListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	applicationEnvListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	applicationEnvListCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "a", "", "Your application name")
	applicationEnvListCmd.PersistentFlags().BoolVarP(&ShowCredentials, "credentials", "c", false, "Show credentials")
	applicationEnvListCmd.PersistentFlags().BoolVar(&OutputEnvironmentVariables, "dotenv", false, "Message environment variables KEY=VALUE")
	applicationEnvCmd.AddCommand(applicationEnvListCmd)
}

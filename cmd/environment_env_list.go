package cmd

import (
	"github.com/spf13/cobra"
)

var environmentEnvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environment variables",
	Long: `LIST show all environment variables from an environment. For example:

	qovery environment env list`,
	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, true, false)
		ShowEnvironmentVariablesByBranchName(OrganizationName, ProjectName, BranchName, ShowCredentials, OutputEnvironmentVariables)
	},
}

func init() {
	environmentEnvListCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	environmentEnvListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	environmentEnvListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	environmentEnvListCmd.PersistentFlags().BoolVarP(&ShowCredentials, "credentials", "c", false, "Show credentials")
	environmentEnvListCmd.PersistentFlags().BoolVar(&OutputEnvironmentVariables, "dotenv", false, "Message environment variables KEY=VALUE")

	environmentEnvCmd.AddCommand(environmentEnvListCmd)
}

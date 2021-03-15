package cmd

import (
	"github.com/Qovery/qovery-cli/io"
	"github.com/spf13/cobra"
)

var environmentStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Deploy and start the current environment",
	Long: `START deploy and turn on an environment. For example:

	qovery environment start`,

	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, true, true, true)

		projectId := io.GetProjectByName(ProjectName, OrganizationName).Id
		application := io.GetApplicationByName(projectId, BranchName, ApplicationName, true)
		environment := io.GetEnvironmentByName(projectId, BranchName, true)

		io.Deploy(projectId, environment.Id, application.Id, application.Repository.CommitId)
		ShowDeploymentMessage()
	},
}

func init() {
	environmentStartCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	environmentStartCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	environmentStartCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	environmentStartCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "a", "", "Your application name")

	environmentCmd.AddCommand(environmentStartCmd)
}

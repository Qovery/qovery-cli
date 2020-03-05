package cmd

import (
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
)

var environmentStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Deploy and start the current environment",
	Long: `START deploy and turn on an environment. For example:

	qovery environment start`,

	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = util.CurrentBranchName()
			qoveryYML, err := util.CurrentQoveryYML()
			if err != nil {
				util.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
			ApplicationName = qoveryYML.Application.Name
		}

		projectId := api.GetProjectByName(ProjectName).Id
		applicationId := api.GetApplicationByName(projectId, BranchName, ApplicationName).Id

		environments := api.GetBranchByName(projectId, BranchName).Environments

		var environment api.Environment
		for _, e := range environments {
			if e.Application.Name == ApplicationName {
				environment = e
			}
		}

		api.Deploy(projectId, BranchName, applicationId, environment.CommitId)
		ShowDeploymentMessage()
	},
}

func init() {
	environmentStartCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	environmentStartCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	environmentCmd.AddCommand(environmentStartCmd)
}

package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
)

var deployCmd = &cobra.Command{
	Use:   "deploy <commit id>",
	Short: "Perform deploy actions",
	Long: `DEPLOY performs actions on deploy. For example:

	qovery deploy`,
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

		if len(args) != 1 {
			_ = cmd.Help()
			return
		}

		commitId := args[0]

		projectId := api.GetProjectByName(ProjectName).Id
		environmentId := api.GetEnvironmentByName(projectId, BranchName).Id
		applicationId := api.GetApplicationByName(ProjectName, environmentId, ApplicationName).Id

		api.Deploy(projectId, environmentId, applicationId, commitId)

		ShowDeploymentMessage()
	},
}

func init() {
	deployCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	deployCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	deployCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "a", "", "Your application name")

	RootCmd.AddCommand(deployCmd)
}

func ShowDeploymentMessage() {
	fmt.Println(color.YellowString("deployment in progress..."))
	fmt.Println("Hint: type \"qovery status --watch\" to track the progression of this deployment")
}

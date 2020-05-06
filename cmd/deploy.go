package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/io"
)

var deployCmd = &cobra.Command{
	Use:   "deploy <commit id>",
	Short: "Perform deploy actions",
	Long: `DEPLOY performs actions on deploy. For example:

	qovery deploy`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = io.CurrentBranchName()
			qoveryYML, err := io.CurrentQoveryYML()
			if err != nil {
				io.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
			ApplicationName = qoveryYML.Application.GetSanitizeName()
		}

		if len(args) != 1 {
			_ = cmd.Help()
			return
		}

		commitId := args[0]

		projectId := io.GetProjectByName(ProjectName).Id
		environmentId := io.GetEnvironmentByName(projectId, BranchName).Id
		applicationId := io.GetApplicationByName(ProjectName, environmentId, ApplicationName).Id

		io.Deploy(projectId, environmentId, applicationId, commitId)

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

package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
)

var applicationEnvDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete environment variable",
	Long: `DELETE an environment variable from an application. For example:

	qovery application env delete`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			qoveryYML, err := util.CurrentQoveryYML()
			if err != nil {
				util.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			BranchName = util.CurrentBranchName()
			ApplicationName = qoveryYML.Application.Name
			ProjectName = qoveryYML.Application.Project
		}

		if len(args) != 1 {
			_ = cmd.Help()
			return
		}

		projectId := api.GetProjectByName(ProjectName).Id
		environment := api.GetEnvironmentByName(projectId, BranchName)
		application := api.GetApplicationByName(projectId, environment.Id, ApplicationName)

		ev := api.ListApplicationEnvironmentVariables(projectId, environment.Id, application.Id).GetEnvironmentVariableByKey(args[0])
		api.DeleteApplicationEnvironmentVariable(ev.Id, projectId, environment.Id, application.Id)

		fmt.Println("ok")
	},
}

func init() {
	applicationEnvDeleteCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	applicationEnvDeleteCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	applicationEnvDeleteCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "a", "", "Your application name")
	// TODO select application

	applicationEnvCmd.AddCommand(applicationEnvDeleteCmd)
}

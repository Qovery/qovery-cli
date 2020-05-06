package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/io"
)

var applicationEnvDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete environment variable",
	Long: `DELETE an environment variable from an application. For example:

	qovery application env delete`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			qoveryYML, err := io.CurrentQoveryYML()
			if err != nil {
				io.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			BranchName = io.CurrentBranchName()
			ApplicationName = qoveryYML.Application.GetSanitizeName()
			ProjectName = qoveryYML.Application.Project
		}

		if len(args) != 1 {
			_ = cmd.Help()
			return
		}

		projectId := io.GetProjectByName(ProjectName).Id
		environment := io.GetEnvironmentByName(projectId, BranchName)
		application := io.GetApplicationByName(projectId, environment.Id, ApplicationName)

		ev := io.ListApplicationEnvironmentVariables(projectId, environment.Id, application.Id).GetEnvironmentVariableByKey(args[0])
		io.DeleteApplicationEnvironmentVariable(ev.Id, projectId, environment.Id, application.Id)

		fmt.Println(color.GreenString("ok"))
	},
}

func init() {
	applicationEnvDeleteCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	applicationEnvDeleteCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	applicationEnvDeleteCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "a", "", "Your application name")
	// TODO select application

	applicationEnvCmd.AddCommand(applicationEnvDeleteCmd)
}

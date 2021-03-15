package cmd

import (
	"fmt"
	"github.com/Qovery/qovery-cli/io"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var applicationEnvDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete environment variable",
	Long: `DELETE an environment variable from an application. For example:

	qovery application env delete`,
	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, true, true, true)

		if len(args) != 1 {
			_ = cmd.Help()
			return
		}

		projectId := io.GetProjectByName(ProjectName, OrganizationName).Id
		environment := io.GetEnvironmentByName(projectId, BranchName, true)
		application := io.GetApplicationByName(projectId, environment.Id, ApplicationName, true)

		ev := io.ListApplicationEnvironmentVariables(projectId, environment.Id, application.Id).GetEnvironmentVariableByKey(args[0])
		io.DeleteApplicationEnvironmentVariable(ev.Id, projectId, environment.Id, application.Id)

		fmt.Println(color.GreenString("ok"))
	},
}

func init() {
	applicationEnvDeleteCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	applicationEnvDeleteCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	applicationEnvDeleteCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	applicationEnvDeleteCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "a", "", "Your application name")
	// TODO select application

	applicationEnvCmd.AddCommand(applicationEnvDeleteCmd)
}

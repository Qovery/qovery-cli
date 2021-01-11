package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"qovery-cli/io"
)

var applicationEnvAddCmd = &cobra.Command{
	Use:   "add <key> <value>",
	Short: "Add environment variable",
	Long: `ADD an environment variable to an application. For example:

	qovery application env add`,
	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, true, true, true)

		if len(args) != 2 {
			_ = cmd.Help()
			return
		}

		projectId := io.GetProjectByName(ProjectName, OrganizationName).Id
		environment := io.GetEnvironmentByName(projectId, BranchName, true)
		application := io.GetApplicationByName(projectId, environment.Id, ApplicationName, true)
		io.CreateApplicationEnvironmentVariable(io.EnvironmentVariable{Key: args[0], Value: args[1]}, projectId,
			environment.Id, application.Id)

		fmt.Println(color.GreenString("ok"))
	},
}

func init() {
	applicationEnvAddCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	applicationEnvAddCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	applicationEnvAddCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "a", "", "Your application name")
	// TODO select application

	applicationEnvCmd.AddCommand(applicationEnvAddCmd)
}

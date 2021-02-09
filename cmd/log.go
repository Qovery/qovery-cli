package cmd

import (
	"github.com/spf13/cobra"
	"qovery-cli/io"
)

var logCmd = &cobra.Command{
	Use:     "log",
	Aliases: []string{"logs"},
	Short:   "Show application logs",
	Long: `LOG show all application logs within a project and environment. For example:

	qovery log`,
	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, true, true, true)
		ShowApplicationLog(OrganizationName, ProjectName, BranchName, ApplicationName, Tail, FollowFlag)
	},
}

func init() {
	logCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	logCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	logCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	logCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "a", "", "Your application name")
	logCmd.PersistentFlags().IntVar(&Tail, "tail", 0, "Specify if the logs should be streamed")
	logCmd.PersistentFlags().BoolVarP(&FollowFlag, "follow", "f", false, "Specify if the logs should be streamed")

	RootCmd.AddCommand(logCmd)
}

func ShowApplicationLog(organizationName string, projectName string, branchName string, applicationName string, lastLines int, follow bool) {
	projectId := io.GetProjectByName(projectName, organizationName).Id
	environment := io.GetEnvironmentByName(projectId, branchName, true)
	application := io.GetApplicationByName(projectId, environment.Id, applicationName, true)
	io.ListApplicationLogs(lastLines, follow, projectId, environment.Id, application.Id)
}

func ShowEnvironmentLog(organizationName string, projectName string, branchName string, lastLines int, follow bool) {
	projectId := io.GetProjectByName(projectName, organizationName).Id
	environment := io.GetEnvironmentByName(projectId, branchName, true)
	io.ListEnvironmentLogs(lastLines, follow, projectId, environment.Id)
}

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/pkg/browser"
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

		if EnvironmentFlag {
			ShowEnvironmentLog(OrganizationName, ProjectName, BranchName, Tail, FollowFlag)
			return
		}

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
	logCmd.PersistentFlags().BoolVarP(&EnvironmentFlag, "environment", "e", false, "Display logs from all apps in the environment")

	RootCmd.AddCommand(logCmd)
}

func ShowApplicationLog(organizationName string, projectName string, branchName string, applicationName string, lastLines int, follow bool) {
	project := io.GetProjectByName(projectName, organizationName)
	projectId := project.Id
	orgId := project.Organization.Id
	environment := io.GetEnvironmentByName(projectId, branchName, true)
	application := io.GetApplicationByName(projectId, environment.Id, applicationName, true)

	logUrl := "https://console.qovery.com/platform/organization/" + orgId + "/projects/" + projectId + "/" + environment.Id + "/" + application.Id + "/logs"

	io.PrintHint("Opening the logs in your browser : " + logUrl)

	browser.OpenURL(logUrl)

	io.ListApplicationLogs(lastLines, follow, projectId, environment.Id, application.Id)
}

func ShowEnvironmentLog(organizationName string, projectName string, branchName string, lastLines int, follow bool) {
	projectId := io.GetProjectByName(projectName, organizationName).Id
	environment := io.GetEnvironmentByName(projectId, branchName, true)
	io.ListEnvironmentLogs(lastLines, follow, projectId, environment.Id)
}

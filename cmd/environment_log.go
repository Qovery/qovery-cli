package cmd

import (
	"github.com/spf13/cobra"
	"qovery.go/io"
)

var environmentLogCmd = &cobra.Command{
	Use:     "environment log",
	Aliases: []string{"env log"},
	Short:   "Show environment logs",
	Long: `LOG show all environment logs within a project. For example:

	qovery environment log`,
	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, true, false)
		ShowEnvironmentLog(OrganizationName, ProjectName, BranchName, Tail, FollowFlag)
	},
}

func init() {
	logCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	logCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	logCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	logCmd.PersistentFlags().IntVar(&Tail, "tail", 100, "Specify if the logs should be streamed")
	logCmd.PersistentFlags().BoolVarP(&FollowFlag, "follow", "f", false, "Specify if the logs should be streamed")
	environmentCmd.AddCommand(environmentLogCmd)
}

func ShowEnvironmentLog(organizationName string, projectName string, branchName string, lastLines int, follow bool) {
	projectId := io.GetProjectByName(projectName, organizationName).Id
	environment := io.GetEnvironmentByName(projectId, branchName)
	io.ListEnvironmentLogs(lastLines, follow, projectId, environment.Id)
}

package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
	"time"
)

var applicationLogCmd = &cobra.Command{
	Use:   "log",
	Short: "Show application logs",
	Long: `LOG show all application logs within a project and environment. For example:

	qovery application log`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = util.CurrentBranchName()
			qoveryYML, err := util.CurrentQoveryYML()
			if err != nil {
				util.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
		}

		ShowApplicationLog(ProjectName, BranchName, Tail, FollowFlag)
	},
}

func init() {
	applicationLogCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	applicationLogCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	// TODO select application
	applicationLogCmd.PersistentFlags().IntVar(&Tail, "tail", 100, "Specify if the logs should be streamed")
	applicationLogCmd.PersistentFlags().BoolVarP(&FollowFlag, "follow", "f", false, "Specify if the logs should be streamed")

	applicationCmd.AddCommand(applicationLogCmd)
}

func ShowApplicationLog(projectName string, branchName string, lastLines int, follow bool) {
	projectId := api.GetProjectByName(projectName).Id
	repositoryId := api.GetRepositoryByCurrentRemoteURL(projectId).Id
	environment := api.GetEnvironmentByBranchId(projectId, repositoryId, branchName)

	if !follow {
		logs := api.ListApplicationLogs(lastLines, projectId, repositoryId, environment.Id, environment.Application.Id).Results

		for _, log := range logs {
			fmt.Print(log.Message)
		}

		return
	}

	var logs []api.Log
	for {
		logs = api.ListApplicationLogs(lastLines, projectId, repositoryId, environment.Id, environment.Application.Id).Results
		if len(logs) > 0 {
			break
		}
	}

	for _, log := range logs {
		fmt.Print(log.Message)
	}

	lastLog := logs[len(logs)-1]
	for {
		time.Sleep(time.Duration(1) * time.Second)
		logs = api.ListApplicationTailLogs(lastLog.Id, projectId, repositoryId, environment.Id, environment.Application.Id).Results
		if len(logs) > 0 {
			for _, log := range logs {
				fmt.Print(log.Message)
			}

			lastLog = logs[len(logs)-1]
		}
	}
}

package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/io"
	"time"
)

var applicationLogCmd = &cobra.Command{
	Use:   "log",
	Short: "Show application logs",
	Long: `LOG show all application logs within a project and environment. For example:

	qovery application log`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = io.CurrentBranchName()
			qoveryYML, err := io.CurrentQoveryYML()
			if err != nil {
				io.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ApplicationName = qoveryYML.Application.GetSanitizeName()
			ProjectName = qoveryYML.Application.Project
		}

		ShowApplicationLog(ProjectName, BranchName, ApplicationName, Tail, FollowFlag)
	},
}

func init() {
	applicationLogCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	applicationLogCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	applicationLogCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "a", "", "Your application name")
	// TODO select application
	applicationLogCmd.PersistentFlags().IntVar(&Tail, "tail", 100, "Specify if the logs should be streamed")
	applicationLogCmd.PersistentFlags().BoolVarP(&FollowFlag, "follow", "f", false, "Specify if the logs should be streamed")

	applicationCmd.AddCommand(applicationLogCmd)
}

func ShowApplicationLog(projectName string, branchName string, applicationName string, lastLines int, follow bool) {
	projectId := io.GetProjectByName(projectName).Id
	environment := io.GetEnvironmentByName(projectId, branchName)
	application := io.GetApplicationByName(projectId, environment.Id, applicationName)

	if !follow {
		logs := io.ListApplicationLogs(lastLines, projectId, environment.Id, application.Id).Results

		for _, log := range logs {
			fmt.Print(log.Message)
		}

		return
	}

	var logs []io.Log
	for {
		logs = io.ListApplicationLogs(lastLines, projectId, environment.Id, application.Id).Results
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
		logs = io.ListApplicationTailLogs(lastLog.Id, projectId, environment.Id, application.Id).Results
		if len(logs) > 0 {
			for _, log := range logs {
				fmt.Print(log.Message)
			}

			lastLog = logs[len(logs)-1]
		}
	}
}

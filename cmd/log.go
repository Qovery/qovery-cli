package cmd

import (
	"github.com/spf13/cobra"
	"os"
	"qovery.go/util"
)

var logCmd = &cobra.Command{
	Use:     "log",
	Aliases: []string{"logs"},
	Short:   "Show application logs",
	Long: `LOG show all application logs within a project and environment. For example:

	qovery log`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = util.CurrentBranchName()
			qoveryYML, err := util.CurrentQoveryYML()
			if err != nil {
				util.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ApplicationName = qoveryYML.Application.Name
			ProjectName = qoveryYML.Application.Project
		}

		ShowApplicationLog(ProjectName, BranchName, ApplicationName, Tail, FollowFlag)
	},
}

func init() {
	logCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	logCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	logCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "a", "", "Your application name")
	// TODO select application
	logCmd.PersistentFlags().IntVar(&Tail, "tail", 100, "Specify if the logs should be streamed")
	logCmd.PersistentFlags().BoolVarP(&FollowFlag, "follow", "f", false, "Specify if the logs should be streamed")

	RootCmd.AddCommand(logCmd)
}

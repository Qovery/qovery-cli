package cmd

import (
	"fmt"
	"github.com/schollz/progressbar/v2"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
	"strings"
	"time"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status from current project and environment",
	Long: `STATUS show status from current project and environment. For example:

	qovery status`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = util.CurrentBranchName()
			ProjectName = util.CurrentQoveryYML().Application.Project

			if BranchName == "" || ProjectName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(1)
			}
		}

		if WatchFlag {
			bar := progressbar.NewOptions(100, progressbar.OptionSetPredictTime(true))
			projectId := api.GetProjectByName(ProjectName).Id

			for {
				a := api.GetBranchByName(projectId, BranchName)
				_ = bar.Set(a.Status.ProgressionInPercent)
				bar.Describe(a.Status.CodeMessage)

				if a.Status.State == "LIVE" || strings.Contains(a.Status.State, "_ERROR") {
					break
				}

				time.Sleep(1 * time.Second)

			}

			a := api.GetBranchByName(projectId, BranchName)
			if a.Status.State == "LIVE" {
				fmt.Printf("\n\nYour environment is ready!\n\n")
			} else {
				fmt.Printf("\n\nSomething goes wrong:\n")
				fmt.Printf("%s\n\n", a.Status.Output)
			}

			fmt.Printf("-- status output --\n\n")
		}

		fmt.Println("Environment")
		ShowEnvironmentStatus(ProjectName, BranchName)
		fmt.Println("\nApplications")
		ShowApplicationList(ProjectName, BranchName)
		fmt.Println("\nDatabases")
		ShowDatabaseList(ProjectName, BranchName)
		fmt.Println("\nBrokers")
		ShowBrokerList(ProjectName, BranchName)
		fmt.Println("\nStorage")
		ShowStorageList(ProjectName, BranchName)
	},
}

func init() {
	statusCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	statusCmd.PersistentFlags().StringVarP(&BranchName, "environment", "e", "", "Your environment name")
	statusCmd.PersistentFlags().BoolVar(&WatchFlag, "watch", false, "Watch the progression until the environment is up and running")

	RootCmd.AddCommand(statusCmd)
}

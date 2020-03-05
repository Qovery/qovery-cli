package cmd

import (
	"fmt"
	"github.com/fatih/color"
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
			qoveryYML, err := util.CurrentQoveryYML()
			if err != nil {
				util.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
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
				fmt.Print("\n\n")
				fmt.Printf(color.GreenString("Your environment is ready!"))
				fmt.Print("\n\n")
				fmt.Printf(color.GreenString("-- status output --"))
			} else {
				fmt.Print("\n\n")
				fmt.Printf(color.RedString("Something goes wrong:"))
				fmt.Printf("\n%s\n\n", a.Status.Output)
				fmt.Printf(color.RedString("-- status output --"))
			}

			fmt.Print("\n\n")
		}

		ShowEnvironmentStatus(ProjectName, BranchName)
		ShowApplicationList(ProjectName, BranchName)
		ShowDatabaseList(ProjectName, BranchName, ShowCredentials)
		//ShowBrokerList(ProjectName, BranchName)
		//ShowStorageList(ProjectName, BranchName)
	},
}

func init() {
	statusCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	statusCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	statusCmd.PersistentFlags().BoolVarP(&ShowCredentials, "credentials", "c", false, "Show credentials")
	statusCmd.PersistentFlags().BoolVar(&WatchFlag, "watch", false, "Watch the progression until the environment is up and running")

	RootCmd.AddCommand(statusCmd)
}

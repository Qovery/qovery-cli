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

		projectId := api.GetProjectByName(ProjectName).Id

		if WatchFlag {
			bar := progressbar.NewOptions(100, progressbar.OptionSetPredictTime(true))

			for {
				a := api.GetBranchByName(projectId, BranchName)
				_ = bar.Set(a.Status.ProgressionInPercent)
				bar.Describe(a.Status.CodeMessage)

				if a.Status.State == "LIVE" || strings.Contains(a.Status.State, "_ERROR") {
					break
				}

				time.Sleep(1 * time.Second)
			}

			aggregatedEnvironment := api.GetBranchByName(projectId, BranchName)

			if aggregatedEnvironment.Status.State == "LIVE" {
				fmt.Print("\n\n")
				fmt.Printf(color.GreenString("Your environment is ready!"))
				fmt.Print("\n\n")
				fmt.Printf(color.GreenString("-- status output --"))
			}

			fmt.Print("\n\n")
		}

		ShowEnvironmentStatus(ProjectName, BranchName)
		ShowApplicationList(ProjectName, BranchName)
		ShowDatabaseList(ProjectName, BranchName, ShowCredentials)
		//ShowBrokerList(ProjectName, BranchName)
		//ShowStorageList(ProjectName, BranchName)

		aggregatedEnvironment := api.GetBranchByName(projectId, BranchName)
		if !strings.Contains(aggregatedEnvironment.Status.State, "_ERROR") {
			// no error
			return
		}

		fmt.Printf(color.RedString("Something goes wrong:"))
		showOutputErrorMessage(aggregatedEnvironment.Status.Output)

		if aggregatedEnvironment.Status.State == "BUILDING_ERROR" {
			util.PrintHint("Ensure your Dockerfile is correct. Run and test your container locally with 'qovery run'")
		}
	},
}

func showOutputErrorMessage(message string) {
	fmt.Printf("\n\n")
	fmt.Println("---------- Start of error message ----------")
	fmt.Printf("%s\n", message)
	fmt.Println("----------- End of error message -----------")
	fmt.Println()
}

func init() {
	statusCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	statusCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	statusCmd.PersistentFlags().BoolVarP(&ShowCredentials, "credentials", "c", false, "Show credentials")
	statusCmd.PersistentFlags().BoolVar(&WatchFlag, "watch", false, "Watch the progression until the environment is up and running")

	RootCmd.AddCommand(statusCmd)
}

package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/schollz/progressbar/v2"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/io"
	"time"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status from current project and environment",
	Long: `STATUS show status from current project and environment. For example:

	qovery status`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = io.CurrentBranchName()
			qoveryYML, err := io.CurrentQoveryYML()
			if err != nil {
				io.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
		}

		projectId := io.GetProjectByName(ProjectName).Id

		if WatchFlag {
			bar := progressbar.NewOptions(100, progressbar.OptionSetPredictTime(true))

			for {
				a := io.GetEnvironmentByName(projectId, BranchName)
				// _ = bar.Set(a.DeploymentStatus.ProgressionInPercent) TODO fix progress bar
				bar.Describe(a.Status.Message)

				if !a.Status.IsRunning() {
					break
				}

				time.Sleep(1 * time.Second)
			}

			aggregatedEnvironment := io.GetEnvironmentByName(projectId, BranchName)

			if aggregatedEnvironment.Status.IsTerminated() {
				fmt.Print("\n\n")
				fmt.Printf("%s", color.GreenString("Your environment is ready!"))
				fmt.Print("\n\n")
				fmt.Printf("%s", color.GreenString("-- status output --"))
			}

			fmt.Print("\n\n")
		}

		envExists := ShowEnvironmentStatus(ProjectName, BranchName)
		// if an environment exists, then show the rest
		ShowApplicationList(ProjectName, BranchName)
		ShowDatabaseList(ProjectName, BranchName, ShowCredentials)
		//ShowBrokerList(ProjectName, BranchName)
		//ShowStorageList(ProjectName, BranchName)

		if !envExists {
			// there is no environment, does the user forget to give access rights to Qovery ? Let's check
			err := false
			for _, url := range io.ListRemoteURLs() {
				gas := io.GitCheck(url)
				if !gas.HasAccess {
					err = true
					io.PrintError("Qovery can't access your repository " + url)
					io.PrintHint("Give access to Qovery to deploy your application. https://docs.qovery.com/docs/using-qovery/interface/cli")
				}
			}

			if !err {
				io.PrintHint("Push your code to deploy your application")
			}
		}

		environment := io.GetEnvironmentByName(projectId, BranchName)
		if environment.Status.IsOk() && !DeploymentOutputFlag {
			// no error
			return
		}

		deployments := io.ListDeployments(projectId, environment.Id)
		deploymentStatuses := io.ListDeploymentStatuses(projectId, environment.Id, deployments.Results[0].Id)

		if !environment.Status.IsOk() {
			fmt.Printf("%s", color.RedString("Something goes wrong:"))
		}

		showOutputErrorMessage(deploymentStatuses.Results)

		//if environment.DeploymentStatus.DeploymentStatus == "BUILDING_ERROR" {
		//	io.PrintHint("Ensure your Dockerfile is correct. Run and test your container locally with 'qovery run'")
		//}
	},
}

func showOutputErrorMessage(statuses []io.DeploymentStatus) {
	fmt.Printf("\n\n")
	fmt.Println("---------- Start of deployment output ----------")

	for _, s := range statuses {
		fmt.Printf("%s\n", s.GetColoredMessage())
	}

	fmt.Println("----------- End of deployment output -----------")
	fmt.Println()
}

func init() {
	statusCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	statusCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	statusCmd.PersistentFlags().BoolVarP(&ShowCredentials, "credentials", "c", false, "Show credentials")
	statusCmd.PersistentFlags().BoolVar(&WatchFlag, "watch", false, "Watch the progression until the environment is up and running")
	statusCmd.PersistentFlags().BoolVar(&DeploymentOutputFlag, "deployment-output", false, "Show deployment output (shown only an error occurred otherwise)")

	RootCmd.AddCommand(statusCmd)
}

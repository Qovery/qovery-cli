package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/io"
	"sort"
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
			aggregatedEnvironment := io.GetEnvironmentByName(projectId, BranchName)
			deploymentStatuses := deploymentStatusesFromLastDeployment(projectId, aggregatedEnvironment.Id)

			fmt.Printf("%s\n", color.GreenString("Environment deployment logs:"))

			for _, status := range deploymentStatuses.Results {
				printStatusMessageLine(status)
			}

			if aggregatedEnvironment.Status.IsTerminated() {
				printEndOfDeploymentMessage()
			} else if aggregatedEnvironment.Status.IsTerminatedWithError() {
				printEndOfDeploymentErrorMessage()
			} else {
				for {
					time.Sleep(3 * time.Second)
					lastStatusTime := deploymentStatuses.Results[len(deploymentStatuses.Results)-1].CreatedAt
					deploymentStatuses = deploymentStatusesFromLastDeployment(projectId, aggregatedEnvironment.Id)
					for _, status := range deploymentStatuses.Results {
						if status.CreatedAt.After(lastStatusTime) {
							printStatusMessageLine(status)
						}
					}
					aggregatedEnvironment = io.GetEnvironmentByName(projectId, BranchName)
					if aggregatedEnvironment.Status.IsTerminated() {
						printEndOfDeploymentMessage()
					} else if aggregatedEnvironment.Status.IsTerminatedWithError() {
						printEndOfDeploymentErrorMessage()
					}
				}
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

		if !WatchFlag {
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
		}

		//if environment.DeploymentStatus.DeploymentStatus == "BUILDING_ERROR" {
		//	io.PrintHint("Ensure your Dockerfile is correct. Run and test your container locally with 'qovery run'")
		//}
	},
}

func printStatusMessageLine(status io.DeploymentStatus) {
	time := fmt.Sprintf("%d-%02d-%02dT%02d:%02d:%02d",
		status.CreatedAt.Year(), status.CreatedAt.Month(), status.CreatedAt.Day(),
		status.CreatedAt.Hour(), status.CreatedAt.Minute(), status.CreatedAt.Second())
	fmt.Print(color.YellowString(time + " | "))
	fmt.Print(color.YellowString(status.Scope + " | "))
	fmt.Print(color.YellowString(status.Level + " | "))
	fmt.Println(status.Message)
}

func printEndOfDeploymentMessage() {
	fmt.Printf("%s", color.GreenString("End of environment deployment logs."))
	fmt.Print("\n\n")
	fmt.Printf("%s", color.GreenString("Your environment is ready!"))
	fmt.Print("\n\n")
	fmt.Printf("%s", color.GreenString("-- status output --"))
}

func printEndOfDeploymentErrorMessage() {
	fmt.Printf("%s", color.GreenString("End of environment deployment logs."))
	fmt.Print("\n\n")
	fmt.Printf("%s", color.GreenString("Your environment deployment has failed!"))
	fmt.Print("\n\n")
	fmt.Printf("%s", color.GreenString("-- status output --"))
}

func deploymentStatusesFromLastDeployment(projectId string, environmentId string) io.DeploymentStatuses {
	deployments := io.ListDeployments(projectId, environmentId)

	if len(deployments.Results) <= 0 {
		return io.DeploymentStatuses{Results: []io.DeploymentStatus{}}
	}

	sortChronologically(deployments)
	deploymentStatuses := io.ListDeploymentStatuses(projectId, environmentId, deployments.Results[0].Id)

	return deploymentStatuses
}

func sortChronologically(deployments io.Deployments) {
	sort.SliceStable(deployments.Results, func(i, j int) bool {
		return deployments.Results[i].CreatedAt.Unix() > deployments.Results[j].CreatedAt.Unix()
	})
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

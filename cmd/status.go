package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/io"
	"sort"
	"text/tabwriter"
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
			environment := io.GetEnvironmentByName(projectId, BranchName)
			deploymentStatuses := deploymentStatusesFromLastDeployment(projectId, environment.Id)

			fmt.Printf("%s\n\n", color.CyanString("Environment deployment logs:"))

			for _, status := range deploymentStatuses.Results {
				printStatusMessageLine(status)
			}

			if environment.Status.IsStarted() {
				printEndOfDeploymentMessage()
			} else if environment.Status.IsNotOk() {
				printEndOfDeploymentErrorMessage()
			} else {
				for {
					time.Sleep(2 * time.Second)
					if deploymentStatuses.Results == nil || len(deploymentStatuses.Results) == 0 {
						fmt.Println("no deployment logs")
						break
					}

					lastStatusTime := deploymentStatuses.Results[len(deploymentStatuses.Results)-1].CreatedAt
					deploymentStatuses = deploymentStatusesFromLastDeployment(projectId, environment.Id)
					for _, status := range deploymentStatuses.Results {
						if status.CreatedAt.After(lastStatusTime) {
							printStatusMessageLine(status)
						}
					}

					environment = io.GetEnvironmentByName(projectId, BranchName)
					if environment.Status.IsStarted() {
						printEndOfDeploymentMessage()
						break
					} else if environment.Status.IsStartError() {
						printEndOfDeploymentErrorMessage()
						break
					}
				}
			}

			fmt.Print("\n\n")
		}

		// refresh environment
		environment := io.GetEnvironmentByName(projectId, BranchName)
		envExists := ShowEnvironmentStatus(environment)

		// if an environment exists, then show the rest
		if environment.Applications != nil {
			ShowApplicationList(environment.Applications)
		}

		if environment.Databases != nil {
			ShowDatabaseList(environment.Databases, ShowCredentials)
		}

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
			if environment.Status.IsOk() && !DeploymentOutputFlag {
				// no error
				return
			}

			deployments := io.ListDeployments(projectId, environment.Id)

			if len(deployments.Results) == 0 {
				return
			}

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
		status.CreatedAt.Hour(), status.CreatedAt.Minute(), status.CreatedAt.Second(),
	)

	message := status.GetColoredMessage()

	if status.Message == "" {
		message = "<empty line>"
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 2, 1, '\t', tabwriter.AlignRight)
	_, _ = fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", time, status.Scope, status.GetColoredLevel(), message)
	_ = writer.Flush()
}

func printEndOfDeploymentMessage() {
	fmt.Println("\nEnd of environment deployment logs.")
	fmt.Printf("\n%s\n\n", color.GreenString("Your environment is ready!"))
	fmt.Println("-- status output --")
}

func printEndOfDeploymentErrorMessage() {
	fmt.Println("\nEnd of environment deployment logs.")
	fmt.Printf("\n%s\n\n", color.RedString("Your environment deployment has failed!"))
	fmt.Println("-- status output --")
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

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var lifecycleDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a lifecycle job",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if lifecycleTag != "" && lifecycleCommitId != "" {
			utils.PrintlnError(fmt.Errorf("you can't use --tag and --commit-id at the same time"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getContextResourcesId(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if !utils.IsEnvironmentInATerminalState(envId, client) {
			utils.PrintlnError(fmt.Errorf("environment id '%s' is not in a terminal state. The request is not queued and you must wait "+
				"for the end of the current operation to run your command. Try again in a few moment", envId))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		lifecycles, err := ListLifecycleJobs(envId, client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		lifecycle := utils.FindByJobName(lifecycles, lifecycleName)

		if lifecycle == nil {
			utils.PrintlnError(fmt.Errorf("lifecycle %s not found", lifecycleName))
			utils.PrintlnInfo("You can list all lifecycle jobs with: qovery lifecycle list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		docker := lifecycle.Source.Docker.Get()
		image := lifecycle.Source.Image.Get()

		var req qovery.JobDeployRequest

		if docker != nil {
			req = qovery.JobDeployRequest{
				GitCommitId: docker.GitRepository.DeployedCommitId,
			}

			if lifecycleCommitId != "" {
				req.GitCommitId = &lifecycleCommitId
			}
		} else {
			req = qovery.JobDeployRequest{
				ImageTag: image.Tag,
			}

			if lifecycleTag != "" {
				req.ImageTag = &lifecycleTag
			}
		}

		_, _, err = client.JobActionsApi.DeployJob(context.Background(), lifecycle.Id).JobDeployRequest(req).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println("Lifecycle job is deploying!")

		if watchFlag {
			utils.WatchJob(lifecycle.Id, envId, client)
		}
	},
}

func init() {
	lifecycleCmd.AddCommand(lifecycleDeployCmd)
	lifecycleDeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleDeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleDeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleDeployCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Job Name")
	lifecycleDeployCmd.Flags().StringVarP(&lifecycleCommitId, "commit-id", "c", "", "Lifecycle Commit ID")
	lifecycleDeployCmd.Flags().StringVarP(&lifecycleTag, "tag", "t", "", "Lifecycle Tag")
	lifecycleDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch lifecycle status until it's ready or an error occurs")

	_ = lifecycleDeployCmd.MarkFlagRequired("lifecycle")
}

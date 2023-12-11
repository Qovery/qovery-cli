package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/pterm/pterm"

	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
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

		if lifecycleName == "" && lifecycleNames == "" {
			utils.PrintlnError(fmt.Errorf("use either --lifecycle \"<container name>\" or --lifecycles \"<container1 name>, <container2 name>\" but not both at the same time"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if lifecycleName != "" && lifecycleNames != "" {
			utils.PrintlnError(fmt.Errorf("you can't use --lifecycle and --lifecycles at the same time"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if lifecycleTag != "" && lifecycleCommitId != "" {
			utils.PrintlnError(fmt.Errorf("you can't use --tag and --commit-id at the same time"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if lifecycleNames != "" {
			// wait until service is ready
			for {
				if utils.IsEnvironmentInATerminalState(envId, client) {
					break
				}

				utils.Println(fmt.Sprintf("Waiting for environment %s to be ready..", pterm.FgBlue.Sprintf(envId)))
				time.Sleep(5 * time.Second)
			}

			// deploy multiple services
			err := utils.DeployJobs(client, envId, lifecycleNames, lifecycleCommitId, lifecycleTag)

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			utils.Println(fmt.Sprintf("Deploying lifecycles %s in progress..", pterm.FgBlue.Sprintf(lifecycleNames)))

			if watchFlag {
				utils.WatchEnvironment(envId, "unused", client)
			}

			return
		}

		lifecycles, err := ListLifecycleJobs(envId, client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		lifecycle := utils.FindByJobName(lifecycles, lifecycleName)

		if lifecycle == nil || lifecycle.LifecycleJobResponse == nil {
			utils.PrintlnError(fmt.Errorf("lifecycle %s not found", lifecycleName))
			utils.PrintlnInfo("You can list all lifecycle jobs with: qovery lifecycle list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		var docker *qovery.BaseJobResponseAllOfSourceOneOf1Docker = nil
		if lifecycle.LifecycleJobResponse.Source.BaseJobResponseAllOfSourceOneOf1 != nil {
			docker = lifecycle.LifecycleJobResponse.Source.BaseJobResponseAllOfSourceOneOf1.Docker
		}
		
		var image *qovery.ContainerSource = nil
		if lifecycle.LifecycleJobResponse.Source.BaseJobResponseAllOfSourceOneOf != nil {
			image = lifecycle.LifecycleJobResponse.Source.BaseJobResponseAllOfSourceOneOf.Image
		}

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
				ImageTag: &image.Tag,
			}

			if lifecycleTag != "" {
				req.ImageTag = &lifecycleTag
			}
		}

		msg, err := utils.DeployService(client, envId, lifecycle.LifecycleJobResponse.Id, utils.JobType, req, watchFlag)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if msg != "" {
			utils.PrintlnInfo(msg)
			return
		}

		if watchFlag {
			utils.Println(fmt.Sprintf("Lifecycle %s deployed!", pterm.FgBlue.Sprintf(lifecycleName)))
		} else {
			utils.Println(fmt.Sprintf("Deploying lifecycle %s in progress..", pterm.FgBlue.Sprintf(lifecycleName)))
		}
	},
}

func init() {
	lifecycleCmd.AddCommand(lifecycleDeployCmd)
	lifecycleDeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleDeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleDeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleDeployCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Job Name")
	lifecycleDeployCmd.Flags().StringVarP(&lifecycleNames, "lifecycles", "", "", "Lifecycle Job Names")
	lifecycleDeployCmd.Flags().StringVarP(&lifecycleCommitId, "commit-id", "c", "", "Lifecycle Commit ID")
	lifecycleDeployCmd.Flags().StringVarP(&lifecycleTag, "tag", "t", "", "Lifecycle Tag")
	lifecycleDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch lifecycle status until it's ready or an error occurs")
}

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

var cronjobDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a cronjob",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if cronjobName == "" && cronjobNames == "" {
			utils.PrintlnError(fmt.Errorf("use either --cronjob \"<cronjob name>\" or --cronjobs \"<cron1 name>, <cron2 name>\" but not both at the same time"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if cronjobName != "" && cronjobNames != "" {
			utils.PrintlnError(fmt.Errorf("you can't use --cronjob and --cronjobs at the same time"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if cronjobTag != "" && cronjobCommitId != "" {
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

		if cronjobNames != "" {
			// wait until service is ready
			for {
				if utils.IsEnvironmentInATerminalState(envId, client) {
					break
				}

				utils.Println(fmt.Sprintf("Waiting for environment %s to be ready..", pterm.FgBlue.Sprintf(envId)))
				time.Sleep(5 * time.Second)
			}

			// deploy multiple services
			err := utils.DeployJobs(client, envId, cronjobNames, cronjobCommitId, cronjobTag)

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			utils.Println(fmt.Sprintf("Deploying cronjobs %s in progress..", pterm.FgBlue.Sprintf(cronjobNames)))

			if watchFlag {
				utils.WatchEnvironment(envId, "unused", client)
			}

			return
		}

		cronjobs, err := ListCronjobs(envId, client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		cronjob := utils.FindByJobName(cronjobs, cronjobName)

		if cronjob == nil || cronjob.CronJobResponse == nil {
			utils.PrintlnError(fmt.Errorf("cronjob %s not found", cronjobName))
			utils.PrintlnInfo("You can list all cronjobs with: qovery cronjob list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		var docker = utils.GetJobDocker(cronjob)
		var image = utils.GetJobImage(cronjob)

		var req qovery.JobDeployRequest

		if docker != nil {
			req = qovery.JobDeployRequest{
				GitCommitId: docker.GitRepository.DeployedCommitId,
			}

			if cronjobCommitId != "" {
				req.GitCommitId = &cronjobCommitId
			}
		} else {
			req = qovery.JobDeployRequest{
				ImageTag: &image.Tag,
			}

			if cronjobTag != "" {
				req.ImageTag = &cronjobTag
			}
		}

		msg, err := utils.DeployService(client, envId, cronjob.CronJobResponse.Id, utils.JobType, req, watchFlag)

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
			utils.Println(fmt.Sprintf("Cronjob %s deployed!", pterm.FgBlue.Sprintf(cronjobName)))
		} else {
			utils.Println(fmt.Sprintf("Deploying cronjob %s in progress..", pterm.FgBlue.Sprintf(cronjobName)))
		}
	},
}

func init() {
	cronjobCmd.AddCommand(cronjobDeployCmd)
	cronjobDeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobDeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobDeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cronjobDeployCmd.Flags().StringVarP(&cronjobName, "cronjob", "n", "", "Cronjob Name")
	cronjobDeployCmd.Flags().StringVarP(&cronjobNames, "cronjobs", "", "", "Cronjob Names (comma separated) (ex: --cronjobs \"cron1,cron2\")")
	cronjobDeployCmd.Flags().StringVarP(&cronjobCommitId, "commit-id", "c", "", "Cronjob Commit ID")
	cronjobDeployCmd.Flags().StringVarP(&cronjobTag, "tag", "t", "", "Cronjob Tag")
	cronjobDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch cronjob status until it's ready or an error occurs")
}

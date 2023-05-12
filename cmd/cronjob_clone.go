package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var cronjobCloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone a cronjob",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
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

		jobs, _, err := client.JobsApi.ListJobs(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		job := utils.FindByJobName(jobs.GetResults(), cronjobName)

		if job == nil {
			utils.PrintlnError(fmt.Errorf("job %s not found", cronjobName))
			utils.PrintlnInfo("You can list all jobs with: qovery job list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		sourceEnvironment, _, err := client.EnvironmentMainCallsApi.GetEnvironment(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		environments, _, err := client.EnvironmentsApi.ListEnvironment(context.Background(), sourceEnvironment.Project.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if targetEnvironmentName == "" {
			// use same env name as the source env
			targetEnvironmentName = sourceEnvironment.Name
		}

		targetEnvironment := utils.FindByEnvironmentName(environments.GetResults(), targetEnvironmentName)

		if targetEnvironment == nil {
			utils.PrintlnError(fmt.Errorf("environment %s not found", targetEnvironmentName))
			utils.PrintlnInfo("You can list all environments with: qovery environment list")
			os.Exit(1)
		}

		if targetCronjobName == "" {
			targetCronjobName = job.Name
		}

		source := qovery.JobRequestAllOfSource{
			Image:  qovery.NullableJobRequestAllOfSourceImage{},
			Docker: qovery.NullableJobRequestAllOfSourceDocker{},
		}

		if job.Source != nil && job.Source.Image.Get() != nil {
			source.Image = job.Source.Image
		}

		if job.Source != nil && job.Source.Docker.Get() != nil {
			docker := qovery.NullableJobRequestAllOfSourceDocker{}
			docker.Set(&qovery.JobRequestAllOfSourceDocker{
				DockerfilePath: job.Source.Docker.Get().DockerfilePath,
				GitRepository: &qovery.ApplicationGitRepositoryRequest{
					Url:      *job.Source.Docker.Get().GitRepository.Url,
					Branch:   job.Source.Docker.Get().GitRepository.Branch,
					RootPath: job.Source.Docker.Get().GitRepository.RootPath,
				},
			})

			source.Docker = docker
		}

		var schedule qovery.JobRequestAllOfSchedule

		if job.Schedule != nil {
			schedule = qovery.JobRequestAllOfSchedule{
				OnStart:  job.Schedule.OnStart,
				OnStop:   job.Schedule.OnStop,
				OnDelete: job.Schedule.OnDelete,
				Cronjob:  nil,
			}

			if job.Schedule.Cronjob != nil {
				schedule.Cronjob = &qovery.JobRequestAllOfScheduleCronjob{
					Arguments:   job.Schedule.Cronjob.Arguments,
					Entrypoint:  job.Schedule.Cronjob.Entrypoint,
					ScheduledAt: job.Schedule.Cronjob.ScheduledAt,
				}
			}
		}
		req := qovery.JobRequest{
			Name:               targetCronjobName,
			Description:        job.Description,
			Cpu:                &job.Cpu,
			Memory:             &job.Memory,
			MaxNbRestart:       job.MaxNbRestart,
			MaxDurationSeconds: job.MaxDurationSeconds,
			AutoPreview:        &job.AutoPreview,
			Port:               job.Port,
			Source:             &source,
			Schedule:           &schedule,
		}

		createdService, res, err := client.JobsApi.CreateJob(context.Background(), targetEnvironment.Id).JobRequest(req).Execute()

		if err != nil {
			utils.PrintlnError(err)

			bodyBytes, err := io.ReadAll(res.Body)
			if err != nil {
				return
			}

			utils.PrintlnError(fmt.Errorf("unable to clone job %s", string(bodyBytes)))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		deploymentStageId := utils.GetDeploymentStageId(client, job.Id)

		_, _, err = client.DeploymentStageMainCallsApi.AttachServiceToDeploymentStage(context.Background(), deploymentStageId, createdService.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		// clone advanced settings
		settings, _, err := client.JobConfigurationApi.GetJobAdvancedSettings(context.Background(), job.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		_, _, err = client.JobConfigurationApi.EditJobAdvancedSettings(context.Background(), createdService.Id).JobAdvancedSettings(*settings).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Cronjob %s cloned!", pterm.FgBlue.Sprintf(cronjobName)))
	},
}

func init() {
	cronjobCmd.AddCommand(cronjobCloneCmd)
	cronjobCloneCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobCloneCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobCloneCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cronjobCloneCmd.Flags().StringVarP(&cronjobName, "cronjob", "n", "", "Cronjob Name")
	cronjobCloneCmd.Flags().StringVarP(&targetEnvironmentName, "target-environment", "", "", "Target Environment Name")
	cronjobCloneCmd.Flags().StringVarP(&targetCronjobName, "target-cronjob-name", "", "", "Target Cronjob Name")

	_ = cronjobCloneCmd.MarkFlagRequired("cronjob")
}

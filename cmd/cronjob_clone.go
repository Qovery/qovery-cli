package cmd

import (
	"context"
	"fmt"
	"github.com/go-errors/errors"
	"io"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
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
		organizationId, projectId, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		job, err := getJobContextResource(client, cronjobName, envId)

		if err != nil || job == nil || job.CronJobResponse == nil {
			utils.PrintlnError(fmt.Errorf("cronjobName %s not found", cronjobName))
			utils.PrintlnInfo("You can list all jobs with: qovery job list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		targetProjectId := projectId // use same project as the source project
		if targetProjectName != "" {

			targetProjectId, err = getProjectContextResourceId(client, targetProjectName, organizationId)

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}
		}

		targetEnvironmentId := envId // use same env as the source env
		if targetEnvironmentName != "" {

			targetEnvironmentId, err = getEnvironmentContextResourceId(client, targetEnvironmentName, targetProjectId)

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}
		}

		if targetCronjobName == "" {
			// use same job name as the source job
			targetCronjobName = job.CronJobResponse.Name
		}

		req := qovery.CloneServiceRequest{
			Name:          targetCronjobName,
			EnvironmentId: targetEnvironmentId,
		}

		clonedService, res, err := client.JobsAPI.CloneJob(context.Background(), job.CronJobResponse.Id).CloneServiceRequest(req).Execute()

		if err != nil {
			// print http body error message
			if res.StatusCode != 200 {
				result, _ := io.ReadAll(res.Body)
				utils.PrintlnError(errors.Errorf("status code: %s ; body: %s", res.Status, string(result)))
			}

			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		name := ""
		if clonedService != nil {
			name = clonedService.CronJobResponse.Name
		}

		utils.Println(fmt.Sprintf("Job %s cloned!", pterm.FgBlue.Sprintf("%s", name)))
	},
}

func init() {
	cronjobCmd.AddCommand(cronjobCloneCmd)
	cronjobCloneCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobCloneCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobCloneCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cronjobCloneCmd.Flags().StringVarP(&cronjobName, "cronjob", "n", "", "Cronjob Name")
	cronjobCloneCmd.Flags().StringVarP(&targetProjectName, "target-project", "", "", "Target Project Name")
	cronjobCloneCmd.Flags().StringVarP(&targetEnvironmentName, "target-environment", "", "", "Target Environment Name")
	cronjobCloneCmd.Flags().StringVarP(&targetCronjobName, "target-cronjob-name", "", "", "Target Cronjob Name")

	_ = cronjobCloneCmd.MarkFlagRequired("cronjob")
}

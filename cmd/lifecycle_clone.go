package cmd

import (
	"context"
	"fmt"
	"github.com/go-errors/errors"
	"io"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var lifecycleCloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone a lifecycle job",
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

		job, err := getJobContextResource(client, lifecycleName, envId)

		if err != nil || job == nil || job.LifecycleJobResponse == nil {
			utils.PrintlnError(fmt.Errorf("lifecycle %s not found", lifecycleName))
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

		if targetLifecycleName == "" {
			// use same job name as the source job
			targetLifecycleName = job.LifecycleJobResponse.Name
		}

		req := qovery.CloneServiceRequest{
			Name:          targetLifecycleName,
			EnvironmentId: targetEnvironmentId,
		}

		clonedService, res, err := client.JobsAPI.CloneJob(context.Background(), job.LifecycleJobResponse.Id).CloneServiceRequest(req).Execute()

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
			name = clonedService.LifecycleJobResponse.Name
		}

		utils.Println(fmt.Sprintf("Job %s cloned!", pterm.FgBlue.Sprintf(name)))
	},
}

func init() {
	lifecycleCmd.AddCommand(lifecycleCloneCmd)
	lifecycleCloneCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleCloneCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleCloneCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleCloneCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Name")
	lifecycleCloneCmd.Flags().StringVarP(&targetProjectName, "target-project", "", "", "Target Project Name")
	lifecycleCloneCmd.Flags().StringVarP(&targetEnvironmentName, "target-environment", "", "", "Target Environment Name")
	lifecycleCloneCmd.Flags().StringVarP(&targetLifecycleName, "target-lifecycle-name", "", "", "Target Lifecycle Name")

	_ = lifecycleCloneCmd.MarkFlagRequired("lifecycle")
}

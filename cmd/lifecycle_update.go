package cmd

import (
	"fmt"
	"io"
	"os"
	"context"

	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var lifecycleUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a lifecycle",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if (lifecycleTag != "" || lifecycleImageName != "") && lifecycleBranch != "" {
			utils.PrintlnError(fmt.Errorf("you can't use --tag or --image-name with --branch at the same time"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if lifecycleTag == "" && lifecycleImageName == "" && lifecycleBranch == "" {
			utils.PrintlnError(fmt.Errorf("you must use --tag or --image-name or --branch"))
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

		lifecycles, err := ListLifecycleJobs(envId, client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		lifecycle := utils.FindByJobName(lifecycles, lifecycleName)

		if lifecycle == nil || lifecycle.LifecycleJobResponse == nil {
			utils.PrintlnError(fmt.Errorf("lifecycle %s not found", lifecycleName))
			utils.PrintlnInfo("You can list all lifecycles with: qovery lifecycle list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		var docker = utils.GetJobDocker(lifecycle)
		var image = utils.GetJobImage(lifecycle)

		if docker != nil && (lifecycleTag != "" || lifecycleImageName != "") {
			utils.PrintlnError(fmt.Errorf("you can't use --tag or --image-name with a lifecycle targetting a Dockerfile. Use --branch instead"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if image != nil && lifecycleBranch != "" {
			utils.PrintlnError(fmt.Errorf("you can't use --branch with a lifecycle targetting an image. Use --tag and/or --image-name instead"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		req := utils.ToJobRequest(*lifecycle)

		if docker != nil {
			req.Source.Docker.Get().GitRepository.Branch = &lifecycleBranch
			req.Source.Image.Set(nil)
		} else {
			if lifecycleTag != "" {
				req.Source.Image.Get().Tag = &lifecycleTag
			}
			if lifecycleImageName != "" {
				req.Source.Image.Get().ImageName = &lifecycleImageName
			}
			req.Source.Docker.Set(nil)
		}

		_, res, err := client.JobMainCallsAPI.EditJob(context.Background(), lifecycle.LifecycleJobResponse.Id).JobRequest(req).Execute()

		if err != nil {
			result, _ := io.ReadAll(res.Body)
			utils.PrintlnError(errors.Errorf("status code: %s ; body: %s", res.Status, string(result)))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Lifecycle %s updated!", pterm.FgBlue.Sprintf("%s", lifecycleName)))
	},
}

func init() {
	lifecycleCmd.AddCommand(lifecycleUpdateCmd)
	lifecycleUpdateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleUpdateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleUpdateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleUpdateCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Name")
	lifecycleUpdateCmd.Flags().StringVarP(&lifecycleBranch, "branch", "b", "", "Lifecycle Branch")
	lifecycleUpdateCmd.Flags().StringVarP(&lifecycleTag, "tag", "t", "", "Lifecycle Tag")
	lifecycleUpdateCmd.Flags().StringVarP(&lifecycleImageName, "image-name", "", "", "Lifecycle Image Name")
}

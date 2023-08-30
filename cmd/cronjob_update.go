package cmd

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"io"
	"os"
)

var cronjobUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a cronjob",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if cronjobTag != "" && cronjobBranch != "" {
			utils.PrintlnError(fmt.Errorf("you can't use --tag and --branch at the same time"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if cronjobTag == "" && cronjobBranch == "" {
			utils.PrintlnError(fmt.Errorf("you must use --tag or --branch"))
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

		cronjobs, err := ListCronjobs(envId, client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		cronjob := utils.FindByJobName(cronjobs, cronjobName)

		if cronjob == nil {
			utils.PrintlnError(fmt.Errorf("cronjob %s not found", cronjobName))
			utils.PrintlnInfo("You can list all cronjobs with: qovery cronjob list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		docker := cronjob.Source.Docker.Get()
		image := cronjob.Source.Image.Get()

		if docker != nil && cronjobTag != "" {
			utils.PrintlnError(fmt.Errorf("you can't use --tag with a cronjob targetting a Dockerfile. Use --branch instead"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if image != nil && cronjobBranch != "" {
			utils.PrintlnError(fmt.Errorf("you can't use --branch with a cronjob targetting an image. Use --tag instead"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		req := utils.ToJobRequest(*cronjob)

		if docker != nil {
			req.Source.Docker.Get().GitRepository.Branch = &cronjobBranch
			req.Source.Image.Set(nil)
		} else {
			req.Source.Image.Get().Tag = &cronjobTag
			req.Source.Docker.Set(nil)
		}

		_, res, err := client.JobMainCallsApi.EditJob(context.Background(), cronjob.Id).JobRequest(req).Execute()

		if err != nil {
			result, _ := io.ReadAll(res.Body)
			utils.PrintlnError(errors.Errorf("status code: %s ; body: %s", res.Status, string(result)))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Cronjob %s updated!", pterm.FgBlue.Sprintf(cronjobName)))
	},
}

func init() {
	cronjobCmd.AddCommand(cronjobUpdateCmd)
	cronjobUpdateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobUpdateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobUpdateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cronjobUpdateCmd.Flags().StringVarP(&cronjobName, "cronjob", "n", "", "Cronjob Name")
	cronjobUpdateCmd.Flags().StringVarP(&cronjobBranch, "branch", "b", "", "Cronjob Branch")
	cronjobUpdateCmd.Flags().StringVarP(&cronjobTag, "tag", "t", "", "Cronjob Tag")
}

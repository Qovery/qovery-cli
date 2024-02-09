package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var cronjobEnvUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update cronjob environment variable or secret",
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

		cronjobs, _, err := client.JobsAPI.ListJobs(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		cronjob := utils.FindByJobName(cronjobs.GetResults(), cronjobName)

		if cronjob == nil || cronjob.CronJobResponse == nil {
			utils.PrintlnError(fmt.Errorf("cronjob %s not found", cronjobName))
			utils.PrintlnInfo("You can list all cronjobs with: qovery cronjob list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		err = utils.UpdateEnvironmentVariable(client, utils.Key, utils.Value, cronjob.CronJobResponse.Id, utils.JobType)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Environment variable %s has been updated", pterm.FgBlue.Sprintf(utils.Key)))
	},
}

func init() {
	cronjobEnvCmd.AddCommand(cronjobEnvUpdateCmd)
	cronjobEnvUpdateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobEnvUpdateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobEnvUpdateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cronjobEnvUpdateCmd.Flags().StringVarP(&cronjobName, "cronjob", "n", "", "Cronjob Name")
	cronjobEnvUpdateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	cronjobEnvUpdateCmd.Flags().StringVarP(&utils.Value, "value", "v", "", "Environment variable or secret value")
	_ = cronjobEnvUpdateCmd.MarkFlagRequired("key")
	_ = cronjobEnvUpdateCmd.MarkFlagRequired("value")
	_ = cronjobEnvUpdateCmd.MarkFlagRequired("cronjob")
}

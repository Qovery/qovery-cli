package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var cronjobExternalSecretDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete cronjob external secret",
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

		err = utils.DeleteServiceVariable(client, cronjob.CronJobResponse.Id, utils.JobType, utils.Key)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("External secret %s has been deleted", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	cronjobExternalSecretCmd.AddCommand(cronjobExternalSecretDeleteCmd)
	cronjobExternalSecretDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobExternalSecretDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobExternalSecretDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cronjobExternalSecretDeleteCmd.Flags().StringVarP(&cronjobName, "cronjob", "n", "", "Cronjob Name")
	cronjobExternalSecretDeleteCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "External secret key")

	_ = cronjobExternalSecretDeleteCmd.MarkFlagRequired("key")
	_ = cronjobExternalSecretDeleteCmd.MarkFlagRequired("cronjob")
}

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var cronjobEnvDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete cronjob environment variable or secret",
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

		err = utils.DeleteVariable(client, cronjob.CronJobResponse.Id, utils.JobType, utils.Key)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Variable %s has been deleted", pterm.FgBlue.Sprintf(utils.Key)))
	},
}

func init() {
	cronjobEnvCmd.AddCommand(cronjobEnvDeleteCmd)
	cronjobEnvDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobEnvDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobEnvDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cronjobEnvDeleteCmd.Flags().StringVarP(&cronjobName, "cronjob", "n", "", "Cronjob Name")
	cronjobEnvDeleteCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")

	_ = cronjobEnvDeleteCmd.MarkFlagRequired("key")
	_ = cronjobEnvDeleteCmd.MarkFlagRequired("cronjob")
}

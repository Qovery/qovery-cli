package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var cronjobEnvOverrideCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Override cronjob environment variable or secret",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, projectId, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		cronjobs, _, err := client.JobsApi.ListJobs(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		cronjob := utils.FindByJobName(cronjobs.GetResults(), cronjobName)

		if cronjob == nil {
			utils.PrintlnError(fmt.Errorf("cronjob %s not found", cronjobName))
			utils.PrintlnInfo("You can list all cronjobs with: qovery cronjob list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		err = utils.CreateOverride(client, projectId, envId, cronjob.Id, utils.JobType, utils.Key, &utils.Value, utils.JobScope)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("%s has been overidden", pterm.FgBlue.Sprintf(utils.Key)))
	},
}

func init() {
	cronjobEnvOverrideCmd.AddCommand(cronjobEnvOverrideCreateCmd)
	cronjobEnvOverrideCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobEnvOverrideCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobEnvOverrideCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cronjobEnvOverrideCreateCmd.Flags().StringVarP(&cronjobName, "cronjob", "n", "", "Cronjob Name")
	cronjobEnvOverrideCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	cronjobEnvOverrideCreateCmd.Flags().StringVarP(&utils.Value, "value", "", "", "Environment variable or secret value")
	cronjobEnvOverrideCreateCmd.Flags().StringVarP(&utils.JobScope, "scope", "", "JOB", "Scope of this alias <PROJECT|ENVIRONMENT|JOB>")

	_ = cronjobEnvOverrideCreateCmd.MarkFlagRequired("key")
	_ = cronjobEnvOverrideCreateCmd.MarkFlagRequired("cronjob")
}

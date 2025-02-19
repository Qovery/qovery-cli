package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var cronjobEnvCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create cronjob environment variable or secret",
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

		err = utils.CreateServiceVariable(client, projectId, envId, cronjob.CronJobResponse.Id, utils.JobScope, utils.Key, utils.Value, utils.IsSecret)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Environment variable %s has been created", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	cronjobEnvCmd.AddCommand(cronjobEnvCreateCmd)
	cronjobEnvCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobEnvCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobEnvCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cronjobEnvCreateCmd.Flags().StringVarP(&cronjobName, "cronjob", "n", "", "Cronjob Name")
	cronjobEnvCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	cronjobEnvCreateCmd.Flags().StringVarP(&utils.Value, "value", "v", "", "Environment variable or secret value")
	cronjobEnvCreateCmd.Flags().StringVarP(&utils.JobScope, "scope", "", "JOB", "Scope of this env var <PROJECT|ENVIRONMENT|JOB>")
	cronjobEnvCreateCmd.Flags().BoolVarP(&utils.IsSecret, "secret", "", false, "This environment variable is a secret")

	_ = cronjobEnvCreateCmd.MarkFlagRequired("key")
	_ = cronjobEnvCreateCmd.MarkFlagRequired("value")
	_ = cronjobEnvCreateCmd.MarkFlagRequired("cronjob")
}

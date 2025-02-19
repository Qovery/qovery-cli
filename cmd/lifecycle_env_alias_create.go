package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var lifecycleEnvAliasCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create lifecycle environment variable or secret alias",
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

		lifecycles, _, err := client.JobsAPI.ListJobs(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		lifecycle := utils.FindByJobName(lifecycles.GetResults(), lifecycleName)

		if lifecycle == nil || lifecycle.LifecycleJobResponse == nil {
			utils.PrintlnError(fmt.Errorf("lifecycle %s not found", lifecycleName))
			utils.PrintlnInfo("You can list all lifecycles with: qovery lifecycle list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		err = utils.CreateServiceAlias(client, projectId, envId, lifecycle.LifecycleJobResponse.Id, utils.JobType, utils.Key, utils.Alias, utils.JobScope)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Alias %s has been created", pterm.FgBlue.Sprintf("%s", utils.Alias)))
	},
}

func init() {
	lifecycleEnvAliasCmd.AddCommand(lifecycleEnvAliasCreateCmd)
	lifecycleEnvAliasCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleEnvAliasCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleEnvAliasCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleEnvAliasCreateCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Name")
	lifecycleEnvAliasCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	lifecycleEnvAliasCreateCmd.Flags().StringVarP(&utils.Alias, "alias", "", "", "Environment variable or secret alias")
	lifecycleEnvAliasCreateCmd.Flags().StringVarP(&utils.JobScope, "scope", "", "JOB", "Scope of this alias <PROJECT|ENVIRONMENT|JOB>")

	_ = lifecycleEnvAliasCreateCmd.MarkFlagRequired("key")
	_ = lifecycleEnvAliasCreateCmd.MarkFlagRequired("alias")
	_ = lifecycleEnvAliasCreateCmd.MarkFlagRequired("lifecycle")
}

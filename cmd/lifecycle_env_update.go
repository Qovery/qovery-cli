package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var lifecycleEnvUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update lifecycle environment variable or secret",
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

		err = utils.UpdateEnvironmentVariable(client, utils.Key, utils.Value, lifecycle.LifecycleJobResponse.Id, utils.JobType)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Environment variable %s has been updated", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	lifecycleEnvCmd.AddCommand(lifecycleEnvUpdateCmd)
	lifecycleEnvUpdateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleEnvUpdateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleEnvUpdateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleEnvUpdateCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Name")
	lifecycleEnvUpdateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	lifecycleEnvUpdateCmd.Flags().StringVarP(&utils.Value, "value", "v", "", "Environment variable or secret value")
	_ = lifecycleEnvUpdateCmd.MarkFlagRequired("key")
	_ = lifecycleEnvUpdateCmd.MarkFlagRequired("value")
	_ = lifecycleEnvUpdateCmd.MarkFlagRequired("lifecycle")
}
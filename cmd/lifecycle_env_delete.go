package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var lifecycleEnvDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete lifecycle environment variable or secret",
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

		lifecycles, _, err := client.JobsApi.ListJobs(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		lifecycle := utils.FindByJobName(lifecycles.GetResults(), lifecycleName)

		if lifecycle == nil {
			utils.PrintlnError(fmt.Errorf("lifecycle %s not found", lifecycleName))
			utils.PrintlnInfo("You can list all lifecycles with: qovery lifecycle list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		err = utils.DeleteByKey(client, projectId, envId, lifecycle.Id, utils.JobType, utils.Key)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Variable %s has been deleted", pterm.FgBlue.Sprintf(utils.Key)))
	},
}

func init() {
	lifecycleEnvCmd.AddCommand(lifecycleEnvDeleteCmd)
	lifecycleEnvDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleEnvDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleEnvDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleEnvDeleteCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Name")
	lifecycleEnvDeleteCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")

	_ = lifecycleEnvDeleteCmd.MarkFlagRequired("key")
	_ = lifecycleEnvDeleteCmd.MarkFlagRequired("lifecycle")
}

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var lifecycleExternalSecretDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete lifecycle external secret",
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

		err = utils.DeleteServiceVariable(client, lifecycle.LifecycleJobResponse.Id, utils.JobType, utils.Key)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("External secret %s has been deleted", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	lifecycleExternalSecretCmd.AddCommand(lifecycleExternalSecretDeleteCmd)
	lifecycleExternalSecretDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleExternalSecretDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleExternalSecretDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleExternalSecretDeleteCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Name")
	lifecycleExternalSecretDeleteCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "External secret key")

	_ = lifecycleExternalSecretDeleteCmd.MarkFlagRequired("key")
	_ = lifecycleExternalSecretDeleteCmd.MarkFlagRequired("lifecycle")
}

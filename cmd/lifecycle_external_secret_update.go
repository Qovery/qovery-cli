package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var lifecycleExternalSecretUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update lifecycle external secret",
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

		err = utils.UpdateServiceExternalSecret(client, utils.Key, utils.Reference, utils.SecretManagerAccessId, lifecycle.LifecycleJobResponse.Id, utils.JobType)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("External secret %s has been updated", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	lifecycleExternalSecretCmd.AddCommand(lifecycleExternalSecretUpdateCmd)
	lifecycleExternalSecretUpdateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleExternalSecretUpdateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleExternalSecretUpdateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleExternalSecretUpdateCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Name")
	lifecycleExternalSecretUpdateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "External secret key")
	lifecycleExternalSecretUpdateCmd.Flags().StringVarP(&utils.Reference, "reference", "r", "", "New reference to the secret in the secrets provider")
	lifecycleExternalSecretUpdateCmd.Flags().StringVarP(&utils.SecretManagerAccessId, "secret-manager-access-id", "", "", "New secret manager access ID")

	_ = lifecycleExternalSecretUpdateCmd.MarkFlagRequired("key")
	_ = lifecycleExternalSecretUpdateCmd.MarkFlagRequired("lifecycle")
}

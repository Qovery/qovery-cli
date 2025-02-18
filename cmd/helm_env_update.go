package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var helmEnvUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update helm environment variable or secret",
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

		helms, _, err := client.HelmsAPI.ListHelms(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		helm := utils.FindByHelmName(helms.GetResults(), helmName)

		if helm == nil {
			utils.PrintlnError(fmt.Errorf("helm %s not found", helmName))
			utils.PrintlnInfo("You can list all helms with: qovery helm list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		err = utils.UpdateServiceVariable(client, utils.Key, utils.Value, helm.Id, utils.HelmType)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Environment variable %s has been updated", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	helmEnvCmd.AddCommand(helmEnvUpdateCmd)
	helmEnvUpdateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmEnvUpdateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmEnvUpdateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmEnvUpdateCmd.Flags().StringVarP(&helmName, "helm", "n", "", "helm Name")
	helmEnvUpdateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	helmEnvUpdateCmd.Flags().StringVarP(&utils.Value, "value", "v", "", "Environment variable or secret value")

	_ = helmEnvUpdateCmd.MarkFlagRequired("key")
	_ = helmEnvUpdateCmd.MarkFlagRequired("value")
	_ = helmEnvUpdateCmd.MarkFlagRequired("helm")
}

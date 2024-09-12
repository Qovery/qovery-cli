package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var helmEnvDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete helm environment variable or secret",
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

		err = utils.DeleteVariable(client, helm.Id, utils.HelmType, utils.Key)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Variable %s has been deleted", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	helmEnvCmd.AddCommand(helmEnvDeleteCmd)
	helmEnvDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmEnvDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmEnvDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmEnvDeleteCmd.Flags().StringVarP(&helmName, "helm", "n", "", "helm Name")
	helmEnvDeleteCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")

	_ = helmEnvDeleteCmd.MarkFlagRequired("key")
	_ = helmEnvDeleteCmd.MarkFlagRequired("helm")
}

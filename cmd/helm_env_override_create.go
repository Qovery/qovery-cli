package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var helmEnvOverrideCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Override helm environment variable or secret",
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

		err = utils.CreateOverride(client, projectId, envId, helm.Id, utils.HelmType, utils.Key, utils.Value, utils.HelmScope)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("%s has been overidden", pterm.FgBlue.Sprintf(utils.Key)))
	},
}

func init() {
	helmEnvOverrideCmd.AddCommand(helmEnvOverrideCreateCmd)
	helmEnvOverrideCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmEnvOverrideCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmEnvOverrideCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmEnvOverrideCreateCmd.Flags().StringVarP(&helmName, "helm", "n", "", "helm Name")
	helmEnvOverrideCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	helmEnvOverrideCreateCmd.Flags().StringVarP(&utils.Value, "value", "", "", "Environment variable or secret value")
	helmEnvOverrideCreateCmd.Flags().StringVarP(&utils.HelmScope, "scope", "", "HELM", "Scope of this alias <PROJECT|ENVIRONMENT|HELM>")

	_ = helmEnvOverrideCreateCmd.MarkFlagRequired("key")
	_ = helmEnvOverrideCreateCmd.MarkFlagRequired("helm")
}

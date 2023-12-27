package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var helmEnvCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create helm environment variable or secret",
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

		err = utils.CreateEnvironmentVariable(client, helm.Id, utils.HelmScope,  utils.Key, utils.Value, utils.IsSecret)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Environment variable %s has been created", pterm.FgBlue.Sprintf(utils.Key)))
	},
}

func init() {
	helmEnvCmd.AddCommand(helmEnvCreateCmd)
	helmEnvCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmEnvCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmEnvCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmEnvCreateCmd.Flags().StringVarP(&helmName, "helm", "n", "", "helm Name")
	helmEnvCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	helmEnvCreateCmd.Flags().StringVarP(&utils.Value, "value", "v", "", "Environment variable or secret value")
	helmEnvCreateCmd.Flags().StringVarP(&utils.HelmScope, "scope", "", "HELM", "Scope of this env var <PROJECT|ENVIRONMENT|HELM>")
	helmEnvCreateCmd.Flags().BoolVarP(&utils.IsSecret, "secret", "", false, "This environment variable is a secret")

	_ = helmEnvCreateCmd.MarkFlagRequired("key")
	_ = helmEnvCreateCmd.MarkFlagRequired("value")
	_ = helmEnvCreateCmd.MarkFlagRequired("helm")
}

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var helmRedeployCmd = &cobra.Command{
	Use:   "redeploy",
	Short: "Redeploy a helm",
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

		msg, err := utils.RedeployService(client, envId, helm.Id, helm.Name, utils.HelmType, watchFlag)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if msg != "" {
			utils.PrintlnInfo(msg)
			return
		}

		if watchFlag {
			utils.Println(fmt.Sprintf("Helm %s redeployed!", pterm.FgBlue.Sprintf(helmName)))
		} else {
			utils.Println(fmt.Sprintf("Redeploying helm %s in progress..", pterm.FgBlue.Sprintf(helmName)))
		}
	},
}

func init() {
	helmCmd.AddCommand(helmRedeployCmd)
	helmRedeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmRedeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmRedeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmRedeployCmd.Flags().StringVarP(&helmName, "helm", "n", "", "Helm Name")
	helmRedeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch helm status until it's ready or an error occurs")

	_ = helmRedeployCmd.MarkFlagRequired("helm")
}

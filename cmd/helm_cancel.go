package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"os"

	"github.com/qovery/qovery-cli/utils"
)

var helmCancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel a helm deployment",
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

		msg, err := utils.CancelServiceDeployment(client, envId, helm.Id, utils.HelmType, watchFlag)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if msg != "" {
			utils.PrintlnInfo(msg)
			return
		}

		utils.Println(fmt.Sprintf("helm %s deployment cancelled!", pterm.FgBlue.Sprintf("%s", helmName)))
	},
}

func init() {
	helmCmd.AddCommand(helmCancelCmd)
	helmCancelCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmCancelCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmCancelCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmCancelCmd.Flags().StringVarP(&helmName, "helm", "n", "", "Helm Name")
	helmCancelCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch cancel until it's done or an error occurs")

	_ = helmCancelCmd.MarkFlagRequired("helm")
}

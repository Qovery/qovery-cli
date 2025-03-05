package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var helmRedeployCmd = &cobra.Command{
	Use:   "redeploy",
	Short: "Redeploy a helm",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateHelmArguments(helmName, helmNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		helmList := buildHelmListFromHelmNames(client, envId, helmName, helmNames)

		_, _, err := client.HelmActionsAPI.
			DeployHelm(context.Background(), helmList[0].Id).
			HelmDeployRequest(qovery.HelmDeployRequest{}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to redeploy helm(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", helmName, helmNames)))
		WatchHelmDeployment(client, envId, helmList, watchFlag, qovery.STATEENUM_RESTARTED)
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

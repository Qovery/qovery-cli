package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var helmDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a helm",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateHelmArguments(helmName, helmNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		helmList := buildHelmListFromHelmNames(client, envId, helmName, helmNames)
		_, err := client.EnvironmentActionsAPI.
			DeleteSelectedServices(context.Background(), envId).
			EnvironmentServiceIdsAllRequest(qovery.EnvironmentServiceIdsAllRequest{
				HelmIds: utils.Map(helmList, func(helm *qovery.HelmResponse) string {
					return helm.Id
				}),
			}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to delete helm(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", helmName, helmNames)))
		WatchHelmDeployment(client, envId, helmList, watchFlag, qovery.STATEENUM_DELETED)
	},
}

func init() {
	helmCmd.AddCommand(helmDeleteCmd)
	helmDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmDeleteCmd.Flags().StringVarP(&helmName, "helm", "n", "", "Helm Name")
	helmDeleteCmd.Flags().StringVarP(&helmNames, "helms", "", "", "Helm Names (comma separated) (ex: --helms \"helm1,helm2\")")
	helmDeleteCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch helm status until it's ready or an error occurs")
}

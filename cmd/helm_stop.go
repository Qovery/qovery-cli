package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
	"strings"

	"github.com/qovery/qovery-cli/utils"
)

var helmStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a helm",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateHelmArguments(helmName, helmNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		helmList := buildHelmListFromHelmNames(client, envId, helmName, helmNames)
		_, err := client.EnvironmentActionsAPI.
			StopSelectedServices(context.Background(), envId).
			EnvironmentServiceIdsAllRequest(qovery.EnvironmentServiceIdsAllRequest{
				HelmIds: utils.Map(helmList, func(helm *qovery.HelmResponse) string {
					return helm.Id
				}),
			}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to stop helm(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", helmName, helmNames)))
		WatchHelmDeployment(client, envId, helmList, watchFlag, qovery.STATEENUM_STOPPED)
	},
}

func buildHelmListFromHelmNames(
	client *qovery.APIClient,
	environmentId string,
	helmName string,
	helmNames string,
) []*qovery.HelmResponse {
	var helmList []*qovery.HelmResponse
	helms, _, err := client.HelmsAPI.ListHelms(context.Background(), environmentId).Execute()
	checkError(err)

	if helmName != "" {
		helm := utils.FindByHelmName(helms.GetResults(), helmName)
		if helm == nil {
			utils.PrintlnError(fmt.Errorf("helm %s not found", helmName))
			utils.PrintlnInfo("You can list all helms with: qovery helm list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
		helmList = append(helmList, helm)
	}
	if helmNames != "" {
		for _, helmName := range strings.Split(helmNames, ",") {
			trimmedHelmName := strings.TrimSpace(helmName)
			helm := utils.FindByHelmName(helms.GetResults(), trimmedHelmName)
			if helm == nil {
				utils.PrintlnError(fmt.Errorf("helm %s not found", helmName))
				utils.PrintlnInfo("You can list all helms with: qovery helm list")
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}
			helmList = append(helmList, helm)
		}
	}

	return helmList
}

func validateHelmArguments(helmName string, helmNames string) {
	if helmName == "" && helmNames == "" {
		utils.PrintlnError(fmt.Errorf("use either --helm \"<helm name>\" or --helms \"<helm1 name>, <helm2 name>\" but not both at the same time"))
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	if helmName != "" && helmNames != "" {
		utils.PrintlnError(fmt.Errorf("you can't use --helm and --helms at the same time"))
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
}

func init() {
	helmCmd.AddCommand(helmStopCmd)
	helmStopCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmStopCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmStopCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmStopCmd.Flags().StringVarP(&helmName, "helm", "n", "", "Helm Name")
	helmStopCmd.Flags().StringVarP(&helmNames, "helms", "", "", "Helm Names (comma separated) (ex: --helms \"helm1,helm2\")")
	helmStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch helm status until it's ready or an error occurs")
}

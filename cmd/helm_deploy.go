package cmd

import (
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"time"
)

var helmDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a helm",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateHelmArguments(helmName, helmNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		helmList := buildHelmListFromHelmNames(client, envId, helmName, helmNames)
		err := utils.DeployHelms(client, envId, helmList, chartVersion, chartGitCommitId, valuesOverrideCommitId)
		checkError(err)
		utils.Println(fmt.Sprintf("Request to deploy helm(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", helmName, helmNames)))
		WatchHelmDeployment(client, envId, helmList, watchFlag, qovery.STATEENUM_DEPLOYED)
	},
}

func WatchHelmDeployment(
	client *qovery.APIClient,
	envId string,
	helmList []*qovery.HelmResponse,
	watchFlag bool,
	finalServiceState qovery.StateEnum,
) {
	if watchFlag {
		time.Sleep(3 * time.Second) // wait for the deployment request to be processed (prevent from race condition)
		if len(helmList) == 1 {
			utils.WatchHelm(helmList[0].Id, envId, client)
		} else {
			utils.WatchEnvironment(envId, finalServiceState, client)
		}
	}
}

func init() {
	helmCmd.AddCommand(helmDeployCmd)
	helmDeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmDeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmDeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmDeployCmd.Flags().StringVarP(&helmName, "helm", "n", "", "helm Name")
	helmDeployCmd.Flags().StringVarP(&helmNames, "helms", "", "", "helm Names (comma separated) (ex: --helms \"helm1,helm2\")")
	helmDeployCmd.Flags().StringVarP(&chartVersion, "chart_version", "", "", "helm chart version")
	helmDeployCmd.Flags().StringVarP(&chartGitCommitId, "chart_git_commit_id", "", "", "helm chart git commit id")
	helmDeployCmd.Flags().StringVarP(&valuesOverrideCommitId, "values_override_git_commit_id", "", "", "helm values override git commit id")
	helmDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch helm status until it's ready or an error occurs")
}

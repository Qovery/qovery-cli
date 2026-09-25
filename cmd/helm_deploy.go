package cmd

import (
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
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
		watch := utils.NewServicesWatch(client, envId, helmIds(helmList), watchFlag)
		err := utils.DeployHelms(client, envId, helmList, chartVersion, chartGitCommitId, valuesOverrideCommitId)
		checkError(err)
		utils.Println(fmt.Sprintf("Request to deploy helm(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", helmName, helmNames)))
		watch.Wait(qovery.STATEENUM_DEPLOYED)
	},
}

func init() {
	helmCmd.AddCommand(helmDeployCmd)
	helmDeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmDeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmDeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmDeployCmd.Flags().StringVarP(&helmName, "helm", "n", "", "helm Name")
	helmDeployCmd.Flags().StringVarP(&helmNames, "helms", "", "", "helm Names (comma separated) (ex: --helms \"helm1,helm2\")")
	helmDeployCmd.Flags().StringVarP(&chartVersion, "chart_version", "", "", "helm chart version")
	helmDeployCmd.Flags().StringVarP(&chartGitCommitId, "chart_git_commit_id", "", "", "helm chart git commit id, or 'latest' for the newest commit of the branch")
	helmDeployCmd.Flags().StringVarP(&valuesOverrideCommitId, "values_override_git_commit_id", "", "", "helm values override git commit id")
	helmDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch helm status until it's ready or an error occurs")
}

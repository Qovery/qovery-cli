package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"time"

	"github.com/spf13/cobra"
	"os"

	"github.com/qovery/qovery-cli/utils"
)

var helmDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a helm",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

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

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if helmNames != "" {
			// wait until service is ready
			for {
				if utils.IsEnvironmentInATerminalState(envId, client) {
					break
				}

				utils.Println(fmt.Sprintf("Waiting for environment %s to be ready..", pterm.FgBlue.Sprintf("%s", envId)))
				time.Sleep(5 * time.Second)
			}

			// deploy multiple services
			err := utils.DeployHelms(client, envId, helmNames, chartVersion, chartGitCommitId, valuesOverrideCommitId)

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			utils.Println(fmt.Sprintf("Deploying helms %s in progress..", pterm.FgBlue.Sprintf("%s", helmNames)))

			if watchFlag {
				utils.WatchEnvironment(envId, "unused", client)
			}

			return
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

		var mCommitId *string
		var mChartVersion *string
		var mValuesOverrideCommitId *string
		if chartGitCommitId != "" {
			mCommitId = &chartGitCommitId
		}

		if chartVersion != "" {
			mChartVersion = &chartVersion
		}

		if valuesOverrideCommitId != "" {
			mValuesOverrideCommitId = &valuesOverrideCommitId
		}

		req := qovery.HelmDeployRequest{
			ChartVersion:              mChartVersion,
			GitCommitId:               mCommitId,
			ValuesOverrideGitCommitId: mValuesOverrideCommitId,
		}

		msg, err := utils.DeployService(client, envId, helm.Id, utils.HelmType, req, watchFlag)

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
			utils.Println(fmt.Sprintf("helm %s deployed!", pterm.FgBlue.Sprintf("%s", helmName)))
		} else {
			utils.Println(fmt.Sprintf("Deploying helm %s in progress..", pterm.FgBlue.Sprintf("%s", helmName)))
		}
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
	helmDeployCmd.Flags().StringVarP(&chartGitCommitId, "chart_git_commit_id", "", "", "helm chart git commit id")
	helmDeployCmd.Flags().StringVarP(&valuesOverrideCommitId, "values_override_git_commit_id", "", "", "helm values override git commit id")
	helmDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch helm status until it's ready or an error occurs")
}
